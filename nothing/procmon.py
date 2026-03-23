"""Process monitoring, Doze status, and location management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _shell(serial: str, cmd: str) -> str:
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _setting(serial: str, namespace: str, key: str) -> str:
    r = run(["adb", "-s", serial, "shell", "settings", "get", namespace, key])
    val = r.stdout.strip() if r.returncode == 0 else ""
    return "" if val in ("null", "null\n", "null\r") else val


# ---------------------------------------------------------------------------
# UID helpers
# ---------------------------------------------------------------------------

_STATE_LABELS: dict[str, str] = {
    "S": "Sleeping",
    "R": "Running",
    "Z": "Zombie",
    "T": "Stopped",
    "D": "Disk sleep",
}


def _uid_numeric(user: str) -> int | None:
    """Convert Android user string to a numeric UID, or None if unknown."""
    if user == "root":
        return 0
    if user == "system":
        return 1000
    m = re.match(r"^u0_a(\d+)$", user)
    if m:
        return 10000 + int(m.group(1))
    m = re.match(r"^u0_i(\d+)$", user)
    if m:
        return 99000 + int(m.group(1))
    return None


def _is_user_app(user: str) -> bool:
    return bool(re.match(r"^u0_a\d+$", user))


def _is_isolated(user: str) -> bool:
    return bool(re.match(r"^u0_i\d+$", user))


def _is_system(user: str) -> bool:
    return user in ("root", "system") or re.match(r"^(shell|radio|log|nobody|nfc|"
                                                    r"bluetooth|wifi|camera|media|"
                                                    r"audioserver|cameraserver|"
                                                    r"credstore|keystore|"
                                                    r"statsd|storaged|"
                                                    r"inet|net_bt|net_bt_admin|"
                                                    r"net_raw|net_admin)$", user) is not None


# ---------------------------------------------------------------------------
# ps output parser
# ---------------------------------------------------------------------------

def _parse_ps(output: str) -> list[dict]:
    """Parse `ps -A -o PID,PPID,USER,NAME,S` output into a list of dicts."""
    processes: list[dict] = []
    lines = output.splitlines()
    # Skip header line(s)
    for line in lines:
        line = line.strip()
        if not line:
            continue
        # Skip the header
        if line.startswith("PID"):
            continue
        parts = line.split()
        if len(parts) < 5:
            continue
        try:
            pid = int(parts[0])
            ppid = int(parts[1])
        except ValueError:
            continue
        user = parts[2]
        state = parts[-1]
        # NAME is everything between USER and S (the last column)
        name = " ".join(parts[3:-1])
        processes.append({
            "pid": pid,
            "ppid": ppid,
            "user": user,
            "name": name,
            "state": state,
        })
    return processes


# ---------------------------------------------------------------------------
# 1. action_process_tree
# ---------------------------------------------------------------------------

def action_process_tree(device: DeviceInfo, package: str | None) -> None:
    """Show full process list with UID/PID/state, optionally filtered by package name."""

    raw = _shell(device.serial, "ps -A -o PID,PPID,USER,NAME,S")
    if not raw:
        raise AdbError("Failed to retrieve process list from device.")

    processes = _parse_ps(raw)

    title = f"  Process Tree — {device.model}"
    print()
    print(title)

    if package:
        # Filtered view
        filtered = [p for p in processes if package in p["name"]]
        print(f"  Filter: {package}")
        print()
        if not filtered:
            print(f"  No processes found matching '{package}'.")
            print()
            return

        col_pid   = "PID"
        col_ppid  = "PPID"
        col_uid   = "UID"
        col_name  = "Name"
        col_state = "State"
        print(f"  {col_pid:<6} {col_ppid:<6} {col_uid:<10} {col_name:<24} {col_state}")
        print(f"  {'─'*6} {'─'*6} {'─'*10} {'─'*24} {'─'*16}")

        for p in sorted(filtered, key=lambda x: x["pid"]):
            state_label = _STATE_LABELS.get(p["state"], p["state"])
            uid_str = p["user"]
            print(
                f"  {p['pid']:<6} {p['ppid']:<6} {uid_str:<10} "
                f"{p['name']:<24} {p['state']} ({state_label})"
            )
        print()

    else:
        # Summary / grouped view
        total = len(processes)
        user_apps   = [p for p in processes if _is_user_app(p["user"])]
        isolated    = [p for p in processes if _is_isolated(p["user"])]
        system_procs = [p for p in processes if _is_system(p["user"])]
        other_count = total - len(user_apps) - len(isolated) - len(system_procs)
        if other_count < 0:
            other_count = 0

        print()
        print(f"  Total: {total} processes")
        print()
        print(f"  System processes: {len(system_procs)}")
        print(f"  User apps:        {len(user_apps)}")
        print(f"  Isolated:         {len(isolated)}")
        print(f"  Other:            {other_count}")
        print()
        print("  User Apps (running):")
        print(f"  {'PID':<6} {'Name':<32} {'UID'}")
        print(f"  {'─'*6} {'─'*32} {'─'*12}")

        top = sorted(user_apps, key=lambda x: x["pid"])[:30]
        for p in top:
            print(f"  {p['pid']:<6} {p['name']:<32} {p['user']}")

        if len(user_apps) > 30:
            print(f"  ... ({len(user_apps) - 30} more)")
        print()


# ---------------------------------------------------------------------------
# 2. action_doze_status
# ---------------------------------------------------------------------------

def _parse_doze_state(dumpsys: str) -> str:
    m = re.search(r"mState\s*=\s*(\S+)", dumpsys)
    return m.group(1) if m else "UNKNOWN"


def _parse_light_state(dumpsys: str) -> str:
    m = re.search(r"mLightState\s*=\s*(\S+)", dumpsys)
    return m.group(1) if m else "UNKNOWN"


def _parse_screen_on(dumpsys: str) -> bool:
    m = re.search(r"mScreenOn\s*=\s*(true|false)", dumpsys, re.IGNORECASE)
    if m:
        return m.group(1).lower() == "true"
    # Also check for interactiveState
    m2 = re.search(r"Interactive:\s*(true|false)", dumpsys, re.IGNORECASE)
    if m2:
        return m2.group(1).lower() == "true"
    return False


def _parse_plugged_in(battery_dump: str) -> bool:
    m = re.search(r"AC powered:\s*(true|false)", battery_dump, re.IGNORECASE)
    if m and m.group(1).lower() == "true":
        return True
    m2 = re.search(r"USB powered:\s*(true|false)", battery_dump, re.IGNORECASE)
    if m2 and m2.group(1).lower() == "true":
        return True
    m3 = re.search(r"Wireless powered:\s*(true|false)", battery_dump, re.IGNORECASE)
    if m3 and m3.group(1).lower() == "true":
        return True
    return False


def _parse_whitelist(whitelist_output: str) -> list[str]:
    """Parse `dumpsys deviceidle whitelist` output into package names."""
    packages: list[str] = []
    for line in whitelist_output.splitlines():
        line = line.strip()
        # Android format: "system-excidle,com.example.app,10062"
        # or: "UID=10023: com.example.app" or just "com.example.app"
        if "," in line:
            parts = line.split(",")
            # Package is the second field in comma-separated format
            if len(parts) >= 2:
                pkg = parts[1].strip()
                if "." in pkg:
                    packages.append(pkg)
        else:
            m = re.match(r"(?:UID=\d+:\s*)?([a-z][a-z0-9_.]+)", line, re.IGNORECASE)
            if m:
                pkg = m.group(1)
                if "." in pkg:
                    packages.append(pkg)
    return packages


_DOZE_STATE_LABELS: dict[str, str] = {
    "ACTIVE":           "active (not in Doze)",
    "IDLE_PENDING":     "idle pending",
    "SENSING":          "sensing",
    "LOCATING":         "locating",
    "IDLE":             "IDLE (full Doze)",
    "IDLE_MAINTENANCE": "idle maintenance",
    "OVERRIDE":         "override",
    "UNKNOWN":          "unknown",
}


def action_doze_status(
    device: DeviceInfo,
    whitelist_add: str | None,
    whitelist_remove: str | None,
) -> None:
    """Show Doze mode status and manage the battery-optimization whitelist."""

    serial = device.serial

    # Handle whitelist mutations first
    if whitelist_add:
        result = _shell(serial, f"dumpsys deviceidle whitelist +{whitelist_add}")
        print()
        print(f"  Whitelist add: {whitelist_add}")
        if result:
            print(f"  Response: {result}")
        else:
            print("  Done (no output from device).")
        print()

    if whitelist_remove:
        result = _shell(serial, f"dumpsys deviceidle whitelist -{whitelist_remove}")
        print()
        print(f"  Whitelist remove: {whitelist_remove}")
        if result:
            print(f"  Response: {result}")
        else:
            print("  Done (no output from device).")
        print()

    # Gather data
    deviceidle_dump = _shell(serial, "dumpsys deviceidle")
    whitelist_raw   = _shell(serial, "dumpsys deviceidle whitelist")
    battery_dump    = _shell(serial, "dumpsys battery")

    state       = _parse_doze_state(deviceidle_dump)
    light_state = _parse_light_state(deviceidle_dump)
    screen_on   = _parse_screen_on(deviceidle_dump)
    plugged_in  = _parse_plugged_in(battery_dump)
    whitelist   = list(dict.fromkeys(_parse_whitelist(whitelist_raw)))  # deduplicate, preserve order

    state_label = _DOZE_STATE_LABELS.get(state, state.lower())

    title = f"  Doze Status — {device.model}"
    print()
    print(title)
    print()
    print(f"  {'State':<16}: {state} ({state_label})")
    print(f"  {'Light State':<16}: {light_state}")
    print(f"  {'Screen On':<16}: {'yes' if screen_on else 'no'}")

    if plugged_in:
        print(f"  {'Plugged In':<16}: yes (Doze requires battery)")
    else:
        print(f"  {'Plugged In':<16}: no")

    print()
    print(f"  Whitelist (battery optimization exempt):")
    if whitelist:
        for pkg in sorted(whitelist):
            print(f"  {pkg}")
        print(f"  ... ({len(whitelist)} total)")
    else:
        print("  (empty)")
    print()


# ---------------------------------------------------------------------------
# 3. action_location
# ---------------------------------------------------------------------------

_LOCATION_MODES: dict[str, str] = {
    "0": "Off",
    "1": "Sensors only (GPS)",
    "2": "Battery saving (Network only)",
    "3": "High Accuracy (GPS + Network)",
    "4": "Device only (GPS)",
}

_MODE_MAP: dict[str, str] = {
    "off":      "0",
    "gps":      "1",
    "device":   "1",
    "sensors":  "1",
    "battery":  "2",
    "on":       "3",
    "high":     "3",
    "accuracy": "3",
}


def _parse_last_known_locations(dump: str) -> list[dict]:
    """
    Parse Last Known Locations from `dumpsys location`.
    Returns a list of dicts with keys: provider, lat, lon, accuracy, age.
    """
    results: list[dict] = []
    in_section = False
    current_provider: str | None = None

    for line in dump.splitlines():
        if "Last Known Locations" in line:
            in_section = True
            continue
        if not in_section:
            continue
        # Stop at next major section (empty line after content, or new header)
        stripped = line.strip()
        if not stripped and current_provider is not None:
            # blank line may end the section — keep going a bit
            continue
        if stripped and not stripped.startswith("passive") and \
                not stripped.startswith("gps") and \
                not stripped.startswith("network") and \
                not stripped.startswith("fused") and \
                re.match(r"^[A-Z]", stripped):
            # New header — exit
            break

        # Provider line: "gps: Location[gps ..." or "gps: null"
        m_prov = re.match(r"^\s*(gps|network|passive|fused)\s*:\s*(.*)", line, re.IGNORECASE)
        if m_prov:
            current_provider = m_prov.group(1).lower()
            rest = m_prov.group(2)
            if "null" in rest.lower():
                results.append({"provider": current_provider, "lat": None, "lon": None,
                                 "accuracy": None, "age": None})
                current_provider = None
                continue
            # Try to parse lat/lon from rest
            entry = _parse_location_line(current_provider, rest)
            if entry:
                results.append(entry)
            current_provider = None

    return results


def _parse_location_line(provider: str, text: str) -> dict | None:
    """Extract lat, lon, accuracy, age from a location dump line."""
    # Pattern: "Location[gps 52.5200,13.4050 acc=8 et=...]"
    m = re.search(r"([-\d.]+),([-\d.]+)", text)
    if not m:
        return None
    lat = float(m.group(1))
    lon = float(m.group(2))

    accuracy: str | None = None
    m_acc = re.search(r"acc(?:uracy)?[=\s]+([\d.]+)", text, re.IGNORECASE)
    if m_acc:
        accuracy = m_acc.group(1) + "m"

    age: str | None = None
    # et= is elapsed time in ns or ms
    m_age = re.search(r"et=(\S+)", text, re.IGNORECASE)
    if m_age:
        age = m_age.group(1)

    return {"provider": provider, "lat": lat, "lon": lon, "accuracy": accuracy, "age": age}


def _format_coord(lat: float, lon: float) -> str:
    ns = "N" if lat >= 0 else "S"
    ew = "E" if lon >= 0 else "W"
    return f"{abs(lat):.4f}° {ns}, {abs(lon):.4f}° {ew}"


def _parse_providers(dump: str) -> dict[str, bool]:
    """Return dict of provider name → enabled status."""
    providers: dict[str, bool] = {}
    for line in dump.splitlines():
        # e.g. "  gps provider [enabled]" or "GpsLocationProvider[...]"
        m = re.search(r"(gps|network|passive)\s+provider[^:]*?(\[|:)?\s*(enabled|disabled)",
                      line, re.IGNORECASE)
        if m:
            providers[m.group(1).lower()] = m.group(3).lower() == "enabled"
    return providers


def _parse_fine_location_apps(appops_output: str) -> list[str]:
    """Parse `cmd appops query-op android:fine_location allow` output."""
    packages: list[str] = []
    for line in appops_output.splitlines():
        line = line.strip()
        # Lines like: "Package com.example.app uid=10023: ALLOW"
        m = re.match(r"(?:Package\s+)?([a-z][a-z0-9_.]+)\s+uid=", line, re.IGNORECASE)
        if m and "." in m.group(1):
            packages.append(m.group(1))
        # Some Android versions: "  com.example.app"
        elif re.match(r"^[a-z][a-z0-9_.]+\.[a-z0-9_.]+$", line, re.IGNORECASE):
            packages.append(line)
    return packages


def action_location(device: DeviceInfo, mode: str | None) -> None:
    """Show GPS/location status and optionally set the location mode."""

    serial = device.serial

    if mode is not None:
        mode_lower = mode.lower()
        numeric = _MODE_MAP.get(mode_lower)
        if numeric is None:
            print()
            print(f"  Unknown mode '{mode}'. Valid options: off, gps, device, sensors, "
                  f"battery, on, high, accuracy")
            print()
            return
        r = run(["adb", "-s", serial, "shell", "settings", "put", "secure",
                 "location_mode", numeric])
        print()
        if r.returncode == 0:
            label = _LOCATION_MODES.get(numeric, numeric)
            print(f"  Location mode set to: {label}")
        else:
            err = r.stderr.strip() if r.stderr else "unknown error"
            print(f"  Failed to set location mode: {err}")
        print()
        return

    # Read-only status
    raw_mode   = _setting(serial, "secure", "location_mode")
    location_dump = _shell(serial, "dumpsys location")
    appops_out = _shell(serial, "cmd appops query-op android:fine_location allow")

    mode_label = _LOCATION_MODES.get(raw_mode, f"Unknown (mode={raw_mode})")
    providers  = _parse_providers(location_dump)
    locations  = _parse_last_known_locations(location_dump)
    fine_apps  = _parse_fine_location_apps(appops_out)

    title = f"  Location — {device.model}"
    print()
    print(title)
    print()
    print(f"  {'Mode':<16}: {mode_label}")

    gps_status     = "enabled" if providers.get("gps", False) else "disabled"
    network_status = "enabled" if providers.get("network", False) else "disabled"
    print(f"  {'GPS Provider':<16}: {gps_status}")
    print(f"  {'Network Provider':<16}: {network_status}")

    print()
    print("  Last Known Location:")
    if locations:
        for loc in locations:
            pname = loc["provider"].capitalize()
            if loc["lat"] is None:
                print(f"    {pname:<9}: (none)")
            else:
                coord_str = _format_coord(loc["lat"], loc["lon"])
                parts = [coord_str]
                if loc["accuracy"]:
                    parts.append(f"accuracy: {loc['accuracy']}")
                if loc["age"]:
                    parts.append(f"age: {loc['age']}")
                print(f"    {pname:<9}: {',  '.join(parts)}")
    else:
        print("    (not available)")

    print()
    app_sample = fine_apps[:8]
    print(f"  Apps with Location Permission (fine): {len(fine_apps)}")
    if app_sample:
        print(f"    {', '.join(app_sample)}" + (" ..." if len(fine_apps) > 8 else ""))
    print()
