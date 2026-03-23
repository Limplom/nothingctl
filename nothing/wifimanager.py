"""WiFi scanning and saved network management for Nothing phones."""

import re
import time

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


def _shell(serial: str, cmd: str) -> str:
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


# ── Helpers ───────────────────────────────────────────────────────────────────

def _band_from_freq(freq: int) -> str:
    """Return human-readable band label from frequency in MHz."""
    if freq < 3000:
        return "2.4 GHz"
    if freq < 6000:
        return "5 GHz"
    return "6 GHz"


def _security_from_caps(caps: str) -> str:
    """Extract the highest security protocol from a capabilities string."""
    caps_upper = caps.upper()
    if "WPA3" in caps_upper or "SAE" in caps_upper:
        return "WPA3"
    if "WPA2" in caps_upper:
        return "WPA2"
    if "WPA" in caps_upper:
        return "WPA"
    if "WEP" in caps_upper:
        return "WEP"
    return "Open"


def _parse_scan_results(output: str) -> list[dict]:
    """
    Parse the output of `cmd wifi list-scan-results`.

    Expected format (header line followed by data lines):
        BSSID              Frequency  RSSI  SSID                    Capabilities
        aa:bb:cc:dd:ee:ff  2412       -45   MyNetwork               [WPA2-PSK]

    Returns a list of dicts with keys: bssid, freq, rssi, ssid, caps.
    """
    networks: list[dict] = []
    lines = output.splitlines()

    # Skip header line(s) — look for the first line that starts with a MAC address
    data_lines = [
        line for line in lines
        if re.match(r"[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:", line.strip())
    ]

    for line in data_lines:
        # Format (Android 12+): BSSID  Frequency  RSSI  Age(sec)  SSID  Flags
        # RSSI may include per-antenna info: -87(0:-93/1:-89)
        parts = line.split()
        if len(parts) < 4:
            continue
        bssid = parts[0]
        try:
            freq = int(parts[1])
            # RSSI: extract the leading integer (may be "-87(0:-93/1:-89)")
            rssi_m = re.match(r"(-?\d+)", parts[2])
            rssi = int(rssi_m.group(1)) if rssi_m else 0
        except (ValueError, IndexError):
            continue
        # Skip the Age(sec) field (e.g. "6,617" or ">1000.0") if present before capabilities
        # Caps start with '['; SSID is between Age and Caps
        skip = 3
        # Age field: numeric, comma-separated digits, or ">NNN.N" (stale entries)
        if skip < len(parts) and re.match(r"[>\d][\d,.*]*$", parts[skip]):
            skip += 1
        rest = parts[skip:]
        caps_idx = next(
            (i for i, p in enumerate(rest) if p.startswith("[")), len(rest)
        )
        ssid = " ".join(rest[:caps_idx]) if caps_idx > 0 else ""
        caps = " ".join(rest[caps_idx:])
        networks.append({
            "bssid": bssid,
            "freq": freq,
            "rssi": rssi,
            "ssid": ssid or "<hidden>",
            "caps": caps,
        })

    return networks


# ── Public actions ─────────────────────────────────────────────────────────────

def action_wifi_scan(device: DeviceInfo) -> None:
    """
    Scan and list nearby WiFi networks, sorted by signal strength (RSSI).
    Triggers a fresh scan if no results are available on the first attempt.
    """
    serial = device.serial

    raw = _shell(serial, "cmd wifi list-scan-results")
    networks = _parse_scan_results(raw)

    if not networks:
        # Trigger a new scan and wait for results
        _shell(serial, "cmd wifi start-scan")
        time.sleep(3)
        raw = _shell(serial, "cmd wifi list-scan-results")
        networks = _parse_scan_results(raw)

    # Sort strongest first
    networks.sort(key=lambda n: n["rssi"], reverse=True)

    print(f"\n  WiFi Scan \u2014 {device.model}")
    print("  (trigger: cmd wifi start-scan)\n")

    if not networks:
        print("  No scan results available.")
        return

    col_ssid  = max(len(n["ssid"]) for n in networks)
    col_ssid  = max(col_ssid, 24)

    header = (
        f"  {'#':<4} "
        f"{'SSID':<{col_ssid}}  "
        f"{'Band':<9} "
        f"{'RSSI':<7} "
        f"{'Security'}"
    )
    print(header)
    print("  " + "\u2500" * (len(header) - 2))

    for idx, net in enumerate(networks, start=1):
        band     = _band_from_freq(net["freq"])
        security = _security_from_caps(net["caps"])
        ssid     = net["ssid"]
        rssi     = net["rssi"]
        print(
            f"  {idx:<4} "
            f"{ssid:<{col_ssid}}  "
            f"{band:<9} "
            f"{rssi:<7} "
            f"{security}"
        )

    print()


def action_wifi_profiles(device: DeviceInfo, forget: str | None) -> None:
    """
    List saved WiFi networks, or forget one by SSID or network ID.

    forget=None  : display all saved networks
    forget=<val> : forget the network identified by SSID string or numeric ID
    """
    serial = device.serial

    # ── Retrieve saved network list ───────────────────────────────────────────
    raw = _shell(serial, "cmd wifi list-networks")
    if not raw:
        raw = _shell(serial, "cmd wifi list-saved-networks")

    # Parse network list.
    # Expected format:
    #   Network Id  SSID  BSSID  Flags
    #   0           HomeNetwork  any  [CURRENT]
    networks: list[dict] = []
    for line in raw.splitlines():
        # Skip header
        if re.match(r"^\s*Network\s+Id", line, re.IGNORECASE):
            continue
        # Each line starts with a numeric network ID
        m = re.match(r"^\s*(\d+)\s+(.+?)\s+(any|\S+:\S+:\S+:\S+:\S+:\S+)\s*(.*)?$", line)
        if not m:
            # Simpler fallback: just ID and SSID
            m2 = re.match(r"^\s*(\d+)\s+(.+)$", line)
            if m2:
                net_id = int(m2.group(1))
                ssid   = m2.group(2).strip()
                flags  = ""
                networks.append({"id": net_id, "ssid": ssid, "flags": flags})
            continue
        net_id = int(m.group(1))
        ssid   = m.group(2).strip()
        flags  = (m.group(4) or "").strip()
        networks.append({"id": net_id, "ssid": ssid, "flags": flags})

    # ── Forget mode ───────────────────────────────────────────────────────────
    if forget is not None:
        target_id:   int | None  = None
        target_ssid: str | None  = None

        if re.match(r"^\d+$", forget.strip()):
            # User provided a numeric ID directly
            target_id   = int(forget.strip())
            target_ssid = next(
                (n["ssid"] for n in networks if n["id"] == target_id), None
            )
        else:
            # User provided an SSID — look it up
            match = next((n for n in networks if n["ssid"] == forget), None)
            if match is None:
                raise AdbError(
                    f"No saved network found with SSID \"{forget}\".\n"
                    "Run without --forget to list all saved networks."
                )
            target_id   = match["id"]
            target_ssid = match["ssid"]

        r = run(["adb", "-s", serial, "shell",
                 f"cmd wifi forget-network {target_id}"])
        if r.returncode != 0:
            raise AdbError(
                f"Failed to forget network id={target_id}: {r.stderr.strip()}"
            )

        label = f"{target_ssid} (id={target_id})" if target_ssid else f"id={target_id}"
        print(f"  [OK] Forgot network: {label}")
        return

    # ── List mode ─────────────────────────────────────────────────────────────
    print(f"\n  Saved WiFi Networks \u2014 {device.model}\n")

    if not networks:
        print("  No saved networks found.")
        return

    col_ssid = max(len(n["ssid"]) for n in networks)
    col_ssid = max(col_ssid, 22)

    header = (
        f"  {'#':<4} "
        f"{'ID':<4} "
        f"{'SSID':<{col_ssid}}  "
        f"{'Status'}"
    )
    print(header)
    print("  " + "\u2500" * (len(header) - 2))

    for idx, net in enumerate(networks):
        flags  = net["flags"].upper()
        status = "current" if "CURRENT" in flags else "\u2014"
        print(
            f"  {idx:<4} "
            f"{net['id']:<4} "
            f"{net['ssid']:<{col_ssid}}  "
            f"{status}"
        )

    print()
