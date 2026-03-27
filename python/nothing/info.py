"""Comprehensive device dashboard for Nothing phones."""

from .device import run
from .models import DeviceInfo

# Map raw ro.board.platform / ro.product.board values to human-friendly SoC names.
_SOC_NAMES: dict[str, str] = {
    # MediaTek
    "mt6886":  "Dimensity 7200 Pro",
    "mt6878":  "Dimensity 7300 Pro",
    "mt6893":  "Dimensity 1200",
    "mt6983":  "Dimensity 9000",
    # Qualcomm — SM model numbers
    "sm6375":  "Snapdragon 778G+",
    "sm7325":  "Snapdragon 778G",
    "sm7435":  "Snapdragon 7s Gen 3",
    "sm8475":  "Snapdragon 8+ Gen 1",
    "sm8550":  "Snapdragon 8 Gen 2",
    "sm8650":  "Snapdragon 8 Gen 3",
    # Qualcomm — codename platform strings (used by some Nothing builds)
    "lahaina": "Snapdragon 778G+",   # Nothing Phone (1) reports this platform string
    "taro":    "Snapdragon 8+ Gen 1",
    "kalama":  "Snapdragon 8 Gen 2",
    "pineapple": "Snapdragon 8 Gen 3",
}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _prop(serial: str, prop: str) -> str:
    """Read a single Android system property; return empty string on failure."""
    r = run(["adb", "-s", serial, "shell", f"getprop {prop}"])
    if r.returncode == 0:
        return r.stdout.strip()
    return ""


def _shell(serial: str, cmd: str) -> str:
    """Run an adb shell command; return stdout stripped, or '' on failure."""
    r = run(["adb", "-s", serial, "shell", cmd])
    if r.returncode == 0:
        return r.stdout.strip()
    return ""


def _kb_to_gb(kb: int) -> str:
    return f"{kb / 1024 / 1024:.1f} GB"


def _parse_meminfo(output: str) -> str:
    """Parse /proc/meminfo MemTotal line → human-readable GB string."""
    for line in output.splitlines():
        if line.startswith("MemTotal:"):
            parts = line.split()
            # parts = ["MemTotal:", "<value>", "kB"]
            try:
                kb = int(parts[1])
                return _kb_to_gb(kb)
            except (IndexError, ValueError):
                pass
    return "not available"


def _parse_df(output: str) -> str:
    """
    Parse the last line of 'df <mount>' output.
    Columns: Filesystem  1K-blocks  Used  Available  Use%  Mounted
    Returns '<used> GB used of <total> GB' or 'not available'.
    """
    lines = [l for l in output.splitlines() if l.strip()]
    if not lines:
        return "not available"
    parts = lines[-1].split()
    # Need at least 4 columns: Filesystem, 1K-blocks, Used, Available
    if len(parts) < 4:
        return "not available"
    try:
        total_kb = int(parts[1])
        used_kb  = int(parts[2])
        return f"{_kb_to_gb(used_kb)} used of {_kb_to_gb(total_kb)}"
    except (ValueError, IndexError):
        return "not available"


def _resolve_soc(serial: str) -> str:
    """Return a human-readable SoC string using known platform mappings."""
    platform = _prop(serial, "ro.board.platform").lower()
    board    = _prop(serial, "ro.product.board").lower()

    for raw in (platform, board):
        if raw in _SOC_NAMES:
            return f"{raw} ({_SOC_NAMES[raw]})"

    # Return the most informative raw value we have
    return platform or board or "not available"


def _imei(serial: str) -> str:
    """Attempt to retrieve IMEI; fall back to ro.serialno, then 'not available'."""
    r = run(["adb", "-s", serial, "shell",
             "service call iphonesubinfo 1 | grep -o \"'[^']*'\" | tr -d \"' \\n\""])
    if r.returncode == 0:
        candidate = r.stdout.strip()
        # A real IMEI is 15 digits
        digits = "".join(c for c in candidate if c.isdigit())
        if len(digits) >= 14:
            return digits[:15]

    # Fallback: serial number (not IMEI, but better than nothing)
    sn = _prop(serial, "ro.serialno")
    if sn:
        return f"{sn}  (serial number, IMEI unavailable)"

    return "not available"


# ---------------------------------------------------------------------------
# Action
# ---------------------------------------------------------------------------

def action_info(device: DeviceInfo, serial_fastboot: str | None = None) -> None:
    """Print a comprehensive device dashboard for the connected Nothing phone."""
    s = device.serial

    # ── Software ──────────────────────────────────────────────────────────
    android_ver     = _prop(s, "ro.build.version.release") or "not available"
    firmware        = _prop(s, "ro.build.display.id")      or "not available"
    security_patch  = _prop(s, "ro.build.version.security_patch") or "not available"
    kernel          = _shell(s, "uname -r")                or "not available"

    # ── Hardware ──────────────────────────────────────────────────────────
    soc = _resolve_soc(s)

    meminfo_raw = _shell(s, "cat /proc/meminfo | grep MemTotal")
    ram = _parse_meminfo(meminfo_raw) if meminfo_raw else "not available"

    df_data   = _shell(s, "df /data | tail -1")
    df_sdcard = _shell(s, "df /sdcard | tail -1")
    storage_data   = _parse_df(df_data)   if df_data   else "not available"
    storage_sdcard = _parse_df(df_sdcard) if df_sdcard else "not available"

    # ── Identity ──────────────────────────────────────────────────────────
    serial_num  = _prop(s, "ro.serialno") or "not available"
    bootloader  = _prop(s, "ro.bootloader") or "not available"
    imei        = _imei(s)

    # ── Connection ────────────────────────────────────────────────────────
    adb_mode = "Wireless (TCP/IP)" if ":" in s else "USB"

    # ── Bootloader lock status (fastboot only) ────────────────────────────
    lock_status: str
    if serial_fastboot:
        r_fb = run(["fastboot", "-s", serial_fastboot, "getvar", "unlocked"])
        fb_out = (r_fb.stdout + r_fb.stderr).lower()
        if "unlocked: yes" in fb_out or "unlocked: true" in fb_out:
            lock_status = "Unlocked"
        elif "unlocked: no" in fb_out or "unlocked: false" in fb_out:
            lock_status = "Locked"
        else:
            lock_status = "not available"
    else:
        lock_status = "Run in fastboot mode to check (fastboot getvar unlocked)"

    # ── Output ────────────────────────────────────────────────────────────
    sep = "\u2500" * 49   # horizontal rule

    print(f"\n  Device Info \u2014 {device.model}  [{device.codename}]\n")

    print(f"  \u2500\u2500 Software {sep}")
    print(f"  {'Android':<15}: {android_ver}")
    print(f"  {'Firmware':<15}: {firmware}")
    print(f"  {'Security patch':<15}: {security_patch}")
    print(f"  {'Kernel':<15}: {kernel}")

    print(f"\n  \u2500\u2500 Hardware {sep}")
    print(f"  {'SoC':<15}: {soc}")
    print(f"  {'RAM':<15}: {ram}")
    print(f"  {'Storage /data':<15}: {storage_data}")
    print(f"  {'Storage /sdcard':<15}: {storage_sdcard}")

    print(f"\n  \u2500\u2500 Identity {sep}")
    print(f"  {'Serial (ADB)':<15}: {serial_num}")
    print(f"  {'Bootloader':<15}: {bootloader}")
    print(f"  {'IMEI':<15}: {imei}")

    print(f"\n  \u2500\u2500 Connection {sep}")
    print(f"  {'ADB mode':<15}: {adb_mode}")
    print(f"  {'Active slot':<15}: {device.current_slot or 'not available'}")

    print(f"\n  \u2500\u2500 Bootloader {sep}")
    print(f"  {'Lock status':<15}: {lock_status}")
    print()
