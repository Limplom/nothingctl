"""Screenshot and screen recording for Nothing phones."""

import subprocess
from datetime import datetime
from pathlib import Path

from .device import adb_pull, run
from .exceptions import AdbError
from .models import DeviceInfo

_MAX_RECORD_DURATION = 180  # seconds; hard limit imposed by screenrecord


def _timestamp() -> str:
    return datetime.now().strftime("%Y%m%d_%H%M%S")


def action_screenshot(device: DeviceInfo, base_dir: Path) -> None:
    """Capture a screenshot and pull it to base_dir/screenshots/."""
    ts = _timestamp()
    remote_path = f"/sdcard/Download/screenshot_{ts}.png"
    local_dir = base_dir / "screenshots"
    local_dir.mkdir(parents=True, exist_ok=True)
    local_path = local_dir / f"screenshot_{ts}.png"

    print(f"Taking screenshot...")
    r = run(["adb", "-s", device.serial, "shell", "screencap", "-p", remote_path])
    if r.returncode != 0:
        raise AdbError(f"screencap failed: {r.stderr.strip()}")

    try:
        adb_pull(remote_path, local_path, device.serial)
    finally:
        run(["adb", "-s", device.serial, "shell", "rm", "-f", remote_path])

    if not local_path.exists():
        raise AdbError(f"Screenshot file not found after pull: {local_path}")

    size_kb = local_path.stat().st_size / 1024
    print(f"[OK] Screenshot saved: {local_path}")
    print(f"     Size: {size_kb:.1f} KB")


def action_screenrecord(device: DeviceInfo, base_dir: Path,
                        duration: int = 30) -> None:
    """Record the screen and pull the video to base_dir/recordings/."""
    if duration > _MAX_RECORD_DURATION:
        print(f"[WARN] Requested duration {duration}s exceeds maximum "
              f"{_MAX_RECORD_DURATION}s — clamping.")
        duration = _MAX_RECORD_DURATION

    ts = _timestamp()
    remote_path = f"/sdcard/Download/screenrecord_{ts}.mp4"
    local_dir = base_dir / "recordings"
    local_dir.mkdir(parents=True, exist_ok=True)
    local_path = local_dir / f"screenrecord_{ts}.mp4"

    # Some devices (e.g. Nothing Phone 1) require an explicit --size or the
    # MediaCodec encoder fails with err=-38. Scale to 720p width to stay within
    # encoder limits while preserving the native aspect ratio.
    size_arg: list[str] = []
    r_size = run(["adb", "-s", device.serial, "shell", "wm size"])
    if r_size.returncode == 0:
        import re as _re
        m = _re.search(r"Physical size:\s*(\d+)x(\d+)", r_size.stdout)
        if m:
            w, h = int(m.group(1)), int(m.group(2))
            if w > 720:
                h = round(h * 720 / w)
                w = 720
            size_arg = ["--size", f"{w}x{h}"]

    print(f"Recording for {duration} seconds (Ctrl-C to stop early)...")

    timeout_ms = (duration + 10) * 1000
    try:
        r = subprocess.run(
            ["adb", "-s", device.serial, "shell",
             "screenrecord", "--time-limit", str(duration), *size_arg, remote_path],
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=timeout_ms / 1000,
        )
    except subprocess.TimeoutExpired:
        print("[WARN] screenrecord timed out — attempting to pull partial recording.")
        r = None
    except KeyboardInterrupt:
        print("\n[WARN] Interrupted — attempting to pull partial recording.")
        r = None

    if r is not None:
        combined = (r.stdout + r.stderr).lower()
        if r.returncode != 0 and ("not found" in combined or "screenrecord" in combined
                                  and "permission denied" not in combined
                                  and r.returncode == 127):
            raise AdbError(
                "screenrecord is not available on this device or Android version. "
                "Requires Android 4.4+ and is absent on some low-RAM or Go devices."
            )
        if r.returncode != 0 and "not found" in combined:
            raise AdbError(
                "screenrecord is not available on this device or Android version."
            )

    try:
        adb_pull(remote_path, local_path, device.serial)
    except AdbError as exc:
        raise AdbError(f"Failed to pull recording: {exc}") from exc
    finally:
        run(["adb", "-s", device.serial, "shell", "rm", "-f", remote_path])

    if not local_path.exists():
        raise AdbError(f"Recording file not found after pull: {local_path}")

    size_mb = local_path.stat().st_size / (1024 * 1024)
    print(f"[OK] Recording saved: {local_path}")
    print(f"     Size: {size_mb:.2f} MB")
