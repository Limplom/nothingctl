"""Wireless ADB setup — tcpip mode + auto-connect."""

import re
import time

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


def _get_device_ip(serial: str) -> str | None:
    """Return the WLAN IP of the device, or None if not found."""
    for iface in ("wlan0", "wlan1", "wlan2"):
        r = run(["adb", "-s", serial, "shell",
                 f"ip -f inet addr show {iface} 2>/dev/null"])
        m = re.search(r"inet (\d+\.\d+\.\d+\.\d+)/", r.stdout)
        if m:
            return m.group(1)
    # Fallback: ip route (works on most Android versions)
    r = run(["adb", "-s", serial, "shell", "ip route"])
    m = re.search(r"src (\d+\.\d+\.\d+\.\d+)", r.stdout)
    return m.group(1) if m else None


def action_wifi_adb(device: DeviceInfo) -> None:
    """
    Switch the device to TCP/IP ADB mode (port 5555) and connect wirelessly.
    The USB cable can be unplugged after this completes.
    Requires the device to be on the same Wi-Fi network as this PC.
    """
    print("\nDetecting device IP address...")
    ip = _get_device_ip(device.serial)
    if not ip:
        raise AdbError(
            "Could not detect device IP. Make sure Wi-Fi is connected,\n"
            "then run manually: adb tcpip 5555 && adb connect <device_ip>:5555"
        )
    print(f"  Device IP : {ip}")

    print("Switching ADB to TCP/IP mode (port 5555)...")
    r = run(["adb", "-s", device.serial, "tcpip", "5555"])
    if r.returncode != 0:
        raise AdbError(f"adb tcpip failed: {r.stderr.strip()}")

    # Brief pause — device needs a moment to reopen the ADB daemon in TCP mode
    time.sleep(2)

    print(f"Connecting to {ip}:5555...")
    r = run(["adb", "connect", f"{ip}:5555"])
    out = r.stdout.strip()

    if "connected" in out.lower():
        print(f"[OK] Wireless ADB active on {ip}:5555")
        print("     You can now disconnect the USB cable.\n")
        print(f"Reconnect later with:  adb connect {ip}:5555")
        print(f"Disconnect with:       adb disconnect {ip}:5555")
    else:
        raise AdbError(
            f"Connection to {ip}:5555 failed: {out}\n"
            "Check that phone and PC are on the same Wi-Fi network."
        )
