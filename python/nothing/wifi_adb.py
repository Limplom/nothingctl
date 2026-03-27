"""Wireless ADB setup — tcpip mode, auto-connect, and Android 11+ pairing."""

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


def action_adb_pair(port: int = 5555) -> None:
    """
    Android 11+ wireless ADB pairing flow.

    Guides the user through the on-device pairing steps, then runs:
      adb pair <ip>:<pairing_port> <pairing_code>
    followed by:
      adb connect <ip>:<port>

    No DeviceInfo is needed — pairing happens before a device is connected.
    The 'port' parameter is the final connection port (default 5555).
    """
    print("\n  Wireless ADB Pairing (Android 11+)\n")
    print("  On your phone:")
    print("    1. Settings -> Developer options -> Wireless debugging")
    print("    2. Tap \"Pair device with pairing code\"")
    print("    3. Note the IP address, pairing port, and 6-digit code shown on screen\n")

    try:
        ip_addr      = input("  Enter device IP address: ").strip()
        pairing_port = input("  Enter pairing port (shown on phone): ").strip()
        pairing_code = input("  Enter 6-digit pairing code: ").strip()
    except (EOFError, KeyboardInterrupt):
        print()
        raise AdbError("Pairing aborted by user.")

    if not ip_addr or not pairing_port or not pairing_code:
        raise AdbError("IP address, pairing port, and pairing code are all required.")

    pair_target = f"{ip_addr}:{pairing_port}"
    print(f"\n  Pairing with {pair_target}...")

    r = run(["adb", "pair", pair_target, pairing_code])
    out = (r.stdout + r.stderr).strip()

    if "successfully paired" not in out.lower():
        raise AdbError(
            f"Pairing failed: {out}\n"
            "Make sure the code and port match exactly what is shown on the phone.\n"
            "The pairing code expires after a short time — try again if needed."
        )
    print("[OK] Device paired!\n")

    connect_target = f"{ip_addr}:{port}"
    print(f"  Connecting to {connect_target}...")
    r = run(["adb", "connect", connect_target])
    out = (r.stdout + r.stderr).strip()

    if "connected" in out.lower():
        print(f"[OK] Wireless ADB active on {connect_target}")
        print(f"Reconnect later with:  adb connect {connect_target}")
        print(f"Disconnect with:       adb disconnect {connect_target}")
    else:
        raise AdbError(
            f"Connection to {connect_target} failed: {out}\n"
            "Pairing succeeded but connection was refused. "
            "Check that both devices are on the same Wi-Fi network."
        )
