"""App management actions for Nothing phones."""

import csv
import json
import re
import sys

from .device import confirm, run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _package_exists(serial: str, package: str) -> bool:
    """Return True if *package* is installed on the device."""
    r = run(["adb", "-s", serial, "shell", f"pm list packages {package}"])
    return f"package:{package}" in r.stdout


def _dumpsys_package(serial: str, package: str) -> str:
    """Run 'dumpsys package <package>' and return raw output."""
    r = run(["adb", "-s", serial, "shell", f"dumpsys package {package}"])
    if r.returncode != 0 and not r.stdout.strip():
        raise AdbError(f"dumpsys package {package} failed: {r.stderr.strip()}")
    return r.stdout


def _fmt_bytes(size: int) -> str:
    """Human-readable byte size."""
    for unit in ("B", "KB", "MB", "GB"):
        if size < 1024:
            return f"{size:.1f} {unit}"
        size /= 1024
    return f"{size:.1f} TB"


def _installer_label(installer: str | None) -> str:
    """Map known installer package names to friendly labels."""
    if not installer or installer in ("null", ""):
        return "Unknown / sideloaded"
    known = {
        "com.android.vending": "Google Play Store",
        "com.amazon.venezia": "Amazon Appstore",
        "org.fdroid.fdroid": "F-Droid",
        "com.huawei.appmarket": "Huawei AppGallery",
    }
    return known.get(installer, installer)


# ---------------------------------------------------------------------------
# Parsing helpers for dumpsys package output
# ---------------------------------------------------------------------------

def _extract_packages_section(output: str, package: str) -> str:
    """
    Extract the first 'Package [<package>]' block from dumpsys output.
    Returns the lines belonging to that block (until the next top-level Package).
    """
    lines = output.splitlines()
    start = None
    for i, line in enumerate(lines):
        if re.match(r"\s{2}Package \[" + re.escape(package) + r"\]", line):
            start = i
            break
    if start is None:
        return ""

    result = []
    for line in lines[start + 1:]:
        # A new top-level Package entry signals end of our block
        if re.match(r"\s{2}Package \[", line):
            break
        result.append(line)
    return "\n".join(result)


def _re_first(pattern: str, text: str, group: int = 1) -> str | None:
    m = re.search(pattern, text)
    return m.group(group).strip() if m else None


def _parse_app_info(serial: str, package: str) -> dict:
    """
    Collect all displayable fields for *package* into a dict.
    Uses 'dumpsys package', 'pm list packages -f', and 'stat'.
    """
    raw = _dumpsys_package(serial, package)
    block = _extract_packages_section(raw, package)

    # --- version ---
    version_name = _re_first(r"versionName=(\S+)", block)
    m_vc = re.search(r"versionCode=(\d+)", block)
    version_code = m_vc.group(1) if m_vc else None

    # --- SDKs ---
    min_sdk = _re_first(r"minSdk=(\d+)", block)
    target_sdk = _re_first(r"targetSdk=(\d+)", block)

    # --- timestamps ---
    # In dumpsys package output the top-level block has:
    #   timeStamp=<install/update time>  lastUpdateTime=<same>
    # Per-user blocks (User 0:) have:
    #   firstInstallTime=<when this user profile got the app>
    # Strategy: lastUpdateTime from top block; firstInstallTime from User 0 sub-block.
    top_block = re.split(r"\n\s+User \d+:", block)[0]
    last_update   = _re_first(r"lastUpdateTime=([\d\- :]+)", top_block) or \
                    _re_first(r"timeStamp=([\d\- :]+)", top_block)
    # firstInstallTime is inside "User 0:" sub-block
    user0_match = re.search(r"User 0:.*?(?=\n\s+User \d+:|\Z)", block, re.DOTALL)
    first_install = None
    if user0_match:
        first_install = _re_first(r"firstInstallTime=([\d\- :]+)", user0_match.group(0))

    # --- code path / APK ---
    code_path = _re_first(r"codePath=(\S+)", block)
    apk_path = None
    if code_path:
        apk_path = code_path.rstrip("/") + "/base.apk"

    # Fallback: pm list packages -f
    if not apk_path:
        r2 = run(["adb", "-s", serial, "shell", f"pm list packages -f {package}"])
        for line in r2.stdout.splitlines():
            line = line.strip()
            if line.startswith("package:") and f"={package}" in line:
                apk_path = line.removeprefix("package:").split("=")[0].strip()
                break

    # --- APK file size ---
    apk_size_str = None
    if apk_path:
        r_stat = run(["adb", "-s", serial, "shell", f"stat {apk_path} 2>/dev/null"])
        m_size = re.search(r"Size:\s+(\d+)", r_stat.stdout)
        if m_size:
            apk_size_str = _fmt_bytes(int(m_size.group(1)))

    # --- data dir size ---
    data_dir = f"/data/data/{package}"
    r_du = run(["adb", "-s", serial, "shell", f"du -sh {data_dir} 2>/dev/null"])
    data_size_str = None
    if r_du.returncode == 0 and r_du.stdout.strip():
        parts = r_du.stdout.strip().split()
        if parts:
            data_size_str = parts[0]

    # --- enabled status ---
    # "enabled=0" means default/enabled; non-zero values mean disabled
    enabled_str = "enabled"
    m_enabled = re.search(r"User 0:.*?enabled=(\d+)", block)
    if m_enabled:
        val = int(m_enabled.group(1))
        # 0 = COMPONENT_ENABLED_STATE_DEFAULT (enabled), 2 = DISABLED_UNTIL_USED, etc.
        if val not in (0, 1):
            enabled_str = "disabled"

    # --- installer ---
    installer = _re_first(r"installerPackageName=(\S+)", block)

    return {
        "package":       package,
        "version_name":  version_name,
        "version_code":  version_code,
        "min_sdk":       min_sdk,
        "target_sdk":    target_sdk,
        "first_install": first_install,
        "last_update":   last_update,
        "apk_path":      apk_path,
        "apk_size":      apk_size_str,
        "data_size":     data_size_str,
        "enabled":       enabled_str,
        "installer":     installer,
    }


# ---------------------------------------------------------------------------
# 1. action_app_info
# ---------------------------------------------------------------------------

def action_app_info(device: DeviceInfo, package: str) -> None:
    """Display detailed information about an installed app."""
    if not _package_exists(device.serial, package):
        raise AdbError(f"Package not found on device: {package}")

    info = _parse_app_info(device.serial, package)

    def _field(label: str, value: str | None, fallback: str = "not available") -> None:
        print(f"  {label:<18}: {value if value else fallback}")

    print(f"\n  App Info — {device.model}\n")
    _field("Package",          info["package"])
    _field("Version name",     info["version_name"])
    _field("Version code",     info["version_code"])
    _field("Min SDK",          info["min_sdk"])
    _field("Target SDK",       info["target_sdk"])
    _field("First installed",  info["first_install"])
    _field("Last updated",     info["last_update"])
    _field("APK path",         info["apk_path"])
    _field("APK size",         info["apk_size"])
    _field("Data size",        info["data_size"], "(no root / not available)")
    _field("Status",           info["enabled"])
    _field("Installer",        _installer_label(info["installer"]))
    print()


# ---------------------------------------------------------------------------
# 2. action_kill_app
# ---------------------------------------------------------------------------

def action_kill_app(device: DeviceInfo, package: str, clear_cache: bool) -> None:
    """Force-stop an app, optionally clearing its data/cache."""
    if not _package_exists(device.serial, package):
        raise AdbError(f"Package not found on device: {package}")

    print(f"  Force-stopping {package} on {device.model}...")
    r = run(["adb", "-s", device.serial, "shell", f"am force-stop {package}"])
    if r.returncode != 0:
        raise AdbError(f"am force-stop failed: {r.stderr.strip()}")
    print("  Done.")

    if clear_cache:
        confirm(f"\n  This will clear ALL data and cache for {package}. Continue?")
        print(f"  Clearing data for {package}...")
        r2 = run(["adb", "-s", device.serial, "shell", f"pm clear {package}"])
        if r2.returncode != 0:
            raise AdbError(f"pm clear failed: {r2.stderr.strip()}")
        if "Success" in r2.stdout:
            print("  Data cleared successfully.")
        else:
            print(f"  pm clear output: {r2.stdout.strip()}")


# ---------------------------------------------------------------------------
# 3. action_launch_app
# ---------------------------------------------------------------------------

def _get_user_apps(serial: str) -> list[tuple[str, str]]:
    """
    Return list of (package, label) tuples for user-installed apps.
    Label is the app name if resolvable; falls back to package name.
    """
    r = run(["adb", "-s", serial, "shell", "pm list packages -3"])
    packages = []
    for line in r.stdout.splitlines():
        line = line.strip()
        if line.startswith("package:"):
            packages.append(line.removeprefix("package:").strip())
    return [(p, p) for p in sorted(packages)]


def action_launch_app(
    device: DeviceInfo,
    package: str | None,
    intent: str | None,
) -> None:
    """
    Launch an app or an intent URI on the device.

    - *package*: launch the Launcher activity via monkey.
    - *intent*: launch an ACTION_VIEW intent with this URI.
    - Both None: list user apps and prompt for selection.
    """
    if package is not None and intent is not None:
        raise AdbError("Specify either --package or --intent, not both.")

    if package is not None:
        _launch_package(device, package)
        return

    if intent is not None:
        _launch_intent(device, intent)
        return

    # Interactive selection
    apps = _get_user_apps(device.serial)
    if not apps:
        print("  No user-installed apps found.")
        return

    print(f"\n  User apps on {device.model}:\n")
    for i, (pkg, label) in enumerate(apps, start=1):
        display = pkg if pkg == label else f"{label}  ({pkg})"
        print(f"  {i:3}. {display}")
    print()

    try:
        raw = input("  Enter number to launch (or press Enter to cancel): ").strip()
    except (EOFError, KeyboardInterrupt):
        print("\n  Cancelled.")
        return

    if not raw:
        print("  Cancelled.")
        return

    try:
        idx = int(raw) - 1
        if idx < 0 or idx >= len(apps):
            raise ValueError
    except ValueError:
        print("  Invalid selection.")
        return

    chosen_pkg, _ = apps[idx]
    _launch_package(device, chosen_pkg)


def _launch_package(device: DeviceInfo, package: str) -> None:
    if not _package_exists(device.serial, package):
        raise AdbError(f"Package not found on device: {package}")

    print(f"  Launching {package} on {device.model}...")
    r = run([
        "adb", "-s", device.serial, "shell",
        f"monkey -p {package} -c android.intent.category.LAUNCHER 1",
    ])
    output = (r.stdout + r.stderr).strip()
    if r.returncode != 0 or "Error" in output or "aborted" in output:
        raise AdbError(f"Failed to launch {package}: {output}")
    print("  Launched.")


def _launch_intent(device: DeviceInfo, uri: str) -> None:
    print(f"  Starting VIEW intent: {uri}")
    r = run([
        "adb", "-s", device.serial, "shell",
        f"am start -a android.intent.action.VIEW -d {uri}",
    ])
    output = (r.stdout + r.stderr).strip()
    if r.returncode != 0 or "Error" in output:
        raise AdbError(f"Failed to start intent '{uri}': {output}")
    print("  Intent sent.")


# ---------------------------------------------------------------------------
# 4. action_package_list
# ---------------------------------------------------------------------------

def _get_all_packages(serial: str, include_system: bool) -> list[dict]:
    """
    Return list of dicts with keys: package, version_code, apk_path.
    Uses 'pm list packages -f --show-versioncode'.
    """
    flags = "" if include_system else "-3"
    r = run([
        "adb", "-s", serial, "shell",
        f"pm list packages -f --show-versioncode {flags}".strip(),
    ])
    if r.returncode != 0:
        raise AdbError(f"pm list packages failed: {r.stderr.strip()}")

    results = []
    for line in r.stdout.splitlines():
        line = line.strip()
        if not line.startswith("package:"):
            continue
        # Format: package:<apk_path>=<pkg> versionCode:<code>
        # or:     package:<apk_path>=<pkg>
        line = line.removeprefix("package:")
        vc_match = re.search(r"\s+versionCode:(\d+)$", line)
        version_code = vc_match.group(1) if vc_match else None
        main_part = line[: vc_match.start()] if vc_match else line

        if "=" in main_part:
            apk_path, _, pkg = main_part.rpartition("=")
        else:
            apk_path = None
            pkg = main_part.strip()

        results.append({
            "package":      pkg.strip(),
            "version_code": version_code or "",
            "apk_path":     apk_path.strip() if apk_path else "",
        })

    return sorted(results, key=lambda d: d["package"])


def _write_text_table(rows: list[dict], out) -> None:
    if not rows:
        out.write("  (no packages)\n")
        return
    w_pkg = max(len(r["package"])      for r in rows)
    w_vc  = max(len(r["version_code"]) for r in rows)
    w_pkg = max(w_pkg, len("Package"))
    w_vc  = max(w_vc,  len("VersionCode"))

    header = f"  {'Package':<{w_pkg}}  {'VersionCode':<{w_vc}}  APK Path"
    sep    = "  " + "-" * (w_pkg + w_vc + 4 + 40)
    out.write(header + "\n")
    out.write(sep + "\n")
    for r in rows:
        out.write(f"  {r['package']:<{w_pkg}}  {r['version_code']:<{w_vc}}  {r['apk_path']}\n")


def action_package_list(
    device: DeviceInfo,
    include_system: bool,
    fmt: str,
    output_path: str | None,
) -> None:
    """List installed apps with package name, version, and APK path."""
    scope = "all" if include_system else "user-installed"
    print(f"\r  Fetching {scope} packages from {device.model}...", end="", flush=True)

    rows = _get_all_packages(device.serial, include_system)

    # Clear progress line
    print("\r" + " " * 60 + "\r", end="", flush=True)

    fmt = fmt.lower()

    if output_path:
        fh = open(output_path, "w", newline="" if fmt == "csv" else None, encoding="utf-8")
    else:
        fh = sys.stdout

    try:
        if fmt == "json":
            json.dump(rows, fh, indent=2, ensure_ascii=False)
            fh.write("\n")
        elif fmt == "csv":
            writer = csv.DictWriter(fh, fieldnames=["package", "version_code", "apk_path"])
            writer.writeheader()
            writer.writerows(rows)
        else:  # text / default
            _write_text_table(rows, fh)
    finally:
        if output_path:
            fh.close()

    if output_path:
        print(f"  Wrote {len(rows)} packages to {output_path}")
    else:
        print(f"\n  Total: {len(rows)} packages")
