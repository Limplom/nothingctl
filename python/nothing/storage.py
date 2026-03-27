"""Storage report and APK extraction for Nothing phones."""

import datetime
from pathlib import Path

from .device import adb_pull, run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _parse_du(line: str) -> tuple[int, str] | None:
    """Parse a 'du -sk' output line → (kb, path). Returns None on error."""
    parts = line.strip().split("\t", 1)
    if len(parts) != 2:
        return None
    try:
        return int(parts[0]), parts[1].strip()
    except ValueError:
        return None


def _fmt_size(kb: int) -> str:
    if kb >= 1024 * 1024:
        return f"{kb / 1024 / 1024:6.1f} GB"
    if kb >= 1024:
        return f"{kb / 1024:6.1f} MB"
    return f"{kb:6d} KB"


def _du_sorted(serial: str, path: str, top_n: int, use_root: bool) -> list[tuple[int, str]]:
    """Run du -sk on path/*, return top_n entries sorted by size descending."""
    cmd = f"du -sk {path}/*/ 2>/dev/null | sort -rn | head -{top_n}"
    if use_root:
        cmd = f"su -c '{cmd}'"
    r = run(["adb", "-s", serial, "shell", cmd])
    results = []
    for line in r.stdout.strip().splitlines():
        parsed = _parse_du(line)
        if parsed:
            results.append(parsed)
    return results


def _free_space(serial: str, mount: str) -> str:
    """Return free/total space string for a mount point."""
    r = run(["adb", "-s", serial, "shell", f"df -h {mount} 2>/dev/null | tail -1"])
    parts = r.stdout.strip().split()
    # df -h columns: Filesystem Size Used Avail Use% Mounted
    if len(parts) >= 4:
        return f"free {parts[3]} of {parts[1]}"
    return "unknown"


# ---------------------------------------------------------------------------
# Storage report
# ---------------------------------------------------------------------------

def action_storage_report(device: DeviceInfo, top_n: int = 20) -> None:
    """
    Show top-N largest directories in /data/data/, /sdcard/Android/data/, /sdcard/.
    /data/data/ requires root.
    """
    sections = [
        ("/data/data",          True,  "App data  (/data/data/)"),
        ("/sdcard/Android/data", False, "App cache (/sdcard/Android/data/)"),
        ("/sdcard",             False, "SD card   (/sdcard/)"),
    ]

    for path, needs_root, title in sections:
        print(f"\n  {title}")
        free = _free_space(device.serial, path)
        print(f"  {free}")
        print("  " + "─" * 52)

        entries = _du_sorted(device.serial, path, top_n, use_root=needs_root)
        if not entries:
            if needs_root:
                print("  (no data — root required)")
            else:
                print("  (empty or not accessible)")
            continue

        for kb, full_path in entries:
            name = full_path.rstrip("/").split("/")[-1]
            print(f"  {_fmt_size(kb)}  {name}")

    print()


# ---------------------------------------------------------------------------
# APK extract
# ---------------------------------------------------------------------------

def action_apk_extract(device: DeviceInfo, base_dir: Path,
                       include_system: bool = False) -> None:
    """
    Pull APKs for all user-installed apps (or all apps with --include-system).
    Saves to base_dir/apk_extract/<timestamp>/.
    """
    flag = "" if include_system else "-3"
    r = run(["adb", "-s", device.serial, "shell", f"pm list packages {flag}"])
    packages = [
        line.split("package:", 1)[1].strip()
        for line in r.stdout.splitlines()
        if line.startswith("package:")
    ]

    if not packages:
        raise AdbError("No packages found.")

    dest = base_dir / "Backups" / "apk_extract"
    dest.mkdir(parents=True, exist_ok=True)

    label = "all" if include_system else "user"
    print(f"\nExtracting {len(packages)} APKs ({label}) → {dest}")
    ok = failed = 0

    for pkg in packages:
        r2 = run(["adb", "-s", device.serial, "shell", f"pm path {pkg}"])
        apk_path = None
        for line in r2.stdout.splitlines():
            if line.startswith("package:"):
                apk_path = line.split("package:", 1)[1].strip()
                break

        if not apk_path:
            print(f"  [SKIP] {pkg} — path not found")
            failed += 1
            continue

        local = dest / f"{pkg}.apk"
        try:
            adb_pull(apk_path, local, device.serial)
            print(f"  [OK]   {pkg}")
            ok += 1
        except Exception as e:
            print(f"  [FAIL] {pkg}: {e}")
            failed += 1

    print(f"\n[OK] {ok}/{len(packages)} APKs extracted → {dest}")
    if failed:
        print(f"     {failed} skipped/failed")
