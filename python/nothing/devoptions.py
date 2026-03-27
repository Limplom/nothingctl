"""Developer Options management for Nothing phones."""

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Settings helpers
# ---------------------------------------------------------------------------

def _get_setting(serial: str, namespace: str, key: str) -> str:
    """Read an Android setting; return empty string if unavailable."""
    r = run(["adb", "-s", serial, "shell", "settings", "get", namespace, key])
    val = r.stdout.strip() if r.returncode == 0 else ""
    return "" if val in ("null", "null\n", "null\r") else val


def _put_setting(serial: str, namespace: str, key: str, value: str) -> None:
    """Write an Android setting; raise AdbError on failure."""
    r = run(["adb", "-s", serial, "shell", "settings", "put", namespace, key, value])
    if r.returncode != 0:
        raise AdbError(
            f"settings put {namespace} {key} {value} failed: {r.stderr.strip()}"
        )


# ---------------------------------------------------------------------------
# Developer options menu definition
# ---------------------------------------------------------------------------

# Each entry is one of:
#   ("Description",  namespace, key, value)            — single setting
#   ("Description",  [list of (namespace, key, value)])  — multiple settings

_ANIM_KEYS = [
    ("global", "window_animation_scale"),
    ("global", "transition_animation_scale"),
    ("global", "animator_duration_scale"),
]

# Stored as (label, action_spec) where action_spec is:
#   (namespace, key, value)  or  [(namespace, key, value), ...]
OPTIONS: dict[str, tuple] = {
    "animations_off":   (
        "Animations aus (0x)",
        [(ns, k, "0") for ns, k in _ANIM_KEYS],
    ),
    "animations_on":    (
        "Animations an (1x)",
        [(ns, k, "1") for ns, k in _ANIM_KEYS],
    ),
    "stay_awake":       (
        "Display bleibt beim Laden an",
        ("global", "stay_on_while_plugged_in", "3"),
    ),
    "stay_awake_off":   (
        "Display-Timeout normal",
        ("global", "stay_on_while_plugged_in", "0"),
    ),
    "show_touches":     (
        "Touches anzeigen",
        ("system", "show_touches", "1"),
    ),
    "hide_touches":     (
        "Touches ausblenden",
        ("system", "show_touches", "0"),
    ),
    "pointer_location": (
        "Pointer Location",
        ("system", "pointer_location", "1"),
    ),
    "usb_debugging":    (
        "USB-Debugging",
        ("global", "adb_enabled", "1"),
    ),
    "bg_process_limit": (
        "Hintergrundprozesse: max 4",
        ("global", "background_process_limit", "4"),
    ),
}

# Menu order (keys from OPTIONS dict)
_MENU_ORDER: list[str] = [
    "animations_off",
    "animations_on",
    "stay_awake",
    "stay_awake_off",
    "show_touches",
    "hide_touches",
    "pointer_location",
    "usb_debugging",
    "bg_process_limit",
]


# ---------------------------------------------------------------------------
# Current-value helpers
# ---------------------------------------------------------------------------

def _current_value_for_option(serial: str, key: str) -> str:
    """Return a compact 'current value' string for the given option key."""
    label, spec = OPTIONS[key]

    if isinstance(spec, list):
        # Multiple settings — show the first value (all three anim scales are equal)
        ns, k, _ = spec[0]
        val = _get_setting(serial, ns, k)
        return val or "(not set)"

    # Single setting
    ns, k, _ = spec
    val = _get_setting(serial, ns, k)
    return val or "(not set)"


# ---------------------------------------------------------------------------
# Apply a single option key
# ---------------------------------------------------------------------------

def _apply_option(serial: str, key: str) -> None:
    """Apply all settings for the given option key."""
    label, spec = OPTIONS[key]

    if isinstance(spec, list):
        for ns, k, v in spec:
            _put_setting(serial, ns, k, v)
    else:
        ns, k, v = spec
        _put_setting(serial, ns, k, v)


# ---------------------------------------------------------------------------
# Interactive menu
# ---------------------------------------------------------------------------

def _show_menu(device: DeviceInfo) -> None:
    """Print the developer options menu with current values and handle selection."""
    serial = device.serial

    print(f"\n  Developer Options — {device.model}\n")
    print(f"  {'#':<3}  {'Key':<20}  {'Description':<38}  Current")
    print(f"  {'-'*3}  {'-'*20}  {'-'*38}  {'-'*12}")

    for idx, opt_key in enumerate(_MENU_ORDER):
        label, _ = OPTIONS[opt_key]
        current  = _current_value_for_option(serial, opt_key)
        print(f"  {idx:<3}  {opt_key:<20}  {label:<38}  {current}")

    print()
    print("  Enter number to apply, or press Enter to cancel.")
    try:
        choice = input("  Select: ").strip()
    except (EOFError, KeyboardInterrupt):
        print()
        return

    if not choice:
        return

    try:
        idx = int(choice)
        if idx < 0 or idx >= len(_MENU_ORDER):
            raise ValueError
    except ValueError:
        print(f"  [WARN] Invalid selection: {choice!r}. Aborted.")
        return

    opt_key = _MENU_ORDER[idx]
    label, _ = OPTIONS[opt_key]
    _apply_option(serial, opt_key)
    print(f"  [OK] Applied: {label}  ({opt_key})")


# ---------------------------------------------------------------------------
# Public actions
# ---------------------------------------------------------------------------

def action_dev_options(
    device: DeviceInfo,
    key:   str | None,
    value: str | None,
) -> None:
    """
    Manage Android developer options.

    key=None, value=None  : show interactive menu with current values
    key + value           : write settings key directly
                            key format: "namespace/setting_key" or a known
                            shorthand from OPTIONS dict
    """
    serial = device.serial

    # ── Direct set ────────────────────────────────────────────────────────
    if key is not None and value is not None:
        # Check if key is a named shorthand
        if key in OPTIONS:
            _apply_option(serial, key)
            label = OPTIONS[key][0]
            print(f"  [OK] Applied: {label}  ({key})")
            return

        # Accept "namespace/setting_key" or "namespace setting_key"
        if "/" in key:
            namespace, _, setting_key = key.partition("/")
        else:
            parts = key.split(None, 1)
            if len(parts) == 2:
                namespace, setting_key = parts
            else:
                raise AdbError(
                    f"Cannot parse key '{key}'. "
                    "Use 'namespace/setting_key' or a named shorthand."
                )

        _put_setting(serial, namespace.strip(), setting_key.strip(), value)
        print(f"  [OK] settings put {namespace} {setting_key} = {value}  [{device.model}]")
        return

    # ── Read-only key query ───────────────────────────────────────────────
    if key is not None and value is None:
        if key in OPTIONS:
            current = _current_value_for_option(serial, key)
            label   = OPTIONS[key][0]
            print(f"\n  {label} ({key}): {current}\n")
            return

        if "/" in key:
            namespace, _, setting_key = key.partition("/")
        else:
            parts = key.split(None, 1)
            if len(parts) == 2:
                namespace, setting_key = parts
            else:
                raise AdbError(f"Cannot parse key '{key}'.")

        current = _get_setting(serial, namespace.strip(), setting_key.strip())
        print(f"\n  settings get {namespace} {setting_key} = {current or '(not set)'}\n")
        return

    # ── Interactive menu ──────────────────────────────────────────────────
    _show_menu(device)


def action_screen_always_on(device: DeviceInfo, enable: bool | None) -> None:
    """
    Control 'stay on while plugged in' (display never sleeps while charging).

    enable=None  : show current status
    enable=True  : stay on (AC + USB + Wireless  → value 3)
    enable=False : normal screen timeout (→ value 0)
    """
    serial = device.serial

    stay_val   = _get_setting(serial, "global", "stay_on_while_plugged_in")
    timeout_ms = _get_setting(serial, "system", "screen_off_timeout")

    # ── Read-only ────────────────────────────────────────────────────────
    if enable is None:
        print(f"\n  Screen Always On — {device.model}\n")

        # Decode stay_on_while_plugged_in bitmask
        try:
            v = int(stay_val)
        except (ValueError, TypeError):
            v = -1

        _PLUGGED = {1: "AC", 2: "USB", 4: "Wireless", 8: "Dock"}
        if v == 0:
            stay_label = "off (normal timeout)"
        elif v > 0:
            sources = [label for bit, label in _PLUGGED.items() if v & bit]
            stay_label = f"on ({', '.join(sources)})"
        else:
            stay_label = f"unknown ({stay_val!r})"

        print(f"  {'Stay-on while plugged':<24}: {stay_label}")

        # Decode screen_off_timeout (milliseconds)
        try:
            ms = int(timeout_ms)
            if ms < 0:
                timeout_label = "never"
            elif ms < 1000:
                timeout_label = f"{ms} ms"
            elif ms < 60_000:
                timeout_label = f"{ms // 1000} s"
            else:
                timeout_label = f"{ms // 60_000} min"
        except (ValueError, TypeError):
            timeout_label = timeout_ms or "(not set)"

        print(f"  {'Screen-off timeout':<24}: {timeout_label}")
        print()
        return

    # ── Enable ────────────────────────────────────────────────────────────
    if enable:
        _put_setting(serial, "global", "stay_on_while_plugged_in", "3")
        print(f"  [OK] Screen stays on while plugged in (AC + USB + Wireless)  [{device.model}]")

    # ── Disable ───────────────────────────────────────────────────────────
    else:
        _put_setting(serial, "global", "stay_on_while_plugged_in", "0")
        print(f"  [OK] Screen always-on disabled (normal timeout restored)  [{device.model}]")
