"""Notification listing and clipboard management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

_MAX_NOTIFICATIONS = 50


# ── helpers ───────────────────────────────────────────────────────────────────

def _shell(serial: str, cmd: str) -> str:
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _get_sdk(serial: str) -> int:
    """Return the Android SDK level as an integer, or 0 if unavailable."""
    raw = _shell(serial, "getprop ro.build.version.sdk")
    try:
        return int(raw)
    except ValueError:
        return 0


# ── notification parsing ──────────────────────────────────────────────────────

def _parse_notifications(output: str) -> list[dict[str, str]]:
    """
    Parse 'dumpsys notification --noredact' output and return a list of dicts
    with keys: pkg, title, text.

    The dumpsys format (Android 13+) groups fields inside NotificationRecord
    blocks.  A simplified excerpt looks like:

        NotificationRecord(0x1a2b3c4d: pkg=com.example.app ...)
          ...
          pkg=com.example.app
          ...
          android.title=Some Title
          android.text=Notification text
    """
    notifications: list[dict[str, str]] = []

    # Split on NotificationRecord boundaries so each chunk belongs to one entry.
    blocks = re.split(r"(?=\bNotificationRecord\b)", output)

    for block in blocks:
        if "NotificationRecord" not in block:
            continue

        pkg = ""
        title = ""
        text = ""

        for line in block.splitlines():
            stripped = line.strip()

            # pkg= can appear on the NotificationRecord header line or on its own.
            if not pkg:
                m = re.search(r"\bpkg=([^\s,)]+)", stripped)
                if m:
                    pkg = m.group(1)

            # android.title= / android.text= are the standard extras keys.
            # Values have format: String (actual text) or SpannableString (actual text)
            if not title:
                m = re.match(r"android\.title=(?:\w+\s+)?\(?(.*?)\)?$", stripped)
                if m:
                    raw = m.group(1).strip()
                    # Strip wrapper: "String (text)" → "text"
                    inner = re.match(r"(?:String|SpannableString)\s+\((.+)\)$", stripped[len("android.title="):].strip())
                    title = inner.group(1).strip() if inner else raw

            if not text:
                m = re.match(r"android\.text=(?:\w+\s+)?\(?(.*?)\)?$", stripped)
                if m:
                    raw = m.group(1).strip()
                    inner = re.match(r"(?:String|SpannableString)\s+\((.+)\)$", stripped[len("android.text="):].strip())
                    text = inner.group(1).strip() if inner else raw

        # Only include records that have at least a package name.
        if pkg:
            # Treat "null" (from Android dumpsys) the same as empty
            if title in ("null", "null\r"):
                title = ""
            if text in ("null", "null\r"):
                text = ""
            notifications.append({
                "pkg": pkg,
                "title": title if title else "(no title)",
                "text": text if text else "(no text)",
            })

    return notifications


# ── public actions ────────────────────────────────────────────────────────────

def action_notifications(device: DeviceInfo, package: str | None) -> None:
    """List active notifications on the device."""
    r = run(["adb", "-s", device.serial, "shell", "dumpsys notification --noredact"])
    if r.returncode != 0:
        raise AdbError(f"dumpsys notification failed: {r.stderr.strip()}")

    all_notifications = _parse_notifications(r.stdout)

    # Apply optional package filter.
    if package:
        shown = [n for n in all_notifications if n["pkg"] == package]
    else:
        shown = all_notifications

    # Enforce the hard cap.
    capped = shown[:_MAX_NOTIFICATIONS]

    print(f"\n  Active Notifications \u2014 {device.model}\n")

    if not capped:
        if package:
            print(f"  No notifications found for package: {package}\n")
        else:
            print("  No active notifications.\n")
        return

    # Column widths.
    pkg_w   = max(len("Package"),               max(len(n["pkg"])   for n in capped))
    title_w = max(len("Title"),                 max(len(n["title"]) for n in capped))
    # Clamp widths so the table stays readable on normal terminals.
    pkg_w   = min(pkg_w,   40)
    title_w = min(title_w, 30)

    header = (
        f"  {'#':<4} "
        f"{'Package':<{pkg_w}}  "
        f"{'Title':<{title_w}}  "
        f"Text"
    )
    print(header)
    print("  " + "-" * (len(header) - 2))

    for idx, notif in enumerate(capped, start=1):
        pkg_col   = notif["pkg"][:pkg_w]
        title_col = notif["title"][:title_w]
        text_col  = notif["text"]
        print(
            f"  {idx:<4} "
            f"{pkg_col:<{pkg_w}}  "
            f"{title_col:<{title_w}}  "
            f"{text_col}"
        )

    print()

    # Footer summary.
    total = len(all_notifications)
    shown_count = len(capped)

    if package:
        footer = f"  Total: {total} notification(s) ({shown_count} shown, filtered by package: {package})"
    elif shown_count < total:
        footer = f"  Total: {total} notification(s) (showing first {shown_count})"
    else:
        footer = f"  Total: {total} notification(s)"

    print(footer)
    print()


def action_clipboard(device: DeviceInfo, text: str | None) -> None:
    """Read or set the device clipboard."""
    serial = device.serial
    sdk = _get_sdk(serial)

    print(f"\n  Clipboard \u2014 {device.model}\n")

    if text is not None:
        # ── SET clipboard ─────────────────────────────────────────────────────
        # `cmd clipboard set-primary` is available from Android 13 (SDK 33).
        # On older versions it may still be present as an undocumented command;
        # we try it regardless and report the result.
        r = run(["adb", "-s", serial, "shell", "cmd", "clipboard", "set-primary", text])

        if r.returncode == 0 and "error" not in r.stdout.lower() and "unknown" not in r.stdout.lower():
            print(f"  Clipboard set to: {text}\n")
        else:
            # Fallback: try the short form without 'set-primary'.
            r2 = run(["adb", "-s", serial, "shell", "cmd", "clipboard", "set", text])
            if r2.returncode == 0 and "error" not in r2.stdout.lower() and "unknown" not in r2.stdout.lower():
                print(f"  Clipboard set to: {text}\n")
            else:
                print(f"  [WARN] Could not set clipboard via adb shell cmd clipboard.")
                if sdk and sdk < 29:
                    print(f"         Device is running SDK {sdk} — cmd clipboard may not be available.")
                else:
                    print(f"         The shell user may lack permission on this build (SDK {sdk}).")
                print()

    else:
        # ── READ clipboard ────────────────────────────────────────────────────
        content: str | None = None

        # Attempt 1: cmd clipboard get  (works on some AOSP/Nothing builds)
        r1 = run(["adb", "-s", serial, "shell", "cmd", "clipboard", "get"])
        if (
            r1.returncode == 0
            and r1.stdout.strip()
            and "error" not in r1.stdout.lower()
            and "unknown" not in r1.stdout.lower()
        ):
            content = r1.stdout.strip()

        # Attempt 2: low-level binder call  (service call clipboard 2 i32 0)
        if content is None:
            r2 = run(["adb", "-s", serial, "shell", "service", "call", "clipboard", "2", "i32", "0"])
            if r2.returncode == 0 and r2.stdout.strip():
                # Output looks like: Result: Parcel(00000000 00000002 00310033 ...)
                # The payload after the first two 32-bit words is UTF-16LE text.
                m = re.search(r"Result:\s*Parcel\(([0-9a-fA-F\s]+)\)", r2.stdout)
                if m:
                    hex_tokens = m.group(1).split()
                    # Drop the first two status/length words.
                    data_tokens = hex_tokens[2:]
                    raw_bytes = bytearray()
                    for token in data_tokens:
                        # Each token is a little-endian 32-bit word (8 hex chars).
                        try:
                            word = int(token, 16)
                            raw_bytes += word.to_bytes(4, byteorder="little")
                        except ValueError:
                            pass
                    # Decode as UTF-16LE and strip null terminators.
                    try:
                        decoded = raw_bytes.decode("utf-16-le").rstrip("\x00").strip()
                        if decoded:
                            content = decoded
                    except (UnicodeDecodeError, ValueError):
                        pass

        if content is not None:
            print(f"  Content: {content}\n")
        else:
            # Android 10+ restricts background clipboard access.
            sdk_label = f"SDK {sdk}" if sdk else "SDK unknown"
            print(f"  [INFO] Clipboard read requires foreground app access on Android 10+ ({sdk_label}).")
            print(f"         Background reads via adb shell are blocked by the OS.")
            print(f"         Use --text \"content\" to set clipboard instead.\n")
