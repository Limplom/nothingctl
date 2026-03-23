"""Battery statistics and charging control for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Shared lookup tables (mirrors battery.py for consistency)
# ---------------------------------------------------------------------------

_HEALTH: dict[int, str] = {
    1: "Unknown",
    2: "Good",
    3: "Overheat",
    4: "Dead",
    5: "Over voltage",
    6: "Unspecified failure",
    7: "Cold",
}

_STATUS: dict[int, str] = {
    1: "Unknown",
    2: "Charging",
    3: "Discharging",
    4: "Not charging",
    5: "Full",
}

_PLUGGED: dict[int, str] = {
    0: "Not plugged",
    1: "AC",
    2: "USB",
    4: "Wireless",
}

# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _shell(serial: str, cmd: str) -> str:
    """Run an adb shell command; return stdout stripped, empty string on failure."""
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _parse_dumpsys_battery(output: str) -> dict[str, str]:
    """Return a flat key→value dict from 'dumpsys battery' output."""
    result: dict[str, str] = {}
    for line in output.splitlines():
        if ": " in line:
            key, _, value = line.partition(": ")
            result[key.strip()] = value.strip()
    return result


def _seconds_to_hms(seconds: float) -> str:
    """Convert a float number of seconds to 'HH:MM:SS'."""
    total = int(seconds)
    h = total // 3600
    m = (total % 3600) // 60
    s = total % 60
    return f"{h:02d}:{m:02d}:{s:02d}"


def _parse_batterystats(output: str) -> list[tuple[str, float, int]]:
    """
    Parse 'dumpsys batterystats --charged' and return a list of
    (package_name, total_wake_seconds, total_wakelocks) for user apps
    (UID >= 10000), sorted by total_wake_seconds descending.

    batterystats output groups data under UID headers like:
        UID 10123:
          ...
          Wake lock com.example.app: 1m 23s 400ms (45 times) ...

    We accumulate all wakelock entries found under each UID block.
    """
    # Map uid -> {package: (wake_secs, count)}
    uid_data: dict[int, dict[str, tuple[float, int]]] = {}
    current_uid: int | None = None

    # Regex: "  Wake lock <name>: <time_expr> (<N> times)"
    # Time expressions look like: "1h 2m 3s 400ms", "2m 3s", "45s 200ms", etc.
    wl_re = re.compile(
        r"Wake lock\s+([^\s:][^:]*?):\s+"
        r"((?:\d+h\s*)?(?:\d+m\s*)?(?:\d+s\s*)?(?:\d+ms\s*)?)"
        r"\((\d+)\s+times?\)",
        re.IGNORECASE,
    )
    uid_re = re.compile(r"^\s*UID\s+(\d+):", re.IGNORECASE)

    def _parse_time(expr: str) -> float:
        """Convert a batterystats time expression to total seconds."""
        total = 0.0
        for amount, unit in re.findall(r"(\d+)\s*(h|m|s|ms)", expr):
            n = int(amount)
            if unit == "h":
                total += n * 3600
            elif unit == "m":
                total += n * 60
            elif unit == "s":
                total += n
            elif unit == "ms":
                total += n / 1000
        return total

    for line in output.splitlines():
        uid_match = uid_re.match(line)
        if uid_match:
            current_uid = int(uid_match.group(1))
            if current_uid not in uid_data:
                uid_data[current_uid] = {}
            continue

        if current_uid is None or current_uid < 10000:
            continue

        wl_match = wl_re.search(line)
        if wl_match:
            pkg   = wl_match.group(1).strip()
            secs  = _parse_time(wl_match.group(2))
            count = int(wl_match.group(3))
            prev_secs, prev_count = uid_data[current_uid].get(pkg, (0.0, 0))
            uid_data[current_uid][pkg] = (prev_secs + secs, prev_count + count)

    # Flatten to a single list of (package, wake_secs, wakelock_count)
    flat: list[tuple[str, float, int]] = []
    for uid_pkgs in uid_data.values():
        for pkg, (secs, count) in uid_pkgs.items():
            flat.append((pkg, secs, count))

    flat.sort(key=lambda x: x[1], reverse=True)
    return flat


# ---------------------------------------------------------------------------
# 1. Battery Stats
# ---------------------------------------------------------------------------

def action_battery_stats(device: DeviceInfo) -> None:
    """Show per-app battery drain (wakelock times) and charge cycles."""
    s = device.serial

    # ── dumpsys battery ────────────────────────────────────────────────────
    r_bat = run(["adb", "-s", s, "shell", "dumpsys battery"])
    fields = _parse_dumpsys_battery(r_bat.stdout) if r_bat.returncode == 0 else {}

    def _int(key: str) -> int | None:
        try:
            return int(fields[key])
        except (KeyError, ValueError):
            return None

    level_raw   = _int("level")
    status_raw  = _int("status")
    health_raw  = _int("health")
    temp_raw    = _int("temperature")
    voltage_raw = _int("voltage")
    plugged_raw = _int("plugged")

    level_str   = f"{level_raw} %" if level_raw is not None else "n/a"
    health_str  = _HEALTH.get(health_raw, f"unknown ({health_raw})") if health_raw is not None else "n/a"

    # Charging status with plug type appended when charging
    if status_raw is not None:
        status_str = _STATUS.get(status_raw, f"unknown ({status_raw})")
        if status_raw == 2 and plugged_raw is not None:  # Charging
            plug_label = _PLUGGED.get(plugged_raw, "")
            if plug_label and plug_label != "Not plugged":
                status_str = f"{status_str} ({plug_label})"
    elif fields.get("AC powered") == "true":
        status_str = "Charging (AC)"
    elif fields.get("USB powered") == "true":
        status_str = "Charging (USB)"
    elif fields.get("Wireless powered") == "true":
        status_str = "Charging (Wireless)"
    else:
        status_str = "n/a"

    if temp_raw is not None:
        temp_str = f"{temp_raw / 10:.1f} \u00b0C"
    else:
        temp_str = "n/a"

    if voltage_raw is not None:
        voltage_str = f"{voltage_raw / 1000:.2f} V"
    else:
        voltage_str = "n/a"

    # ── charge cycles ──────────────────────────────────────────────────────
    cycle_raw = _shell(s, "cat /sys/class/power_supply/battery/cycle_count 2>/dev/null")
    if cycle_raw:
        try:
            cycle_str = str(int(cycle_raw))
        except ValueError:
            cycle_str = "(not available)"
    else:
        cycle_str = "(not available on this device)"

    # ── baseband (context only) ─────────────────────────────────────────────
    baseband = _shell(s, "getprop gsm.version.baseband")
    # Multi-value props return comma-separated list; take the first non-empty entry.
    if "," in baseband:
        baseband = next((b.strip() for b in baseband.split(",") if b.strip()), baseband)

    # ── dumpsys batterystats --charged ─────────────────────────────────────
    r_stats = run(["adb", "-s", s, "shell", "dumpsys batterystats --charged"])
    if r_stats.returncode == 0 and r_stats.stdout.strip():
        app_drain = _parse_batterystats(r_stats.stdout)
    else:
        app_drain = []

    top_apps = app_drain[:10]

    # ── output ────────────────────────────────────────────────────────────
    print(f"\n  Battery Stats \u2014 {device.model}\n")

    print(f"  {'Level':<14}: {level_str}")
    print(f"  {'Status':<14}: {status_str}")
    print(f"  {'Temperature':<14}: {temp_str}")
    print(f"  {'Voltage':<14}: {voltage_str}")
    print(f"  {'Health':<14}: {health_str}")
    print(f"  {'Charge Cycles':<14}: {cycle_str}")
    if baseband:
        print(f"  {'Baseband':<14}: {baseband}")
    print()

    print("  Top App Drain (since last charge):")
    if top_apps:
        header_pkg  = "App"
        header_time = "Wake Time"
        header_wl   = "Wakelocks"
        print(f"  {'#':<4}  {header_pkg:<36}  {header_time:<12}  {header_wl}")
        print(f"  {'-'*4}  {'-'*36}  {'-'*12}  {'-'*9}")
        for rank, (pkg, secs, count) in enumerate(top_apps, 1):
            time_str = _seconds_to_hms(secs)
            print(f"  {rank:<4}  {pkg:<36}  {time_str:<12}  {count}")
    else:
        print("  (no wakelock data available — try again after using the device)")

    print()


# ---------------------------------------------------------------------------
# 2. Charging Control
# ---------------------------------------------------------------------------

# Ordered list of sysfs paths to try for charge limit, most preferred first.
_CHARGE_LIMIT_PATHS: list[str] = [
    "/sys/class/power_supply/battery/charge_control_end_threshold",
    "/sys/class/power_supply/battery/charge_cutoff_percent",
]


def action_charging_control(device: DeviceInfo, limit: int | None) -> None:
    """Read or set the charge limit via sysfs on the Nothing phone."""
    s = device.serial

    # ── Detect which sysfs path exists ────────────────────────────────────
    active_path: str | None = None
    for path in _CHARGE_LIMIT_PATHS:
        probe = _shell(s, f"[ -f {path} ] && echo yes || echo no")
        if probe == "yes":
            active_path = path
            break

    if active_path is None:
        print(
            f"\n  Charge limit control is not supported on {device.model}.\n"
            "  The required sysfs node was not found:\n"
        )
        for path in _CHARGE_LIMIT_PATHS:
            print(f"    {path}")
        print(
            "\n  This feature requires a kernel that exposes one of the above nodes.\n"
            "  Custom kernels (e.g. Sultan, Asus) sometimes add this capability.\n"
        )
        return

    # ── Read-only mode ─────────────────────────────────────────────────────
    if limit is None:
        current = _shell(s, f"cat {active_path} 2>/dev/null")
        print(f"\n  Charge Limit \u2014 {device.model}\n")
        if current:
            try:
                pct = int(current)
                print(f"  {'Current limit':<16}: {pct} %")
            except ValueError:
                print(f"  {'Current limit':<16}: {current} (raw)")
        else:
            print(f"  {'Current limit':<16}: (could not read)")
        print(f"  {'Sysfs path':<16}: {active_path}")
        print()
        return

    # ── Validate the requested value ──────────────────────────────────────
    if not (20 <= limit <= 100):
        raise AdbError(
            f"Charge limit must be between 20 and 100 (got {limit})."
        )

    # ── Write via su -c (requires root) ───────────────────────────────────
    write_cmd = f"su -c 'echo {limit} > {active_path}'"
    r = run(["adb", "-s", s, "shell", write_cmd])

    if r.returncode != 0 or (r.stderr and "not found" in r.stderr.lower()):
        err = (r.stderr or r.stdout).strip()
        raise AdbError(
            f"Failed to set charge limit to {limit} % on {device.model}.\n"
            f"  Root access is required. Error: {err or '(no details)'}"
        )

    # Verify the write took effect
    verify = _shell(s, f"cat {active_path} 2>/dev/null")
    try:
        written = int(verify)
    except (ValueError, TypeError):
        written = None

    print(f"\n  Charge Limit \u2014 {device.model}\n")
    if written == limit:
        print(f"  Charge limit set to {limit} % successfully.")
    elif written is not None:
        print(
            f"  Write appeared to succeed but device reports {written} %\n"
            f"  (requested {limit} %). The kernel may have clamped the value."
        )
    else:
        print(
            f"  Write command returned success but could not verify the new value.\n"
            f"  Requested limit: {limit} %"
        )
    print(f"  Sysfs path: {active_path}")
    print()
