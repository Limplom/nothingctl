"""Thermal zone monitor for Nothing phones (Snapdragon & MediaTek)."""

import time

from .device import run
from .models import DeviceInfo

# Human-friendly names — covers both Snapdragon (QC) and MediaTek (MTK) Nothing phones.
_ZONE_LABELS = {
    # ── MediaTek (Dimensity) ─────────────────────────────────────────────────
    "soc_max":              "SoC max",
    "soc-top1":             "SoC top",
    "soc-top2":             "SoC top (2)",
    "soc-bot1":             "SoC bottom",
    "soc-bot2":             "SoC bottom (2)",
    "cpu-big-core0-1":      "CPU big core 0",
    "cpu-big-core0-2":      "CPU big core 0",
    "cpu-big-core1-1":      "CPU big core 1",
    "cpu-big-core1-2":      "CPU big core 1",
    "cpu-big-core2-1":      "CPU big core 2",
    "cpu-big-core2-2":      "CPU big core 2",
    "cpu-big-core3-1":      "CPU big core 3",
    "cpu-big-core3-2":      "CPU big core 3",
    "cpu-little-core0":     "CPU little core 0",
    "cpu-little-core1":     "CPU little core 1",
    "cpu-little-core2":     "CPU little core 2",
    "cpu-little-core3":     "CPU little core 3",
    "cpu-dsu-1":            "CPU cache (DSU)",
    "cpu-dsu-2":            "CPU cache (DSU)",
    "apu":                  "AI processor (APU)",
    "gpu":                  "GPU",
    "md1":                  "Modem",
    "md2":                  "Modem",
    "md3":                  "Modem",
    "md4":                  "Modem",
    "battery":              "Battery",
    "board_ntc":            "Board (NTC)",
    "ap_ntc":               "Application processor (NTC)",
    "wifi_ntc":             "WiFi module",
    "cam_ntc":              "Camera module",
    "flash_light_ntc":      "Flash / torch",
    "usb_board":            "USB area",
    "usb":                  "USB port",
    "shell_front":          "Shell (front)",
    "shell_back":           "Shell (back)",
    "shell_frame":          "Shell (frame)",
    "shell_max":            "Shell max",
    "ambient":              "Ambient",
    "ltepa_ntc":            "LTE PA",
    "nrpa_ntc":             "NR PA",
    "tsx-ntc":              "TSX",
    "sc_buck_ntc":          "SC buck",
    "consys":               "Connectivity subsystem",
    "mtk-master-charger":   "Charger (main)",
    "mtk-slave-charger":    "Charger (slave)",
    # ── Snapdragon (QC) ─────────────────────────────────────────────────────
    "cpu-0-0-usr":          "CPU cluster 0 (efficiency)",
    "cpu-0-1-usr":          "CPU cluster 0 (efficiency)",
    "cpu-1-0-usr":          "CPU cluster 1 (performance)",
    "cpu-1-1-usr":          "CPU cluster 1 (performance)",
    "cpu-1-2-usr":          "CPU cluster 1 (prime)",
    "cpuss-0-usr":          "CPU subsystem",
    "cpuss-2-usr":          "CPU subsystem",
    "aoss-0":               "SoC main",
    "aoss-1":               "SoC secondary",
    "gpuss-0-usr":          "GPU",
    "gpuss-1-usr":          "GPU",
    "skin-therm-usr":       "Skin temperature",
    "skin-therm":           "Skin temperature",
    "quiet-therm-usr":      "Quiet (near camera)",
    "xo-therm-usr":         "Crystal oscillator",
    "mdm-vq6-usr":          "Modem",
    "mdm-lte-usr":          "Modem LTE",
    "pa-therm0-usr":        "Power amp",
    "pm8350b-bcl-lvl0":     "Battery current limit",
}

# Zones shown first, sorted by temperature.
_PRIORITY_ZONES = {
    # MediaTek
    "soc_max", "shell_max", "shell_front", "shell_back",
    "apu", "gpu",
    "cpu-big-core0-1", "cpu-big-core1-1", "cpu-big-core2-1", "cpu-big-core3-1",
    "cpu-little-core0", "battery",
    # Snapdragon
    "skin-therm", "skin-therm-usr", "aoss-0", "gpuss-0-usr",
    "cpu-1-2-usr", "cpu-1-0-usr", "cpu-0-0-usr",
}

# Sentinel value MediaTek uses when a thermal sensor is not populated.
_INVALID_TEMP = -274000


def _read_thermal_zones(serial: str) -> list[tuple[str, str, int]]:
    """Return list of (zone_path, zone_type, temp_milli_celsius)."""
    # /sys/class/thermal/ requires root on Nothing kernels
    r = run(["adb", "-s", serial, "shell",
             "su -c 'for d in /sys/class/thermal/thermal_zone*/; do "
             "  t=$(cat $d/type 2>/dev/null); "
             "  v=$(cat $d/temp 2>/dev/null); "
             "  echo \"$d|$t|$v\"; "
             "done'"])
    results = []
    for line in r.stdout.strip().splitlines():
        parts = line.split("|")
        if len(parts) == 3:
            zone, ztype, temp_raw = parts
            try:
                temp = int(temp_raw.strip())
                if temp <= _INVALID_TEMP:
                    continue          # sensor not populated, skip
                results.append((zone.strip(), ztype.strip(), temp))
            except ValueError:
                pass
    return results


def _format_temp(milli_c: int) -> str:
    c = milli_c / 1000
    bar_len = max(0, int((c - 20) / 2))     # scale: 20°C = 0, 70°C = 25
    bar     = "█" * min(bar_len, 30)
    warn    = " !" if c >= 60 else "  "
    return f"{c:5.1f} °C  {bar}{warn}"


def action_thermal(device: DeviceInfo, watch: bool = False) -> None:
    """
    Display thermal zone temperatures. With watch=True, refresh every 2 seconds.
    Shows priority zones (CPU, GPU, shell) prominently; all others follow.
    Filters out unpopulated sensors (reported as ~-274°C by MediaTek kernels).
    """
    def _snapshot():
        zones = _read_thermal_zones(device.serial)
        if not zones:
            print("  No thermal zones found (root may be required on some kernels)")
            return

        priority   = [(z, t, v) for z, t, v in zones if t in _PRIORITY_ZONES]
        other      = [(z, t, v) for z, t, v in zones if t not in _PRIORITY_ZONES]
        other_sort = sorted(other, key=lambda x: x[2], reverse=True)

        print(f"\n  {'Zone type':<28} {'Temperature':>20}")
        print("  " + "─" * 55)
        if priority:
            for _, ztype, temp in sorted(priority, key=lambda x: x[2], reverse=True):
                label = _ZONE_LABELS.get(ztype, ztype)
                print(f"  {label:<28} {_format_temp(temp):>20}")
            print()
        for _, ztype, temp in other_sort[:10]:   # top 10 remaining
            label = _ZONE_LABELS.get(ztype, ztype) or ztype
            print(f"  {label:<28} {_format_temp(temp):>20}")
        print(f"\n  ! = above 60°C (throttling likely)")

    if watch:
        print("Thermal monitor (Ctrl-C to stop, refresh every 2s)\n")
        try:
            while True:
                print("\033[2J\033[H", end="")  # clear screen
                print(f"  Nothing {device.model}  —  live thermal\n")
                _snapshot()
                time.sleep(2)
        except KeyboardInterrupt:
            print("\nStopped.")
    else:
        _snapshot()
