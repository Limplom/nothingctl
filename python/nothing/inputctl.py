"""Input event control for Nothing phones (tap, swipe, text, keyevent)."""

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Keycode reference table
# ---------------------------------------------------------------------------

_KEYCODES: list[tuple[str, int]] = [
    ("KEYCODE_HOME",              3),
    ("KEYCODE_BACK",              4),
    ("KEYCODE_POWER",            26),
    ("KEYCODE_VOLUME_UP",        24),
    ("KEYCODE_VOLUME_DOWN",      25),
    ("KEYCODE_WAKEUP",          224),
    ("KEYCODE_SLEEP",           223),
    ("KEYCODE_MENU",             82),
    ("KEYCODE_CAMERA",           27),
    ("KEYCODE_MEDIA_PLAY_PAUSE", 85),
    ("KEYCODE_BRIGHTNESS_UP",   221),
    ("KEYCODE_BRIGHTNESS_DOWN", 220),
]


def _print_keycode_reference() -> None:
    """Print a reference table of common Android keycodes."""
    print("\n  Android Keycode Reference\n")
    print(f"  {'Keycode name':<30}  {'Code':>4}")
    print(f"  {'-' * 30}  {'-' * 4}")
    for name, code in _KEYCODES:
        print(f"  {name:<30}  {code:>4}")
    print()
    print("  Usage:  --keyevent KEYCODE_HOME  or  --keyevent 3")
    print()


# ---------------------------------------------------------------------------
# Text escaping
# ---------------------------------------------------------------------------

def _escape_input_text(text: str) -> str:
    """
    Escape characters that break 'adb shell input text'.

    The Android input command interprets spaces as argument separators and
    treats several shell metacharacters literally.  We wrap the entire string
    in single quotes (ADB passes the argument to /system/bin/sh) and escape
    single quotes that appear inside the text itself.
    """
    # Replace every single quote with: end-quote, literal ', re-open-quote
    escaped = text.replace("'", "'\\''")
    return f"'{escaped}'"


# ---------------------------------------------------------------------------
# Main action
# ---------------------------------------------------------------------------

def action_input(
    device: DeviceInfo,
    tap:      str | None,
    swipe:    str | None,
    text:     str | None,
    keyevent: str | None,
) -> None:
    """
    Send input events to a Nothing phone via ADB.

    tap      : "x,y"
    swipe    : "x1,y1,x2,y2" or "x1,y1,x2,y2,duration_ms"
    text     : arbitrary string (special characters are escaped)
    keyevent : keycode name (e.g. KEYCODE_HOME) or numeric code (e.g. 3)

    If no argument is given, print a keycode reference table.
    """
    s = device.serial

    if tap is None and swipe is None and text is None and keyevent is None:
        _print_keycode_reference()
        return

    # ── Tap ────────────────────────────────────────────────────────────────
    if tap is not None:
        parts = [p.strip() for p in tap.split(",")]
        if len(parts) != 2:
            raise AdbError(
                f"Invalid tap format '{tap}'. Expected 'x,y' (e.g. '540,1200')."
            )
        x, y = parts
        r = run(["adb", "-s", s, "shell", "input", "tap", x, y])
        if r.returncode != 0:
            raise AdbError(f"input tap failed: {r.stderr.strip()}")
        print(f"  Tap sent: ({x}, {y})  [{device.model}]")

    # ── Swipe ──────────────────────────────────────────────────────────────
    if swipe is not None:
        parts = [p.strip() for p in swipe.split(",")]
        if len(parts) == 4:
            x1, y1, x2, y2 = parts
            cmd = ["adb", "-s", s, "shell", "input", "swipe", x1, y1, x2, y2]
            desc = f"({x1},{y1}) -> ({x2},{y2})"
        elif len(parts) == 5:
            x1, y1, x2, y2, dur = parts
            cmd = ["adb", "-s", s, "shell", "input", "swipe", x1, y1, x2, y2, dur]
            desc = f"({x1},{y1}) -> ({x2},{y2}), duration {dur} ms"
        else:
            raise AdbError(
                f"Invalid swipe format '{swipe}'. "
                "Expected 'x1,y1,x2,y2' or 'x1,y1,x2,y2,duration_ms'."
            )
        r = run(cmd)
        if r.returncode != 0:
            raise AdbError(f"input swipe failed: {r.stderr.strip()}")
        print(f"  Swipe sent: {desc}  [{device.model}]")

    # ── Text ───────────────────────────────────────────────────────────────
    if text is not None:
        escaped = _escape_input_text(text)
        # Use shell=False; pass the quoted string as a single shell argument
        # by invoking sh -c so Android's /system/bin/sh handles the quoting.
        r = run(["adb", "-s", s, "shell", f"input text {escaped}"])
        if r.returncode != 0:
            raise AdbError(f"input text failed: {r.stderr.strip()}")
        preview = text if len(text) <= 40 else text[:40] + "…"
        print(f"  Text sent: {preview!r}  [{device.model}]")

    # ── Keyevent ───────────────────────────────────────────────────────────
    if keyevent is not None:
        r = run(["adb", "-s", s, "shell", "input", "keyevent", str(keyevent)])
        if r.returncode != 0:
            raise AdbError(f"input keyevent failed: {r.stderr.strip()}")
        print(f"  Keyevent sent: {keyevent}  [{device.model}]")
