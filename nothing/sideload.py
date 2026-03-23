"""APK / split-APK sideload helpers."""

from pathlib import Path

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


def action_sideload(device: DeviceInfo, apk_path: str, downgrade: bool = False) -> None:
    """
    Install an APK or a directory of split APKs.

    - Single file (.apk)    → adb install -r [-d]
    - Directory             → adb install-multiple -r [-d] *.apk
    - downgrade=True        → adds -d flag (allows lower versionCode)
    """
    path = Path(apk_path)
    if not path.exists():
        raise AdbError(f"Path not found: {path}")

    if path.is_dir():
        apks = sorted(path.glob("*.apk"))
        if not apks:
            raise AdbError(f"No .apk files found in {path}")
        print(f"\nSplit APK install ({len(apks)} parts):")
        for a in apks:
            print(f"  {a.name}")
        cmd = ["adb", "-s", device.serial, "install-multiple", "-r"]
        if downgrade:
            cmd.append("-d")
        cmd.extend(str(a) for a in apks)
    else:
        if path.suffix.lower() != ".apk":
            raise AdbError(f"Expected a .apk file or directory, got: {path.name}")
        print(f"\nInstalling {path.name} ({path.stat().st_size // 1024} KB)...")
        cmd = ["adb", "-s", device.serial, "install", "-r"]
        if downgrade:
            cmd.append("-d")
        cmd.append(str(path))

    r = run(cmd, capture=True)
    output = (r.stdout + r.stderr).strip()

    if r.returncode == 0 and "Success" in output:
        print(f"[OK] Installed successfully.")
    elif "INSTALL_FAILED_VERSION_DOWNGRADE" in output:
        raise AdbError(
            "Install blocked: target version is older than installed.\n"
            "Use --downgrade to allow lower versionCode installs."
        )
    elif "INSTALL_FAILED_ALREADY_EXISTS" in output:
        raise AdbError("Package already installed. Use -r flag (already included) — "
                       "if still failing, uninstall first.")
    else:
        raise AdbError(f"Install failed: {output}")
