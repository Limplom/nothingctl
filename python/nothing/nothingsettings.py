"""Nothing-specific Android settings reader/writer and Essential Space control."""

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Known Nothing settings keys — (namespace, key, human_label, device_hint)
# device_hint: "all", "phone1", "phone2plus", "new" (2a/3a/3a Lite)
# ---------------------------------------------------------------------------
_KNOWN_KEYS: list[tuple[str, str, str, str]] = [
    # Glyph — Phone (1) only
    ("global",  "glyph_long_torch_enable",      "Glyph long torch",              "all"),
    ("global",  "nt_glyph_interface_debug_enable", "Glyph debug mode",           "phone1"),
    # Glyph — Phone (2a) / (3a) / (3a Lite)
    ("global",  "glyph_pocket_mode_state",       "Glyph pocket mode",             "new"),
    ("global",  "glyph_screen_upward_state",     "Glyph screen-upward mode",      "new"),
    # Wireless charging — Phone (1)
    ("global",  "nt_wireless_forward_charge",    "Wireless charging",             "phone1"),
    ("global",  "nt_wireless_reverse_charge",    "Wireless reverse charging",     "phone1"),
    ("global",  "nt_reverse_charging_limiting_level", "Reverse charge limit (%)", "phone1"),
    # Essential Space — Phone (3a) / newer
    ("global",  "essential_notification_rules",  "Essential Space notif. rules",  "new"),
    ("secure",  "essential_has_set_default_rule","Essential Space default rule set","new"),
    ("secure",  "nt_essential_key_onboarding",   "Essential key onboarding done", "new"),
    # UI / misc — all devices
    ("global",  "nt_circle_to_search_support",   "Circle-to-Search support",      "all"),
    ("global",  "nt_is_upgrade",                 "OTA upgrade flag",              "all"),
    ("global",  "ambient_enabled",               "Always-on display",             "all"),
    ("global",  "ambient_tilt_to_wake",          "Tilt-to-wake (AOD)",            "all"),
    ("global",  "ambient_touch_to_wake",         "Touch-to-wake (AOD)",           "all"),
    ("global",  "led_effect_google_assistant_enalbe", "LED Google Assistant effect", "all"),
    ("system",  "nothing_icon_pack",             "Icon pack",                     "all"),
    # Phone (3a) Lite specific
    ("system",  "nothing_camera_foreground",     "Camera foreground mode",        "new"),
    # Game mode
    ("secure",  "nt_game_mode_gaming",           "Game mode active",              "all"),
    ("secure",  "nt_game_mode_notification_display_mode", "Game mode notif. display", "all"),
    ("secure",  "nt_game_slider_enable",         "Game slider enabled",           "phone1"),
    # Misc secure
    ("secure",  "nt_mistouch_prevention_enable", "Mistouch prevention",           "phone1"),
    ("secure",  "nt_face_recognition_unlock_with_mask", "Face unlock with mask",  "phone1"),
    ("secure",  "nt_flip_to_record_state",       "Flip-to-record",                "new"),
    ("secure",  "nt_glimpse_lockscreen_cleared",  "Glimpse lockscreen seen",      "new"),
]

# Keys that require Phone (2) or newer for Essential Space
_ESSENTIAL_SPACE_MODELS = ("Phone (2)", "Phone (2a)", "Phone (3)", "Phone (3a)")


def _is_newer_device(device: DeviceInfo) -> bool:
    """Return True for Phone (2a), (3a), (3a Lite) — the 'hearthstone' generation."""
    model = device.model.lower()
    codename = device.codename.lower()
    return any(x in model for x in ("2a", "3a", "3", "pacman", "galaxian")) or \
           codename in ("pacman", "galaxian")


def _get_setting(serial: str, namespace: str, key: str) -> str:
    r = run(["adb", "-s", serial, "shell", f"settings get {namespace} {key}"])
    return r.stdout.strip()


def _set_setting(serial: str, namespace: str, key: str, value: str) -> None:
    r = run(["adb", "-s", serial, "shell", f"settings put {namespace} {key} {value}"])
    if r.returncode != 0:
        raise AdbError(f"Failed to set {namespace}/{key}={value}: {r.stderr.strip()}")


def action_nothing_settings(
    device: DeviceInfo,
    key: str | None,
    value: str | None,
) -> None:
    """
    Read or write Nothing-specific Android settings.

    key=None, value=None  — list all known Nothing settings with current values.
    key given, value=None — read that key (auto-detect namespace or use ns:key syntax).
    key + value           — write that key to value.
    """
    # ── Write ────────────────────────────────────────────────────────────────
    if key is not None and value is not None:
        # Support "namespace:key" or plain key
        if ":" in key:
            ns, raw_key = key.split(":", 1)
        else:
            # Try to find namespace from known map
            match = next((ns for ns, k, _, _ in _KNOWN_KEYS if k == key), None)
            if match is None:
                raise AdbError(
                    f"Unknown key '{key}'. Use 'namespace:key' syntax "
                    f"(e.g. global:{key}) to write an arbitrary key."
                )
            ns = match
            raw_key = key
        _set_setting(device.serial, ns, raw_key, value)
        print(f"[OK] Set {ns}/{raw_key} = {value}")
        return

    # ── Read single key ──────────────────────────────────────────────────────
    if key is not None:
        if ":" in key:
            ns, raw_key = key.split(":", 1)
        else:
            match = next(((ns, k) for ns, k, _, _ in _KNOWN_KEYS if k == key), None)
            if match is None:
                raise AdbError(
                    f"Unknown key '{key}'. Use 'namespace:key' syntax "
                    f"(e.g. global:{key}) to read an arbitrary key."
                )
            ns, raw_key = match
        val = _get_setting(device.serial, ns, raw_key)
        print(f"  {ns}/{raw_key} = {val}")
        return

    # ── List all known keys ──────────────────────────────────────────────────
    print(f"\n  Nothing Settings — {device.model} ({device.serial})")
    print(f"  {'Namespace':<8}  {'Key':<44}  {'Label':<38}  Value")
    print(f"  {'-'*8}  {'-'*44}  {'-'*38}  -----")

    for ns, k, label, hint in _KNOWN_KEYS:
        val = _get_setting(device.serial, ns, k)
        # Show even if "null" — gives complete picture
        marker = " *" if val == "null" else "  "
        print(f"{marker} {ns:<8}  {k:<44}  {label:<38}  {val}")

    print()
    print("  (* = key not present on this device)")
    print("  Use --nothing-settings-key ns:key --nothing-settings-value val to write.")


# ---------------------------------------------------------------------------
# Essential Space
# ---------------------------------------------------------------------------

_ESSENTIAL_PKG       = "com.nothing.ntessentialspace"
_ESSENTIAL_INTEL_PKG = "com.nothing.essentialintelligence"

# Models that support Essential Space (Phone (2) and newer)
_ESSENTIAL_SUPPORTED_MODELS = (
    "Phone (2)",
    "Phone (2a)",
    "Phone (3)",
    "Phone (3a)",
)


def _has_essential_space(serial: str) -> bool:
    """Return True if the Essential Space package is installed."""
    r = run(["adb", "-s", serial, "shell", f"pm list packages {_ESSENTIAL_PKG}"])
    return _ESSENTIAL_PKG in r.stdout


def action_essential_space(device: DeviceInfo, enable: bool | None) -> None:
    """
    Show or toggle Essential Space (Nothing Phone (2) and newer).

    enable=None  — show current status.
    enable=True  — enable Essential Space.
    enable=False — disable Essential Space.
    """
    if not _has_essential_space(device.serial):
        print(
            f"  [INFO] Essential Space is only available on Nothing Phone (2) and newer.\n"
            f"         Package '{_ESSENTIAL_PKG}' not found on {device.model}."
        )
        return

    # ── Toggle ───────────────────────────────────────────────────────────────
    if enable is not None:
        val = "1" if enable else "0"
        label = "enabled" if enable else "disabled"
        _set_setting(device.serial, "secure", "essential_space_enabled", val)
        print(f"[OK] Essential Space {label}.")
        print("     Changes take effect immediately.")
        return

    # ── Status ───────────────────────────────────────────────────────────────
    print(f"\n  Essential Space — {device.model} ({device.serial})")

    # essential_space_enabled may not exist yet (key is set on first toggle)
    enabled_val  = _get_setting(device.serial, "secure", "essential_space_enabled")
    rules_val    = _get_setting(device.serial, "global", "essential_notification_rules")
    default_rule = _get_setting(device.serial, "secure", "essential_has_set_default_rule")
    onboarding   = _get_setting(device.serial, "secure", "nt_essential_key_onboarding")

    state_map = {"1": "ENABLED", "0": "DISABLED", "null": "not set (default)"}
    print(f"  Enabled               : {state_map.get(enabled_val, enabled_val)}")
    print(f"  Default rule set      : {'yes' if default_rule == '1' else 'no'}")
    print(f"  Onboarding completed  : {'yes' if onboarding  == '1' else 'no'}")
    print(f"  Notification rules    : {rules_val}")

    # Check intelligence package
    intel_r = run(["adb", "-s", device.serial, "shell",
                   f"pm list packages {_ESSENTIAL_INTEL_PKG}"])
    if _ESSENTIAL_INTEL_PKG in intel_r.stdout:
        print(f"  Essential Intelligence: installed")
    else:
        print(f"  Essential Intelligence: not installed")

    print(f"\n  Toggle:")
    print(f"    python nothingctl.py --essential-space --essential-enable")
    print(f"    python nothingctl.py --essential-space --no-essential-enable")
