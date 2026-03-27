"""Read and write Android system properties via ADB."""

from .backup import check_adb_root
from .device import adb_shell, run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Property grouping
# ---------------------------------------------------------------------------

# Ordered list of prefix groups; first match wins.
_PREFIX_GROUPS = [
    "ro.product",
    "ro.build",
    "ro.boot",
    "ro.hardware",
    "persist",
    "sys",
    "gsm",
    "net",
    "wifi",
]


def _group_key(prop_name: str) -> str:
    """Return the display-group name for a property key."""
    for prefix in _PREFIX_GROUPS:
        if prop_name.startswith(prefix):
            return prefix
    return "other"


# ---------------------------------------------------------------------------
# Parsing helpers
# ---------------------------------------------------------------------------

def _parse_getprop(output: str) -> list[tuple[str, str]]:
    """
    Parse `adb shell getprop` output.

    Lines are formatted as:  [key]: [value]
    Returns a list of (key, value) tuples preserving declaration order.
    """
    props: list[tuple[str, str]] = []
    for line in output.splitlines():
        line = line.strip()
        if not line.startswith("["):
            continue
        # Format: [key]: [value]
        bracket_close = line.find("]:")
        if bracket_close == -1:
            continue
        key = line[1:bracket_close]
        rest = line[bracket_close + 2:].strip()
        # Value is wrapped in [ … ]
        if rest.startswith("[") and rest.endswith("]"):
            value = rest[1:-1]
        else:
            value = rest
        if key:
            props.append((key, value))
    return props


# ---------------------------------------------------------------------------
# action_prop_get
# ---------------------------------------------------------------------------

def action_prop_get(device: DeviceInfo, key: str | None) -> None:
    """
    Read system properties.

    If *key* is given, print its value directly.
    If *key* is None, dump all properties grouped by prefix.
    """
    if key:
        value = adb_shell(f"getprop {key}", device.serial)
        if not value:
            print(f"  [WARN] Property '{key}' is empty or not set.")
        else:
            print(f"  {key} = {value}")
        return

    # Full dump
    r = run(["adb", "-s", device.serial, "shell", "getprop"])
    if r.returncode != 0:
        raise AdbError(f"getprop failed: {r.stderr.strip()}")

    props = _parse_getprop(r.stdout)
    if not props:
        raise AdbError("getprop returned no output.")

    # Bucket into groups, preserving order within each group
    grouped: dict[str, list[tuple[str, str]]] = {g: [] for g in _PREFIX_GROUPS}
    grouped["other"] = []

    for key_name, value in props:
        group = _group_key(key_name)
        grouped[group].append((key_name, value))

    print(f"\n  System Properties — Nothing {device.model}\n")

    group_order = _PREFIX_GROUPS + ["other"]
    for group in group_order:
        entries = grouped.get(group, [])
        if not entries:
            continue

        print(f"  [{group}.*]")
        # Align values: find longest key in this group
        max_len = max(len(k) for k, _ in entries)
        for k, v in entries:
            print(f"    {k:<{max_len}}  = {v}")
        print()


# ---------------------------------------------------------------------------
# action_prop_set
# ---------------------------------------------------------------------------

def action_prop_set(device: DeviceInfo, key: str, value: str) -> None:
    """
    Write a system property via root su.

    Requires Magisk root. Most ro.* properties reset on reboot; use persist.*
    for persistent changes.
    """
    if not check_adb_root(device.serial):
        raise AdbError(
            "Root not available via ADB shell.\n"
            "Enable in Magisk: Settings -> Superuser access -> Apps and ADB."
        )

    print(
        "  NOTE: Most ro.* properties reset on reboot. "
        "Use persist.* for persistent changes."
    )

    if key.startswith("ro."):
        print(
            f"  [WARN] ro.* properties are read-only at the system level "
            f"— this may not persist."
        )

    r = run(["adb", "-s", device.serial, "shell", f'su -c "setprop {key} {value}"'])
    if r.returncode != 0:
        stderr = r.stderr.strip() or r.stdout.strip()
        raise AdbError(f"setprop {key} failed: {stderr}")

    print(f"  [OK] Set {key} = {value}")
