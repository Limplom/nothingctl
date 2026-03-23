"""Nothing Glyph Interface diagnostics and control via ADB."""

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# Phone (1) and (2) use the old dedicated service package.
# Phone (2a), (3a) and newer use the hearthstone app instead.
_GLYPH_PKG_LEGACY = "ly.nothing.glyph.service"
_GLYPH_PKG_NEW    = "com.nothing.hearthstone"

# Secure setting (Phone 1/2) — single on/off toggle
_SETTING_LEGACY = ("secure", "glyph_interface_enable")

# Global settings (Phone 2a/3a) — individual feature toggles
_SETTINGS_NEW = [
    ("global", "glyph_long_torch_enable",   "Long torch"),
    ("global", "glyph_pocket_mode_state",   "Pocket mode"),
    ("global", "glyph_screen_upward_state", "Screen-upward mode"),
]

# Nothing Phone model → Glyph zone names for display
# Keys are matched against device.model (may be marketing name OR codename).
# Phone (3a) Lite shares the Galaxian codename and A001 model prefix with the
# regular (3a) but is a Lite/budget variant — same Glyph strip layout.
_GLYPH_ZONES = {
    "Phone (1)":        ["Camera", "Diagonal", "Battery dot", "Battery bar", "USB"],
    "spacewar":         ["Camera", "Diagonal", "Battery dot", "Battery bar", "USB"],
    "Phone (2)":        ["Camera top", "Camera bottom", "Diagonal",
                         "Battery left", "Battery right", "USB", "Notification"],
    "pong":             ["Camera top", "Camera bottom", "Diagonal",
                         "Battery left", "Battery right", "USB", "Notification"],
    "Phone (2a)":       ["Camera", "Battery", "Bottom strip"],
    "pacman":           ["Camera", "Battery", "Bottom strip"],
    "Phone (3a) Lite":  ["Camera", "Bottom strip"],
    "Phone (3a)":       ["Camera top", "Camera bottom", "Battery", "Bottom strip"],
    "galaxian":         ["Camera top", "Camera bottom", "Battery", "Bottom strip"],
    "A001":             ["Camera top", "Camera bottom", "Battery", "Bottom strip"],
    "CMF Phone 1":      ["Ring", "Dot"],
}


def _detect_pkg(serial: str) -> str | None:
    """Return whichever Glyph package is installed, or None."""
    for pkg in (_GLYPH_PKG_NEW, _GLYPH_PKG_LEGACY):
        r = run(["adb", "-s", serial, "shell", f"pm list packages {pkg}"])
        if pkg in r.stdout:
            return pkg
    return None


def _is_legacy(pkg: str) -> bool:
    return pkg == _GLYPH_PKG_LEGACY


def _glyph_service_running(serial: str, pkg: str) -> bool:
    r = run(["adb", "-s", serial, "shell",
             f"dumpsys activity services {pkg} 2>/dev/null | grep -c ServiceRecord"])
    return r.stdout.strip() not in ("", "0")


def action_glyph(device: DeviceInfo, enable: str | None) -> None:
    """
    Diagnostics for Nothing Glyph interface.

    enable: "on" / "off" to toggle (legacy devices only), None for status only.
    """
    pkg = _detect_pkg(device.serial)
    if not pkg:
        raise AdbError(
            f"No Glyph package found ({_GLYPH_PKG_NEW} or {_GLYPH_PKG_LEGACY}).\n"
            "This device may not support the Glyph interface, or "
            "the package was removed via debloat."
        )

    # ── Toggle (legacy — Phone 1/2 only) ────────────────────────────────────
    if enable is not None:
        if _is_legacy(pkg):
            ns, key = _SETTING_LEGACY
            val = "1" if enable.lower() in ("on", "1", "true") else "0"
            run(["adb", "-s", device.serial, "shell", f"settings put {ns} {key} {val}"])
        else:
            # Phone (2a)/(3a)/(3a Lite): stop or restart the GlyphService
            if enable.lower() in ("on", "1", "true"):
                r = run(["adb", "-s", device.serial, "shell",
                         "su -c 'am startservice com.nothing.thirdparty/.GlyphService'"])
            else:
                r = run(["adb", "-s", device.serial, "shell",
                         "su -c 'am stopservice com.nothing.thirdparty/.GlyphService'"])
            if r.returncode != 0:
                print(f"[WARN] GlyphService toggle may have failed (needs root).")
                print(f"       Fallback: use the Glyphs tile in Quick Settings.")
                return
        label = "enabled" if enable.lower() in ("on", "1", "true") else "disabled"
        print(f"[OK] Glyph interface {label}.")
        print("     Changes take effect immediately (no reboot needed).")
        return

    # ── Status display ───────────────────────────────────────────────────────
    running = _glyph_service_running(device.serial, pkg)
    print(f"\n  Glyph package  : {pkg}")
    print(f"  Service state  : {'running' if running else 'not running'}")

    if _is_legacy(pkg):
        ns, key = _SETTING_LEGACY
        r = run(["adb", "-s", device.serial, "shell", f"settings get {ns} {key}"])
        val = r.stdout.strip()
        state = {"1": "ENABLED", "0": "DISABLED"}.get(val, f"unknown ({val})")
        print(f"  Interface      : {state}")
    else:
        # Phone (2a)/(3a) — show individual feature states
        print(f"\n  Glyph feature settings:")
        for ns, key, label in _SETTINGS_NEW:
            r = run(["adb", "-s", device.serial, "shell", f"settings get {ns} {key}"])
            val = r.stdout.strip()
            state = {"1": "on", "0": "off"}.get(val, f"unknown ({val})")
            print(f"    {label:<26} {state}")
        print(f"\n  [INFO] Main on/off toggle: Glyphs Quick Settings tile")

    # ── Zone map for this model ──────────────────────────────────────────────
    model_key = next(
        (k for k in _GLYPH_ZONES if k.lower() in device.model.lower()), None
    )
    if model_key:
        zones = _GLYPH_ZONES[model_key]
        print(f"\n  Glyph zones on {model_key} ({len(zones)}):")
        for z in zones:
            print(f"    • {z}")
    else:
        print(f"\n  Zone map: not available for model '{device.model}'")

    # ── Controls hint ────────────────────────────────────────────────────────
    print(f"\n  Toggle:")
    print(f"    python nothingctl.py --glyph --glyph-enable on")
    print(f"    python nothingctl.py --glyph --glyph-enable off")
    if not _is_legacy(pkg):
        print(f"    (or use the Glyphs tile in Quick Settings)")
