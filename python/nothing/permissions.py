"""Dangerous permission auditing for installed apps."""

import sys

from .device import adb_shell, run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Dangerous permission list
# ---------------------------------------------------------------------------

DANGEROUS_PERMISSIONS = [
    "android.permission.READ_CONTACTS",
    "android.permission.WRITE_CONTACTS",
    "android.permission.READ_CALL_LOG",
    "android.permission.WRITE_CALL_LOG",
    "android.permission.READ_PHONE_STATE",
    "android.permission.CALL_PHONE",
    "android.permission.CAMERA",
    "android.permission.RECORD_AUDIO",
    "android.permission.ACCESS_FINE_LOCATION",
    "android.permission.ACCESS_COARSE_LOCATION",
    "android.permission.ACCESS_BACKGROUND_LOCATION",
    "android.permission.READ_EXTERNAL_STORAGE",
    "android.permission.WRITE_EXTERNAL_STORAGE",
    "android.permission.READ_MEDIA_IMAGES",
    "android.permission.READ_MEDIA_VIDEO",
    "android.permission.READ_MEDIA_AUDIO",
    "android.permission.BODY_SENSORS",
    "android.permission.ACTIVITY_RECOGNITION",
    "android.permission.SEND_SMS",
    "android.permission.RECEIVE_SMS",
    "android.permission.READ_SMS",
    "android.permission.BLUETOOTH_SCAN",
    "android.permission.BLUETOOTH_CONNECT",
    "android.permission.NEARBY_WIFI_DEVICES",
    "android.permission.USE_BIOMETRIC",
    "android.permission.USE_FINGERPRINT",
    "android.permission.PROCESS_OUTGOING_CALLS",
    "android.permission.READ_CALENDAR",
    "android.permission.WRITE_CALENDAR",
]

# Set for O(1) membership checks
_DANGEROUS_SET = set(DANGEROUS_PERMISSIONS)

# Short display name: strip "android.permission." prefix
def _short(perm: str) -> str:
    return perm.removeprefix("android.permission.")


# ---------------------------------------------------------------------------
# Parsing helpers
# ---------------------------------------------------------------------------

def _get_packages(serial: str) -> list[str]:
    """Return list of user-installed package names (pm list packages -3)."""
    r = run(["adb", "-s", serial, "shell", "pm list packages -3"])
    if r.returncode != 0:
        raise AdbError(f"Failed to list packages: {r.stderr.strip()}")
    packages = []
    for line in r.stdout.splitlines():
        line = line.strip()
        if line.startswith("package:"):
            packages.append(line.removeprefix("package:").strip())
    return packages


def _dumpsys_package(serial: str, package: str) -> str:
    """Run dumpsys package <package> and return raw output."""
    r = run(["adb", "-s", serial, "shell", f"dumpsys package {package}"])
    if r.returncode != 0 and not r.stdout.strip():
        raise AdbError(f"dumpsys package {package} failed: {r.stderr.strip()}")
    return r.stdout


def _parse_granted_dangerous(dumpsys_output: str) -> list[str]:
    """
    Parse lines like:
        android.permission.CAMERA: granted=true, flags=[ USER_SET]
    and return only those in DANGEROUS_PERMISSIONS that are granted=true.
    """
    granted = []
    for line in dumpsys_output.splitlines():
        line = line.strip()
        if "android.permission." not in line:
            continue
        if "granted=true" not in line:
            continue
        # Extract permission name — it appears before the colon
        colon_idx = line.find(":")
        if colon_idx == -1:
            continue
        perm = line[:colon_idx].strip()
        if perm in _DANGEROUS_SET:
            granted.append(perm)
    return granted


def _parse_all_dangerous(dumpsys_output: str) -> tuple[list[str], list[str]]:
    """
    Return (granted_dangerous, not_granted_dangerous) for a single package's
    dumpsys output.
    """
    granted_set: set[str] = set()
    seen_dangerous: set[str] = set()

    for line in dumpsys_output.splitlines():
        line = line.strip()
        if "android.permission." not in line:
            continue
        colon_idx = line.find(":")
        if colon_idx == -1:
            continue
        perm = line[:colon_idx].strip()
        if perm not in _DANGEROUS_SET:
            continue
        seen_dangerous.add(perm)
        if "granted=true" in line:
            granted_set.add(perm)

    granted = [p for p in DANGEROUS_PERMISSIONS if p in granted_set]
    not_granted = [p for p in DANGEROUS_PERMISSIONS if p in seen_dangerous and p not in granted_set]
    # Include permissions NOT seen in dumpsys as not-granted too
    all_not_granted = [p for p in DANGEROUS_PERMISSIONS if p not in granted_set]
    return granted, all_not_granted


# ---------------------------------------------------------------------------
# Public action
# ---------------------------------------------------------------------------

def action_permissions(device: DeviceInfo, package: str | None = None) -> None:
    """
    Audit dangerous permissions.

    If *package* is given, show granted and not-granted dangerous permissions
    for that single package.

    If *package* is None, scan all user-installed apps and list those that
    have at least one dangerous permission granted.
    """
    if package:
        _audit_single(device, package)
    else:
        _audit_all(device)


def _audit_single(device: DeviceInfo, package: str) -> None:
    """Show dangerous permission status for one package."""
    output = _dumpsys_package(device.serial, package)

    # Detect package-not-found — dumpsys returns nearly empty output
    if f"Package [{package}]" not in output and f"package:{package}" not in output:
        # Verify via pm list packages
        r = run(["adb", "-s", device.serial, "shell", f"pm list packages {package}"])
        if f"package:{package}" not in r.stdout:
            raise AdbError(f"Package not found on device: {package}")

    granted, not_granted = _parse_all_dangerous(output)

    print(f"\n  Permissions for {package}:\n")

    if granted:
        print("  GRANTED (dangerous):")
        for perm in granted:
            print(f"    {_short(perm)}")
    else:
        print("  GRANTED (dangerous): none")

    print()
    print("  NOT GRANTED (dangerous):")
    for perm in not_granted:
        print(f"    {_short(perm)}")
    print()


def _audit_all(device: DeviceInfo) -> None:
    """Scan all user-installed packages and report dangerous permission grants."""
    packages = _get_packages(device.serial)
    total = len(packages)

    results: list[tuple[str, list[str]]] = []  # (package, [granted_perms])

    for i, pkg in enumerate(packages, start=1):
        print(f"\r  Scanning {i}/{total}...", end="", flush=True)
        try:
            output = _dumpsys_package(device.serial, pkg)
            granted = _parse_granted_dangerous(output)
            if granted:
                results.append((pkg, granted))
        except AdbError:
            # Skip packages that fail to query
            continue

    # Clear the progress line
    print("\r" + " " * 40 + "\r", end="", flush=True)

    if not results:
        print("  [OK] No user-installed apps have dangerous permissions granted.")
        return

    print(f"  Permission Audit — {len(results)} apps with dangerous permissions\n")

    for pkg, perms in results:
        short_names = ", ".join(_short(p) for p in perms)
        print(f"  {pkg}")
        print(f"    {short_names}")
        print()

    print("  Run with --package <pkg> for per-app detail.")
