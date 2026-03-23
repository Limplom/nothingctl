"""Display settings and color profile management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _setting(serial: str, namespace: str, key: str) -> str:
    """Read an Android setting via 'adb shell settings get'."""
    r = run(["adb", "-s", serial, "shell", "settings", "get", namespace, key])
    val = r.stdout.strip() if r.returncode == 0 else ""
    return "" if val in ("null", "null\n", "null\r") else val


def _put_setting(serial: str, namespace: str, key: str, value: str) -> None:
    """Write an Android setting; raise AdbError on failure."""
    r = run(["adb", "-s", serial, "shell", "settings", "put", namespace, key, value])
    if r.returncode != 0:
        raise AdbError(f"settings put {namespace} {key} failed: {r.stderr.strip()}")


def _shell(serial: str, cmd: str) -> str:
    """Run adb shell command; return stdout stripped, empty string on failure."""
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


# ---------------------------------------------------------------------------
# Internal formatters
# ---------------------------------------------------------------------------

def _fmt_timeout(ms_str: str) -> str:
    """Convert a screen timeout value in milliseconds to a human-readable string."""
    try:
        ms = int(ms_str)
    except (ValueError, TypeError):
        return ms_str or "n/a"

    seconds = ms // 1000
    if seconds < 60:
        return f"{seconds}s"
    minutes = seconds // 60
    if minutes == 1:
        return "1 min"
    return f"{minutes} min"


def _fmt_rotation(val: str) -> str:
    """Convert user_rotation integer to a human-readable label."""
    mapping = {"0": "portrait", "1": "landscape", "2": "reverse-portrait", "3": "reverse-landscape"}
    return mapping.get(val, val or "n/a")


def _fmt_on_off(val: str) -> str:
    """Return 'on' or 'off' from a '1'/'0' setting value."""
    if val == "1":
        return "on"
    if val == "0":
        return "off"
    return val or "n/a"


def _parse_wm_size(raw: str) -> str:
    """Extract resolution string from 'wm size' output (e.g. 'Physical size: 1080x2400')."""
    m = re.search(r'Physical size:\s*(\d+x\d+)', raw)
    if m:
        return m.group(1) + " (physical)"
    m = re.search(r'(\d+x\d+)', raw)
    if m:
        return m.group(1)
    return raw or "n/a"


def _parse_wm_density(raw: str) -> str:
    """Extract DPI value from 'wm density' output."""
    # Output is either "Physical density: 420" or "Override density: 420"
    m = re.search(r'(?:Override|Physical) density:\s*(\d+)', raw)
    if m:
        return m.group(1)
    m = re.search(r'(\d+)', raw)
    if m:
        return m.group(1)
    return raw or "n/a"


def _parse_refresh_rate(raw: str) -> str:
    """Extract the first refresh rate value from dumpsys display output."""
    for line in raw.splitlines():
        # Match patterns like: mRefreshRate=120.0, refreshRate=120.0, "refreshRate": 120.0
        m = re.search(r'(?:mRefreshRate|refreshRate)[=:\s"]+([0-9]+(?:\.[0-9]+)?)', line)
        if m:
            return m.group(1) + " Hz"
    return "n/a"


def _color_profile_label(val: str) -> str:
    """Return a human-readable label for a Nothing OS display_color_mode value."""
    mapping = {"0": "Natural (sRGB)", "1": "Vivid (P3)", "256": "Custom"}
    return mapping.get(val, f"Unknown ({val})" if val else "n/a")


# ---------------------------------------------------------------------------
# Key → (namespace, settings_key) map for action_display writes
# ---------------------------------------------------------------------------

_DISPLAY_SETTINGS: dict[str, tuple[str, str]] = {
    "brightness":      ("system", "screen_brightness"),
    "brightness_auto": ("system", "screen_brightness_mode"),
    "timeout":         ("system", "screen_off_timeout"),
    "rotation":        ("system", "user_rotation"),
    "rotation_auto":   ("system", "accelerometer_rotation"),
    "font_scale":      ("system", "font_scale"),
}


# ---------------------------------------------------------------------------
# 1. Display settings
# ---------------------------------------------------------------------------

def action_display(device: DeviceInfo, key: str | None, value: str | None) -> None:
    """Read or set display settings on the connected Nothing phone.

    With no key/value: prints a summary of all display settings.
    With key + value:  writes the specified setting.
    """
    s = device.serial

    # ── Write mode ────────────────────────────────────────────────────────
    if key is not None and value is not None:
        key = key.lower()

        if key == "dpi":
            r = run(["adb", "-s", s, "shell", "wm", "density", value])
            if r.returncode != 0:
                raise AdbError(f"wm density {value} failed: {r.stderr.strip()}")
            print(f"  DPI set to {value} on {device.model}.")
            return

        if key not in _DISPLAY_SETTINGS:
            raise AdbError(
                f"Unknown display key '{key}'. "
                f"Valid keys: {', '.join(sorted(_DISPLAY_SETTINGS) + ['dpi'])}"
            )

        namespace, setting_key = _DISPLAY_SETTINGS[key]
        _put_setting(s, namespace, setting_key, value)
        print(f"  {key} set to {value} on {device.model}.")
        return

    # ── Read mode ─────────────────────────────────────────────────────────
    # Physical size
    size_raw     = _shell(s, "wm size")
    resolution   = _parse_wm_size(size_raw)

    # DPI — prefer override density if present, fall back to physical
    density_raw  = _shell(s, "wm density")
    dpi          = _parse_wm_density(density_raw)

    # Refresh rate — first match from dumpsys display
    display_dump = _shell(s, "dumpsys display | grep -E 'mRefreshRate|refreshRate'")
    refresh_rate = _parse_refresh_rate(display_dump)

    # Settings-based values
    brightness      = _setting(s, "system", "screen_brightness")
    brightness_mode = _setting(s, "system", "screen_brightness_mode")
    timeout_raw     = _setting(s, "system", "screen_off_timeout")
    font_scale      = _setting(s, "system", "font_scale")
    rotation_val    = _setting(s, "system", "user_rotation")
    rotation_auto   = _setting(s, "system", "accelerometer_rotation")

    # Formatted values
    brightness_display = brightness or "n/a"
    if brightness:
        brightness_display = f"{brightness} / 255 (auto: {_fmt_on_off(brightness_mode)})"

    timeout_display  = _fmt_timeout(timeout_raw) if timeout_raw else "n/a"
    font_display     = font_scale or "n/a"
    rotation_display = _fmt_rotation(rotation_val)
    if rotation_auto:
        rotation_display += f" (auto: {_fmt_on_off(rotation_auto)})"

    # ── Output ────────────────────────────────────────────────────────────
    print(f"\n  Display — {device.model}\n")
    print(f"  {'Resolution':<16}: {resolution}")
    print(f"  {'DPI':<16}: {dpi}")
    print(f"  {'Refresh Rate':<16}: {refresh_rate}")
    print()
    print(f"  {'Brightness':<16}: {brightness_display}")
    print(f"  {'Font Scale':<16}: {font_display}")
    print(f"  {'Rotation':<16}: {rotation_display}")
    print(f"  {'Screen Timeout':<16}: {timeout_display}")
    print()


# ---------------------------------------------------------------------------
# 2. Color profile
# ---------------------------------------------------------------------------

# Accepted profile name aliases → canonical integer string
_PROFILE_ALIASES: dict[str, str] = {
    "natural": "0",
    "vivid":   "1",
    "custom":  "256",
}


def action_color_profile(device: DeviceInfo, profile: str | None) -> None:
    """Read or set the display color profile on the connected Nothing phone.

    With no profile: prints current color mode and night light settings.
    With profile:    sets display_color_mode to the requested value.
    """
    s = device.serial

    # ── Write mode ────────────────────────────────────────────────────────
    if profile is not None:
        canonical = _PROFILE_ALIASES.get(profile.lower(), profile)
        _put_setting(s, "system", "display_color_mode", canonical)
        label = _color_profile_label(canonical)
        print(f"  Color profile set to {label} on {device.model}.")
        return

    # ── Read mode ─────────────────────────────────────────────────────────
    color_mode       = _setting(s, "system", "display_color_mode")
    night_active     = _setting(s, "secure", "night_display_activated")
    night_temp       = _setting(s, "secure", "night_display_color_temperature")

    mode_display     = _color_profile_label(color_mode) if color_mode else "n/a"
    if color_mode:
        mode_display = f"{_color_profile_label(color_mode)} ({color_mode})"

    night_display    = _fmt_on_off(night_active)

    # ── Output ────────────────────────────────────────────────────────────
    print(f"\n  Color Profile — {device.model}\n")
    print(f"  {'Color Mode':<16}: {mode_display}")
    print(f"  {'Night Light':<16}: {night_display}")
    if night_active == "1" and night_temp:
        print(f"  {'Color Temp':<16}: {night_temp} K")
    print()
