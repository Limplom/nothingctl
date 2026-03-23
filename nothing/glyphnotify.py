"""Glyph notification configuration viewer."""

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

_GLYPH_PKG_LEGACY = "ly.nothing.glyph.service"
_GLYPH_PKG_NEW    = "com.nothing.hearthstone"
_GLYPH_NOTIFY_PKG = "com.nothing.glyphnotification"

# Known Glyph-related global settings keys with human labels
_GLYPH_GLOBAL_KEYS: list[tuple[str, str]] = [
    ("glyph_long_torch_enable",        "Long torch"),
    ("glyph_pocket_mode_state",        "Pocket mode"),
    ("glyph_screen_upward_state",      "Screen-upward mode"),
    ("nt_glyph_interface_debug_enable","Glyph debug mode"),
]

# Hearthstone services that use Glyph
_HEARTHSTONE_GLYPH_SERVICES = (
    "GlyphService",
    "GlyphComposer",
    "GlyphManagerService",
)


def _pkg_installed(serial: str, pkg: str) -> bool:
    r = run(["adb", "-s", serial, "shell", f"pm list packages {pkg}"])
    return pkg in r.stdout


def _get_setting(serial: str, namespace: str, key: str) -> str:
    r = run(["adb", "-s", serial, "shell", f"settings get {namespace} {key}"])
    return r.stdout.strip()


def _get_glyph_settings(serial: str) -> list[tuple[str, str, str]]:
    """Return list of (label, key, value) for all known glyph global settings."""
    results = []
    for key, label in _GLYPH_GLOBAL_KEYS:
        val = _get_setting(serial, "global", key)
        if val != "null":
            results.append((label, key, val))
    return results


def _get_hearthstone_services(serial: str) -> list[str]:
    """Return running ServiceRecord names for com.nothing.hearthstone."""
    r = run(["adb", "-s", serial, "shell",
             "dumpsys activity services com.nothing.hearthstone 2>/dev/null"])
    services = []
    for line in r.stdout.splitlines():
        line = line.strip()
        if line.startswith("* ServiceRecord"):
            # Extract the service class name
            # Format: * ServiceRecord{... u0 com.nothing.hearthstone/.foo.Bar ...}
            parts = line.split()
            for part in parts:
                if "com.nothing.hearthstone/" in part:
                    cls = part.rstrip("}")
                    short = cls.split("/")[-1].split(".")[-1]
                    services.append(short)
                    break
    return services


def _get_glyph_notify_info(serial: str) -> dict:
    """
    Query the com.nothing.glyphnotification package for notification channel info.
    Returns a dict with keys: installed, channel_importance, channel_name.
    """
    info: dict = {"installed": False}
    if not _pkg_installed(serial, _GLYPH_NOTIFY_PKG):
        return info
    info["installed"] = True

    r = run(["adb", "-s", serial, "shell",
             "dumpsys notification 2>/dev/null | grep -A 12 'com.nothing.glyphnotification'"])
    info["raw"] = r.stdout.strip()

    # Parse channel importance and name from raw dumpsys output
    for line in r.stdout.splitlines():
        line = line.strip()
        if "mId='Glyph'" in line and "mImportance=" in line:
            # Extract importance value
            try:
                imp_str = line.split("mImportance=")[1].split(",")[0]
                imp_map = {"1": "MIN", "2": "LOW", "3": "DEFAULT", "4": "HIGH", "5": "MAX"}
                info["channel_importance"] = imp_map.get(imp_str, imp_str)
            except (IndexError, ValueError):
                pass
    return info


def action_glyph_notify(device: DeviceInfo) -> None:
    """
    Show Glyph notification configuration for a Nothing device.

    Displays:
    - Glyph-related settings keys and their current values
    - com.nothing.glyphnotification package status and channel config
    - Running Hearthstone services that interact with Glyph
    - Note about deep notification data requiring root/NDK access
    """
    serial = device.serial

    print(f"\n  Glyph Notifications — {device.model} ({serial})")

    # ── 1. Glyph settings keys ────────────────────────────────────────────────
    glyph_settings = _get_glyph_settings(serial)
    if glyph_settings:
        print(f"\n  Glyph settings:")
        for label, key, val in glyph_settings:
            state = {"1": "on", "0": "off"}.get(val, val)
            print(f"    {label:<34} {state}  (global/{key})")
    else:
        print(f"\n  Glyph settings: none found on this device")

    # ── 2. GlyphNotification package ─────────────────────────────────────────
    print(f"\n  Glyph Notification package ({_GLYPH_NOTIFY_PKG}):")
    notify_info = _get_glyph_notify_info(serial)
    if notify_info["installed"]:
        importance = notify_info.get("channel_importance", "unknown")
        print(f"    Installed              : yes")
        print(f"    Notification channel   : Glyph  (importance={importance})")
    else:
        print(f"    Installed              : no  (package not present on this device)")

    # ── 3. Hearthstone services ──────────────────────────────────────────────
    print(f"\n  Active Hearthstone services ({_GLYPH_PKG_NEW}):")
    services = _get_hearthstone_services(serial)
    if services:
        for svc in services:
            print(f"    • {svc}")
    else:
        print(f"    (none running)")

    # ── 4. Apps using Glyph notification listener ────────────────────────────
    print(f"\n  Notification listeners (enabled_notification_listeners):")
    listeners_val = _get_setting(serial, "secure", "enabled_notification_listeners")
    if listeners_val and listeners_val != "null":
        for entry in listeners_val.split(":"):
            pkg = entry.split("/")[0].strip()
            cls = entry.split("/")[1].strip() if "/" in entry else ""
            print(f"    • {pkg}")
    else:
        print(f"    (none)")

    # ── 5. Legacy Glyph package check ─────────────────────────────────────────
    print(f"\n  Glyph packages:")
    for pkg, label in (
        (_GLYPH_PKG_NEW,    "Hearthstone (new, Phone 2a/3a/3a Lite)"),
        (_GLYPH_PKG_LEGACY, "Glyph service (legacy, Phone 1/2)"),
        (_GLYPH_NOTIFY_PKG, "Glyph notification controller"),
    ):
        installed = _pkg_installed(serial, pkg)
        print(f"    {'[+]' if installed else '[-]'} {label:<46} ({pkg})")

    # ── 6. Deeper data notice ─────────────────────────────────────────────────
    print()
    print(
        "  [INFO] Per-app Glyph lighting rules are stored inside Hearthstone's\n"
        "         private database (/data/data/com.nothing.hearthstone/databases/).\n"
        "         Reading that data requires root access or a backup extraction.\n"
        "         The ContentProvider content://com.nothing.hearthstone.provider/\n"
        "         is not publicly accessible without root."
    )
