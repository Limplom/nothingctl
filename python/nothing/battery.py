"""Battery health report for Nothing phones."""

from .device import run
from .models import DeviceInfo

# dumpsys battery → integer health codes
_HEALTH = {
    1: "Unknown",
    2: "Good",
    3: "Overheat",
    4: "Dead",
    5: "Over voltage",
    6: "Unspecified failure",
    7: "Cold",
}

# dumpsys battery → integer status codes
_STATUS = {
    1: "Unknown",
    2: "Charging",
    3: "Discharging",
    4: "Not charging",
    5: "Full",
}

# dumpsys battery → integer plugged codes
_PLUGGED = {
    0: "Not plugged",
    1: "AC",
    2: "USB",
    4: "Wireless",
}


def _parse_dumpsys_battery(output: str) -> dict[str, str]:
    """Return a flat key→value dict from 'dumpsys battery' output."""
    result: dict[str, str] = {}
    for line in output.splitlines():
        if ": " in line:
            key, _, value = line.partition(": ")
            result[key.strip()] = value.strip()
    return result


def _get_cycle_count(serial: str) -> str:
    """Try multiple sources for battery cycle count; return a display string."""
    # 1 — batterystats grep
    r = run(["adb", "-s", serial, "shell",
             "dumpsys batterystats | grep -E 'Charge cycle count'"])
    if r.returncode == 0 and r.stdout.strip():
        for line in r.stdout.strip().splitlines():
            if "Charge cycle count" in line and "=" in line:
                raw = line.split("=")[-1].strip().split()[0]
                try:
                    return str(int(raw))
                except ValueError:
                    pass

    # 2 — sysfs node
    r2 = run(["adb", "-s", serial, "shell",
              "cat /sys/class/power_supply/battery/cycle_count 2>/dev/null"])
    if r2.returncode == 0 and r2.stdout.strip():
        try:
            return str(int(r2.stdout.strip()))
        except ValueError:
            pass

    return "(not available on this kernel)"


def action_battery(device: DeviceInfo) -> None:
    """Display a battery health report for the connected Nothing phone."""
    # ── dumpsys battery ────────────────────────────────────────────────────
    r = run(["adb", "-s", device.serial, "shell", "dumpsys battery"])
    fields = _parse_dumpsys_battery(r.stdout) if r.returncode == 0 else {}

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

    level_str  = f"{level_raw} %" if level_raw is not None else "not available"
    status_str = _STATUS.get(status_raw, f"unknown ({status_raw})") if status_raw is not None else "not available"
    health_str = _HEALTH.get(health_raw, f"unknown ({health_raw})") if health_raw is not None else "not available"

    # Some devices (e.g. Phone 1) report power source as individual boolean fields
    # instead of a single numeric "plugged" field.
    if plugged_raw is not None:
        plugged_str = _PLUGGED.get(plugged_raw, f"unknown ({plugged_raw})")
    elif fields.get("AC powered") == "true":
        plugged_str = "AC"
    elif fields.get("USB powered") == "true":
        plugged_str = "USB"
    elif fields.get("Wireless powered") == "true":
        plugged_str = "Wireless"
    elif fields.get("Dock powered") == "true":
        plugged_str = "Dock"
    else:
        plugged_str = "Not plugged"

    if temp_raw is not None:
        temp_c = temp_raw / 10
        temp_str = f"{temp_c:.1f} \u00b0C"
    else:
        temp_str = "not available"

    if voltage_raw is not None:
        voltage_v = voltage_raw / 1000
        voltage_str = f"{voltage_v:.2f} V"
    else:
        voltage_str = "not available"

    # ── cycle count ────────────────────────────────────────────────────────
    cycle_str = _get_cycle_count(device.serial)

    # ── battery capacity ───────────────────────────────────────────────────
    # Try charge_full (current max, µAh) then charge_full_design (nominal, µAh).
    # Some MediaTek kernels report charge_full_design in a non-µAh unit — we
    # sanity-check and skip values that would convert to < 500 mAh.
    capacity_str: str | None = None
    for sysfs_node, label in [
        ("charge_full",        "current max"),
        ("charge_full_design", "design"),
    ]:
        r_cap = run(["adb", "-s", device.serial, "shell",
                     f"cat /sys/class/power_supply/battery/{sysfs_node} 2>/dev/null"])
        if r_cap.returncode == 0 and r_cap.stdout.strip():
            try:
                uah = int(r_cap.stdout.strip())
                mah = uah // 1000
                if mah >= 500:          # sanity check: a phone battery is ≥ 500 mAh
                    capacity_str = f"{mah} mAh ({label})"
                    break
            except ValueError:
                pass

    # ── output ────────────────────────────────────────────────────────────
    print(f"\n  Battery Report \u2014 {device.model}\n")
    print(f"  {'Level':<13}: {level_str}")
    print(f"  {'Status':<13}: {status_str}")
    print(f"  {'Health':<13}: {health_str}")
    print(f"  {'Temperature':<13}: {temp_str}")
    print(f"  {'Voltage':<13}: {voltage_str}")
    print(f"  {'Plugged':<13}: {plugged_str}")
    print()
    print(f"  {'Cycle count':<13}: {cycle_str}  (estimated \u2014 varies by kernel)")
    if capacity_str:
        print(f"  {'Capacity':<13}: {capacity_str}")
    print()
