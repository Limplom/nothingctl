"""System maintenance: cache clearing and locale management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Low-level helpers
# ---------------------------------------------------------------------------

def _shell(serial: str, cmd: str) -> str:
    """Run an adb shell command; return stdout stripped, empty string on failure."""
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _setting(serial: str, namespace: str, key: str) -> str:
    """Read an Android setting via 'adb shell settings get'."""
    r = run(["adb", "-s", serial, "shell", "settings", "get", namespace, key])
    val = r.stdout.strip() if r.returncode == 0 else ""
    return "" if val in ("null", "null\n", "null\r") else val


# ---------------------------------------------------------------------------
# Storage helpers
# ---------------------------------------------------------------------------

def _parse_df_available(df_output: str) -> int | None:
    """
    Parse 'df /data' output and return available space in 1K-blocks.

    df on Android typically outputs:
        Filesystem     1K-blocks    Used Available Use% Mounted on
        /dev/...       <total>  <used>  <avail>  <pct>  /data
    The second data line has the numbers; Available is column index 3.
    Returns None if parsing fails.
    """
    lines = [l for l in df_output.splitlines() if l.strip()]
    for line in lines:
        # Skip the header line
        if line.lstrip().startswith("Filesystem"):
            continue
        parts = line.split()
        # Some kernels wrap long filesystem names onto the next line,
        # giving only 4 numeric columns (Used Available Use% Mounted).
        if len(parts) == 4:
            try:
                return int(parts[1])   # Available is index 1 in wrapped form
            except ValueError:
                pass
        if len(parts) >= 4:
            try:
                return int(parts[3])   # Available is index 3 in standard form
            except ValueError:
                pass
    return None


def _fmt_kb(kb: int) -> str:
    """Format a kilobyte count into a human-readable string."""
    if kb >= 1024 * 1024:
        return f"{kb / 1024 / 1024:.2f} GB"
    if kb >= 1024:
        return f"{kb / 1024:.1f} MB"
    return f"{kb} KB"


def _get_free_data(serial: str) -> int | None:
    """Return free space on /data in KB, or None if unavailable."""
    r = run(["adb", "-s", serial, "shell", "df /data"])
    if r.returncode != 0 or not r.stdout.strip():
        return None
    return _parse_df_available(r.stdout)


# ---------------------------------------------------------------------------
# 1. Cache Clear
# ---------------------------------------------------------------------------

def action_cache_clear(device: DeviceInfo, package: str | None) -> None:
    """
    Clear app caches on the connected Nothing phone.

    package=None  : system-wide trim via 'pm trim-caches 10G'
    package=<PKG> : clear only that app's cache via 'cmd package clear-cache'
    """
    s = device.serial
    print(f"\n  Cache Clear \u2014 {device.model}\n")

    # ── Snapshot free space before ──────────────────────────────────────────
    free_before = _get_free_data(s)

    # ── System-wide trim ───────────────────────────────────────────────────
    r_trim = run(["adb", "-s", s, "shell", "pm", "trim-caches", "10G"])
    if r_trim.returncode == 0:
        print("  [OK] System cache trimmed (target: 10 GB freed).")
    else:
        err = r_trim.stderr.strip() or r_trim.stdout.strip()
        print(f"  [WARN] System trim failed: {err or 'unknown error'}")

    # ── Single-package cache clear ─────────────────────────────────────────
    if package is not None:
        r_pkg = run(["adb", "-s", s, "shell", "cmd", "package", "clear-cache", package])
        if r_pkg.returncode == 0:
            print(f"  [OK] Cache cleared: {package}")
        else:
            err = r_pkg.stderr.strip() or r_pkg.stdout.strip()
            raise AdbError(
                f"Failed to clear cache for '{package}': {err or 'unknown error'}"
            )

    # ── Snapshot free space after ──────────────────────────────────────────
    free_after = _get_free_data(s)

    if free_before is not None or free_after is not None:
        print()
        if free_before is not None:
            print(f"  {'Free (/data) before':<22}: {_fmt_kb(free_before)}")
        if free_after is not None:
            print(f"  {'Free (/data) after':<22}: {_fmt_kb(free_after)}")
        if free_before is not None and free_after is not None:
            delta = free_after - free_before
            sign  = "+" if delta >= 0 else ""
            print(f"  {'Change':<22}: {sign}{_fmt_kb(delta)}")

    print()


# ---------------------------------------------------------------------------
# 2. Locale / Timezone / Time Format
# ---------------------------------------------------------------------------

# Mapping from UTC offset minutes to a representative label, used to annotate
# timezone display.  We derive this at runtime from 'date +%z' instead.

def _get_utc_offset(serial: str) -> str:
    """Return a UTC offset string like 'UTC+1' or 'UTC-5:30', or '' on failure."""
    raw = _shell(serial, "date +%z")          # e.g. "+0100" or "-0530"
    m   = re.match(r"([+-])(\d{2})(\d{2})$", raw.strip())
    if not m:
        return ""
    sign, hh, mm = m.group(1), int(m.group(2)), int(m.group(3))
    label = f"UTC{sign}{hh}" if mm == 0 else f"UTC{sign}{hh}:{mm:02d}"
    return label


def action_locale(
    device: DeviceInfo,
    lang: str | None,
    timezone: str | None,
    hour24: bool | None,
) -> None:
    """
    Read or set locale, timezone, and time format on the connected Nothing phone.

    All parameters None  : display current settings.
    lang=<locale>        : set system language (e.g. 'de-DE', 'en-US').
    timezone=<tz>        : set timezone (e.g. 'Europe/Berlin').
    hour24=True/False    : enable or disable 24-hour time format.
    """
    s = device.serial

    # ── Apply changes first (so the read-back reflects the new state) ───────

    if lang is not None:
        # Persist via the Settings database; apps will pick this up on next
        # Activity resume without requiring a reboot.
        r = run(["adb", "-s", s, "shell", "settings", "put", "system",
                 "system_locales", lang])
        if r.returncode != 0:
            err = r.stderr.strip() or r.stdout.strip()
            raise AdbError(f"Failed to set language to '{lang}': {err or 'unknown error'}")
        # Notify the system that the locale changed so running apps react.
        run(["adb", "-s", s, "shell",
             "am broadcast -a android.intent.action.LOCALE_CHANGED"])
        print(f"  [OK] Language set to: {lang}")

    if timezone is not None:
        # 'cmd alarm set-timezone' is the most reliable non-root method across
        # Android 10+ (works on all Nothing phones shipped with Android 11+).
        r = run(["adb", "-s", s, "shell", "cmd", "alarm", "set-timezone", timezone])
        if r.returncode != 0:
            err = r.stderr.strip() or r.stdout.strip()
            raise AdbError(
                f"Failed to set timezone to '{timezone}': {err or 'unknown error'}"
            )
        print(f"  [OK] Timezone set to: {timezone}")

    if hour24 is not None:
        value = "24" if hour24 else "12"
        r = run(["adb", "-s", s, "shell", "settings", "put", "system",
                 "time_12_24", value])
        if r.returncode != 0:
            err = r.stderr.strip() or r.stdout.strip()
            raise AdbError(
                f"Failed to set time format to '{value}h': {err or 'unknown error'}"
            )
        print(f"  [OK] Time format set to: {value}h")

    # If any change was applied, print a blank line before the summary.
    if any(x is not None for x in (lang, timezone, hour24)):
        print()

    # ── Read current state ──────────────────────────────────────────────────
    # Language: try persist.sys.locale first (set by the OS after a change),
    # then the Settings DB entry we may have just written.
    locale_val = _shell(s, "getprop persist.sys.locale")
    if not locale_val:
        locale_val = _setting(s, "system", "system_locales")
    if not locale_val:
        # Android sometimes stores this as a comma-separated BCP-47 list.
        locale_val = _shell(s, "getprop ro.product.locale")
    lang_display = locale_val or "(not available)"

    # Timezone
    tz_val    = _shell(s, "getprop persist.sys.timezone")
    utc_label = _get_utc_offset(s)
    if tz_val and utc_label:
        tz_display = f"{tz_val} ({utc_label})"
    else:
        tz_display = tz_val or "(not available)"

    # 24-hour format
    fmt_val = _setting(s, "system", "time_12_24")
    if fmt_val == "24":
        fmt_display = "24h"
    elif fmt_val == "12":
        fmt_display = "12h"
    elif not fmt_val:
        fmt_display = "(system default)"
    else:
        fmt_display = fmt_val

    # ── Output ─────────────────────────────────────────────────────────────
    print(f"  Locale \u2014 {device.model}\n")
    print(f"  {'Language':<16}: {lang_display}")
    print(f"  {'Timezone':<16}: {tz_display}")
    print(f"  {'Time Format':<16}: {fmt_display}")
    print()
