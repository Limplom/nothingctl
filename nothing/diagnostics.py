"""Device diagnostics: logcat dump, bugreport, ANR/tombstone collection."""

import datetime
from pathlib import Path

from .device import adb_pull, run
from .exceptions import AdbError
from .models import DeviceInfo

_TEMP = "/data/local/tmp"


# ---------------------------------------------------------------------------
# Logcat
# ---------------------------------------------------------------------------

def action_logcat(
    device: DeviceInfo,
    base_dir: Path,
    package: str | None = None,
    tag: str | None = None,
    level: str | None = None,
    lines: int = 500,
) -> None:
    """
    Dump the current logcat buffer to a local file with optional filters.

    package: filter by app package name (resolved to PID)
    tag:     filter by log tag
    level:   minimum log level — V / D / I / W / E / F (default: all)
    lines:   max lines to capture (default 500)
    """
    # Build filter spec: "tag:level" or "*:V" for all
    if tag and level:
        filter_spec = f"{tag}:{level.upper()} *:S"
    elif tag:
        filter_spec = f"{tag}:V *:S"
    elif level:
        filter_spec = f"*:{level.upper()}"
    else:
        filter_spec = "*:V"

    # Resolve package → PID for --package filter
    pid_filter = ""
    if package:
        r = run(["adb", "-s", device.serial, "shell", f"pidof {package}"])
        pid = r.stdout.strip().split()[0] if r.stdout.strip() else None
        if pid:
            pid_filter = f"--pid={pid}"
            print(f"  Package {package} → PID {pid}")
        else:
            print(f"  [WARN] {package} not running — capturing full buffer")

    ts        = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    dest_dir  = base_dir / "logs"
    dest_dir.mkdir(parents=True, exist_ok=True)

    label = package or tag or "full"
    dest  = dest_dir / f"logcat_{label}_{ts}.txt"

    # -d = dump buffer and exit, -v threadtime = full timestamp + thread info
    cmd = ["adb", "-s", device.serial, "logcat",
           "-d", "-v", "threadtime", "-t", str(lines)]
    if pid_filter:
        cmd.append(pid_filter)
    cmd.append(filter_spec)

    print(f"  Capturing logcat (max {lines} lines, filter: {filter_spec})...")
    r = run(cmd)

    if not r.stdout.strip():
        print("  [WARN] Empty logcat — buffer may have been cleared.")
        return

    dest.write_text(r.stdout, encoding="utf-8", errors="replace")
    line_count = r.stdout.count("\n")
    print(f"[OK] {line_count} lines → {dest}")


# ---------------------------------------------------------------------------
# Bugreport
# ---------------------------------------------------------------------------

def action_bugreport(device: DeviceInfo, base_dir: Path) -> None:
    """
    Trigger adb bugreport and save the ZIP to base_dir/bugreports/.
    Bugreport includes logcat, ANR traces, tombstones, dumpsys, and more.
    This typically takes 30-90 seconds.
    """
    ts       = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    dest_dir = base_dir / "bugreports"
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest     = dest_dir / f"bugreport_{device.serial}_{ts}.zip"

    print(f"  Generating bugreport (this takes 30–90 seconds)...")
    print(f"  Saving to: {dest}")

    # adb bugreport <path> saves directly to the local path
    r = run(["adb", "-s", device.serial, "bugreport", str(dest)])

    if not dest.exists():
        raise AdbError(
            f"Bugreport not created.\n"
            f"Output: {(r.stdout + r.stderr).strip()}"
        )

    size_mb = dest.stat().st_size / 1024 / 1024
    print(f"[OK] Bugreport saved — {size_mb:.1f} MB → {dest}")
    print(f"     Open the ZIP with any archive manager or 'unzip -l \"{dest}\"' on macOS/Linux")


# ---------------------------------------------------------------------------
# ANR / tombstone dump
# ---------------------------------------------------------------------------

def action_anr_dump(device: DeviceInfo, base_dir: Path) -> None:
    """
    Pull ANR traces from /data/anr/ and tombstones from /data/tombstones/.
    Both require root. Saves to base_dir/diagnostics/<timestamp>/.
    """
    ts      = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    dest    = base_dir / "diagnostics" / ts
    dest.mkdir(parents=True, exist_ok=True)

    sources = [
        ("/data/anr",        "anr"),
        ("/data/tombstones", "tombstones"),
    ]

    any_found = False

    for remote_dir, label in sources:
        # Check if directory has any files
        r = run(["adb", "-s", device.serial, "shell",
                 f"su -c 'ls {remote_dir}/ 2>/dev/null | wc -l'"])
        count_str = r.stdout.strip()
        try:
            count = int(count_str)
        except ValueError:
            count = 0

        if count == 0:
            print(f"  {label:<12}: empty (no crashes recorded)")
            continue

        print(f"  {label:<12}: {count} file(s) — copying...")

        # Copy to a readable temp location, then pull
        tmp = f"{_TEMP}/{label}_dump_{ts}"
        r2 = run(["adb", "-s", device.serial, "shell",
                  f"su -c 'cp -r {remote_dir} {tmp} && chmod -R 644 {tmp}/* 2>/dev/null && echo __OK__'"])

        if "__OK__" not in r2.stdout:
            print(f"  [WARN] Could not copy {remote_dir} (root needed?)")
            continue

        local_dest = dest / label
        local_dest.mkdir(exist_ok=True)
        try:
            adb_pull(tmp, local_dest, device.serial)
            any_found = True
            print(f"         saved → {local_dest}/")
        except Exception as e:
            print(f"  [WARN] Pull failed: {e}")
        finally:
            run(["adb", "-s", device.serial, "shell",
                 f"su -c 'rm -rf {tmp}'"])

    if any_found:
        print(f"\n[OK] Diagnostics saved → {dest}")
    else:
        print(f"\n[OK] No ANR traces or tombstones found — device is clean.")
        dest.rmdir()
