"""Reboot target selection for Nothing phones."""

from .device import adb_shell, run
from .exceptions import AdbError
from .models import DeviceInfo

_MENU = """\
  Reboot target:
    [0] System (normal reboot)
    [1] Bootloader / Fastboot
    [2] Recovery
    [3] Safe mode
    [4] Download mode  (MediaTek only)
    [5] ADB Sideload
"""

_TARGET_MAP = {
    "0": "system",
    "1": "bootloader",
    "2": "recovery",
    "3": "safe",
    "4": "download",
    "5": "sideload",
}


def _is_mediatek(serial: str) -> bool:
    """Return True if the device SoC is MediaTek (ro.board.platform contains 'mt')."""
    try:
        platform = adb_shell("getprop ro.board.platform", serial)
        return "mt" in platform.lower()
    except AdbError:
        return False


def action_reboot(device: DeviceInfo, target: str | None) -> None:
    """Reboot the device to the specified target, or prompt interactively."""
    if target is None:
        print(_MENU)
        try:
            choice = input("  Select [0]: ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nAborted.")
            return

        if choice == "":
            choice = "0"

        if choice not in _TARGET_MAP:
            raise AdbError(f"Invalid selection: {choice!r}. Choose 0–5.")

        target = _TARGET_MAP[choice]

    target = target.lower()

    if target in ("system", None):
        print("Rebooting to system...")
        r = run(["adb", "-s", device.serial, "reboot"])
        if r.returncode != 0:
            raise AdbError(f"Reboot failed: {r.stderr.strip()}")
        print("[OK] Reboot command sent.")

    elif target == "bootloader":
        print("Rebooting to bootloader...")
        r = run(["adb", "-s", device.serial, "reboot", "bootloader"])
        if r.returncode != 0:
            raise AdbError(f"Reboot to bootloader failed: {r.stderr.strip()}")
        print("[OK] Reboot command sent.")

    elif target == "recovery":
        print("Rebooting to recovery...")
        r = run(["adb", "-s", device.serial, "reboot", "recovery"])
        if r.returncode != 0:
            raise AdbError(f"Reboot to recovery failed: {r.stderr.strip()}")
        print("[OK] Reboot command sent.")

    elif target == "safe":
        print("Rebooting to safe mode...")
        r = run(["adb", "-s", device.serial, "shell",
                 "setprop persist.sys.safemode 1 && reboot"])
        if r.returncode != 0:
            raise AdbError(f"Reboot to safe mode failed: {r.stderr.strip()}")
        print("[OK] Reboot command sent.")
        print("[WARN] Safe mode disables itself automatically after the next reboot.")

    elif target == "download":
        if _is_mediatek(device.serial):
            print("Rebooting to download mode (MediaTek)...")
            r = run(["adb", "-s", device.serial, "reboot", "download"])
            if r.returncode != 0:
                raise AdbError(f"Reboot to download mode failed: {r.stderr.strip()}")
            print("[OK] Reboot command sent.")
        else:
            print("[WARN] Download mode is only natively supported on MediaTek devices.")
            print("[WARN] For Qualcomm devices, use EDL (Emergency Download) mode instead:")
            print("         Power off the device, then hold Volume Down + connect USB.")

    elif target == "sideload":
        print("Rebooting to ADB sideload mode...")
        r = run(["adb", "-s", device.serial, "reboot", "sideload"])
        if r.returncode != 0:
            raise AdbError(f"Reboot to sideload failed: {r.stderr.strip()}")
        print("[OK] Reboot command sent.")

    else:
        raise AdbError(
            f"Unknown reboot target: {target!r}. "
            "Valid targets: system, bootloader, recovery, safe, download, sideload."
        )
