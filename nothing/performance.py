"""CPU governor, I/O scheduler, and thermal profile management."""

from .backup import check_adb_root
from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# Governors and schedulers used per profile
_PROFILE_GOVERNOR = {
    "performance": "performance",
    "powersave":   "powersave",
}
_DEADLINE_SCHEDULERS = ("sda", "sdb", "sdc", "mmcblk0")


# ---------------------------------------------------------------------------
# Current state detection
# ---------------------------------------------------------------------------

def _get_cpu_governor(serial: str) -> str:
    """Read the scaling governor for cpu0 (representative of all CPUs)."""
    r = run(["adb", "-s", serial, "shell",
             "su -c 'cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor'"])
    if r.returncode == 0 and r.stdout.strip():
        return r.stdout.strip()
    return "unknown"


def _get_io_scheduler(serial: str) -> str:
    """Read the I/O scheduler for the first block device that responds."""
    for dev in _DEADLINE_SCHEDULERS:
        r = run(["adb", "-s", serial, "shell",
                 f"su -c 'cat /sys/block/{dev}/queue/scheduler 2>/dev/null'"])
        if r.returncode == 0 and r.stdout.strip():
            return r.stdout.strip()
    return "unknown"


def _get_thermal_profile(serial: str) -> str:
    """Read Nothing-specific thermal profile property (may not exist)."""
    r = run(["adb", "-s", serial, "shell",
             "getprop vendor.powerhal.profile 2>/dev/null"])
    if r.returncode == 0 and r.stdout.strip():
        return r.stdout.strip()
    return "(not available)"


def _print_current_state(serial: str) -> None:
    gov       = _get_cpu_governor(serial)
    scheduler = _get_io_scheduler(serial)
    thermal   = _get_thermal_profile(serial)
    print(f"\n  Current CPU governor : {gov}")
    print(f"  Current I/O scheduler: {scheduler}")
    print(f"  Thermal profile      : {thermal}")


# ---------------------------------------------------------------------------
# Applying profiles
# ---------------------------------------------------------------------------

def _count_cpus(serial: str) -> int:
    """Count online CPUs that have a cpufreq scaling_governor file."""
    r = run(["adb", "-s", serial, "shell",
             "su -c 'ls /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor 2>/dev/null'"
             ])
    if r.returncode != 0 or not r.stdout.strip():
        return 0
    return len(r.stdout.strip().splitlines())


def _apply_governor_loop(serial: str, governor: str) -> int:
    """Set scaling governor on all CPUs in a single shell command. Returns CPU count."""
    cmd = (
        "for CPU in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do "
        f"  su -c 'echo {governor} > $CPU' 2>/dev/null; "
        "done"
    )
    run(["adb", "-s", serial, "shell", cmd])
    return _count_cpus(serial)


def _apply_io_scheduler(serial: str, scheduler: str) -> list[str]:
    """Apply I/O scheduler to all block devices that exist. Returns list of applied devs."""
    applied = []
    for dev in _DEADLINE_SCHEDULERS:
        r = run(["adb", "-s", serial, "shell",
                 f"su -c 'echo {scheduler} > /sys/block/{dev}/queue/scheduler 2>/dev/null'"])
        # Verify it was actually applied
        check = run(["adb", "-s", serial, "shell",
                     f"su -c 'cat /sys/block/{dev}/queue/scheduler 2>/dev/null'"])
        if check.returncode == 0 and check.stdout.strip():
            applied.append(dev)
    return applied


def _detect_balanced_governor(serial: str) -> str:
    """
    Choose balanced governor based on SoC.
    MediaTek (ro.board.platform contains 'mt') -> prefer 'walt', fallback 'schedutil'.
    Qualcomm / other -> 'schedutil'.
    """
    r = run(["adb", "-s", serial, "shell", "getprop ro.board.platform"])
    platform = r.stdout.strip().lower() if r.returncode == 0 else ""

    if "mt" in platform:
        # Check if walt governor is available
        check = run(["adb", "-s", serial, "shell",
                     "su -c 'cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors 2>/dev/null'"])
        if check.returncode == 0 and "walt" in check.stdout.lower():
            return "walt"
    return "schedutil"


def _apply_profile(serial: str, profile: str) -> None:
    print(f"\nApplying '{profile}' profile...")

    if profile == "performance":
        cpu_count = _apply_governor_loop(serial, "performance")
        print(f"  Set governor to 'performance' on {cpu_count} CPUs.")
        applied = _apply_io_scheduler(serial, "deadline")
        if applied:
            print(f"  Set I/O scheduler to 'deadline' on: {', '.join(applied)}")
        else:
            print("  [WARN] Could not apply I/O scheduler (block devices not writable).")

    elif profile == "balanced":
        governor  = _detect_balanced_governor(serial)
        cpu_count = _apply_governor_loop(serial, governor)
        print(f"  Set governor to '{governor}' on {cpu_count} CPUs.")
        applied = _apply_io_scheduler(serial, "cfq")
        if not applied:
            # CFQ not available on all kernels — leave scheduler as-is
            print("  I/O scheduler unchanged (cfq not available on this kernel).")
        else:
            print(f"  Set I/O scheduler to 'cfq' on: {', '.join(applied)}")

    elif profile == "powersave":
        cpu_count = _apply_governor_loop(serial, "powersave")
        print(f"  Set governor to 'powersave' on {cpu_count} CPUs.")
        print("  (I/O scheduler left unchanged for powersave.)")

    else:
        raise AdbError(
            f"Unknown profile '{profile}'. "
            "Valid profiles: performance, balanced, powersave."
        )

    print()
    print("[WARN] Changes are NOT persistent — they will reset on reboot.")


# ---------------------------------------------------------------------------
# Interactive menu
# ---------------------------------------------------------------------------

_MENU = """\
  Select profile:
    [0] Performance  (max clocks, deadline I/O)
    [1] Balanced     (schedutil, default)
    [2] Powersave    (min clocks)
    [3] Show current state only
"""

_PROFILE_MAP = {
    "0": "performance",
    "1": "balanced",
    "2": "powersave",
}


def _interactive_menu(serial: str) -> str | None:
    """Show state + menu; return chosen profile name or None to exit."""
    _print_current_state(serial)
    print()
    print(_MENU)
    try:
        choice = input("  Select [1]: ").strip()
    except (EOFError, KeyboardInterrupt):
        print()
        return None

    if not choice:
        choice = "1"

    if choice == "3":
        return None  # already printed state, nothing more to do

    profile = _PROFILE_MAP.get(choice)
    if profile is None:
        print(f"[WARN] Invalid selection: {choice!r}. Aborted.")
        return None
    return profile


# ---------------------------------------------------------------------------
# Main action
# ---------------------------------------------------------------------------

def action_performance(device: DeviceInfo, profile: str | None) -> None:
    """
    Manage CPU governor / I/O scheduler / thermal profile.

    If profile is None, show current state and an interactive menu.
    Valid profile names: 'performance', 'balanced', 'powersave'.
    Requires Magisk root (ADB shell must have su access).
    """
    if not check_adb_root(device.serial):
        raise AdbError(
            "Root not available via ADB shell.\n"
            "Enable in Magisk: Settings -> Superuser access -> Apps and ADB."
        )

    if profile is None:
        profile = _interactive_menu(device.serial)
        if profile is None:
            return

    valid = {"performance", "balanced", "powersave"}
    if profile not in valid:
        raise AdbError(
            f"Unknown profile '{profile}'. "
            f"Valid options: {', '.join(sorted(valid))}"
        )

    _apply_profile(device.serial, profile)
    print(f"[OK] Profile '{profile}' applied.")
