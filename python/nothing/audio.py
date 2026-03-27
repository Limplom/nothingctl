"""Audio volume and routing management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


# ---------------------------------------------------------------------------
# Stream definitions
# ---------------------------------------------------------------------------

# (stream_id, display_name)
_STREAMS: list[tuple[int, str]] = [
    (0, "Voice Call"),
    (1, "System"),
    (2, "Ring"),
    (3, "Media"),
    (4, "Alarm"),
    (5, "Notification"),
]

# Name aliases → stream ID
_STREAM_ALIASES: dict[str, int] = {
    "voice":        0,
    "call":         0,
    "system":       1,
    "ring":         2,
    "media":        3,
    "music":        3,
    "alarm":        4,
    "notification": 5,
    "notify":       5,
}

# Device type string → human-readable label
_DEVICE_LABELS: dict[str, str] = {
    "AUDIO_DEVICE_OUT_SPEAKER":              "Speaker",
    "AUDIO_DEVICE_OUT_WIRED_HEADPHONE":      "Wired Headphones",
    "AUDIO_DEVICE_OUT_WIRED_HEADSET":        "Wired Headset",
    "AUDIO_DEVICE_OUT_BLUETOOTH_A2DP":       "Bluetooth A2DP",
    "AUDIO_DEVICE_OUT_BLUETOOTH_A2DP_HEADPHONES": "Bluetooth Headphones",
    "AUDIO_DEVICE_OUT_BLUETOOTH_A2DP_SPEAKER":    "Bluetooth Speaker",
    "AUDIO_DEVICE_OUT_BLUETOOTH_SCO":        "Bluetooth SCO",
    "AUDIO_DEVICE_OUT_BLUETOOTH_SCO_HEADSET":"Bluetooth SCO Headset",
    "AUDIO_DEVICE_OUT_USB_HEADSET":          "USB Headset",
    "AUDIO_DEVICE_OUT_USB_DEVICE":           "USB Audio",
    "AUDIO_DEVICE_OUT_EARPIECE":             "Earpiece",
    "AUDIO_DEVICE_OUT_HDMI":                 "HDMI",
}

BAR_WIDTH = 20


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _shell(serial: str, cmd: str) -> str:
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _resolve_stream(stream: str) -> int:
    """Resolve a stream name or numeric string to a stream ID."""
    # Try numeric first
    try:
        sid = int(stream)
        valid_ids = {s[0] for s in _STREAMS}
        if sid not in valid_ids:
            raise AdbError(
                f"Unknown stream ID {sid}. Valid IDs: "
                + ", ".join(str(s[0]) for s in _STREAMS)
            )
        return sid
    except ValueError:
        pass

    # Try alias
    key = stream.lower()
    if key in _STREAM_ALIASES:
        return _STREAM_ALIASES[key]

    raise AdbError(
        f"Unknown stream '{stream}'. Valid names: "
        + ", ".join(sorted(_STREAM_ALIASES.keys()))
    )


def _get_stream_volume(serial: str, stream_id: int) -> tuple[int, int]:
    """
    Query volume for a single stream.
    Returns (current_volume, max_volume).
    Raises AdbError on parse failure.
    """
    output = _shell(serial, f"cmd media_session volume --stream {stream_id} --get")
    # Expected: "volume is 5 in range [0..15]"
    m = re.search(r"volume is\s+(\d+)\s+in range\s+\[(\d+)\.\.(\d+)\]", output)
    if not m:
        raise AdbError(
            f"Could not parse volume output for stream {stream_id}: {output!r}"
        )
    current = int(m.group(1))
    max_vol = int(m.group(3))
    return current, max_vol


def _bar(current: int, maximum: int, width: int = BAR_WIDTH) -> str:
    """Return an ASCII block bar of given width."""
    if maximum <= 0:
        return " " * width
    filled = int(round(current / maximum * width))
    filled = max(0, min(width, filled))
    return "\u2588" * filled + " " * (width - filled)


# ---------------------------------------------------------------------------
# Public actions
# ---------------------------------------------------------------------------

def action_audio(device: DeviceInfo, stream: str | None, volume: int | None) -> None:
    """Read or set audio stream volumes."""

    # ── set mode ──────────────────────────────────────────────────────────
    if stream is not None and volume is not None:
        stream_id = _resolve_stream(stream)
        stream_name = next(name for sid, name in _STREAMS if sid == stream_id)
        run(["adb", "-s", device.serial, "shell",
             "cmd", "media_session", "volume", "--stream", str(stream_id), "--set", str(volume)])
        # Confirm by reading back
        try:
            current, maximum = _get_stream_volume(device.serial, stream_id)
            print(
                f"\n  {stream_name} volume set to {current}/{maximum} "
                f"on {device.model}\n"
            )
        except AdbError:
            print(f"\n  {stream_name} volume set to {volume} on {device.model}\n")
        return

    if (stream is None) != (volume is None):
        raise AdbError("Provide both --stream and --volume to set volume, or neither to read all.")

    # ── read / display all streams ────────────────────────────────────────
    print(f"\n  Audio Volumes \u2014 {device.model}\n")
    print(f"  {'Stream':<16} {'Vol':>5}  {'Max':>5}   {'Bar'}")
    print("  " + "\u2500" * (16 + 5 + 5 + BAR_WIDTH + 12))

    for stream_id, stream_name in _STREAMS:
        try:
            current, maximum = _get_stream_volume(device.serial, stream_id)
        except AdbError:
            current, maximum = 0, 0

        bar_str = _bar(current, maximum)
        print(
            f"  {stream_name:<16} {current:>5}  {maximum:>5}   [{bar_str}]"
        )

    print()


def action_audio_route(device: DeviceInfo) -> None:
    """Show active audio output path and connected Bluetooth audio devices."""

    # ── active output device from dumpsys audio ───────────────────────────
    audio_dump = _shell(device.serial, "dumpsys audio")

    active_output: str = "Unknown"

    # Strategy 1: look for "Devices: <name>(<hex>)" lines in stream volume sections
    # (format used by Android 12+: "   Devices: speaker(2)")
    _SYSFS_DEVICE_LABELS: dict[str, str] = {
        "speaker":          "Speaker",
        "earpiece":         "Earpiece",
        "bt_a2dp":          "Bluetooth A2DP",
        "bt_sco":           "Bluetooth SCO",
        "usb_headset":      "USB Headset",
        "wired_headphone":  "Wired Headphones",
        "wired_headset":    "Wired Headset",
        "hdmi":             "HDMI",
        "ble_headset":      "Bluetooth LE Headset",
    }
    # We want the "active" output — find the Devices line in the Music stream section.
    # Music is stream index 3 — look for it in the "Stream volumes" block.
    in_stream_section = False
    stream_idx = -1
    for line in audio_dump.splitlines():
        if "Stream volumes" in line:
            in_stream_section = True
            stream_idx = 0
            continue
        if in_stream_section:
            if line.strip().startswith("Current:"):
                stream_idx += 1
            elif line.strip().startswith("Devices:"):
                # stream_idx 1=VoiceCall, 2=System, 3=Ring, 4=Music
                if stream_idx == 4:  # Music stream
                    m = re.search(r"Devices:\s*(\w+)\(", line)
                    if m:
                        dev = m.group(1).lower()
                        active_output = _SYSFS_DEVICE_LABELS.get(dev, dev)
                    break
            elif line.strip() == "" and stream_idx > 6:
                break  # end of section

    # Strategy 2: look for explicit "Output device:" lines (older Android format)
    if active_output == "Unknown":
        for line in audio_dump.splitlines():
            m = re.search(r"Output device:\s*(AUDIO_DEVICE_OUT_\w+)", line)
            if m:
                raw = m.group(1)
                active_output = _DEVICE_LABELS.get(raw, raw)
                break

    # Strategy 3: any AUDIO_DEVICE_OUT_ mention
    if active_output == "Unknown":
        for line in audio_dump.splitlines():
            m = re.search(r"(AUDIO_DEVICE_OUT_\w+)", line)
            if m:
                raw = m.group(1)
                active_output = _DEVICE_LABELS.get(raw, raw)
                if raw != "AUDIO_DEVICE_OUT_SPEAKER":
                    break

    # ── Bluetooth devices from dumpsys bluetooth_manager ─────────────────
    bt_dump = _shell(device.serial, "dumpsys bluetooth_manager")

    bt_devices: list[tuple[str, str, str]] = []  # (name, profile, state)

    # Parse blocks: look for "name:" lines and nearby "connectionState: 2"
    # Walk line-by-line keeping a small rolling context.
    lines = bt_dump.splitlines()
    i = 0
    while i < len(lines):
        name_match = re.search(r"name:\s*(.+)", lines[i])
        if name_match:
            candidate_name = name_match.group(1).strip()
            if not candidate_name or candidate_name.lower() in ("null", ""):
                i += 1
                continue

            # Look ahead up to 10 lines for connectionState and profile info
            window = lines[i : i + 10]
            connected = any(
                re.search(r"connectionState:\s*2", l) for l in window
            )
            if connected:
                # Try to identify profile from the section header above (up to 5 lines back)
                profile = "—"
                for back_line in lines[max(0, i - 5) : i]:
                    if "A2DP" in back_line:
                        profile = "A2DP"
                        break
                    if "HFP" in back_line or "HeadsetService" in back_line:
                        profile = "HFP"
                        break
                    if "HID" in back_line:
                        profile = "HID"
                        break
                    if "LE" in back_line:
                        profile = "BLE"
                        break

                # Avoid duplicates (same name + profile)
                entry = (candidate_name, profile, "Connected")
                if entry not in bt_devices:
                    bt_devices.append(entry)
        i += 1

    # ── output ────────────────────────────────────────────────────────────
    print(f"\n  Audio Route \u2014 {device.model}\n")
    print(f"  Active Output   : {active_output}")
    print()
    print("  Bluetooth Devices:")

    if bt_devices:
        name_w    = max(len(name) for name, _, _ in bt_devices)
        name_w    = max(name_w, 18)
        for name, profile, state in bt_devices:
            print(f"    {name:<{name_w}}  {profile:<6}  {state}")
    else:
        print("    (none)")

    print()
