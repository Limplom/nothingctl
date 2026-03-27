// Package thermal provides thermal zone monitoring for Nothing phones.
package thermal

import (
	"fmt"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
)

var zoneLabels = map[string]string{
	// MediaTek (Dimensity)
	"soc_max":            "SoC max",
	"soc-top1":           "SoC top",
	"soc-top2":           "SoC top (2)",
	"soc-bot1":           "SoC bottom",
	"soc-bot2":           "SoC bottom (2)",
	"cpu-big-core0-1":    "CPU big core 0",
	"cpu-big-core0-2":    "CPU big core 0",
	"cpu-big-core1-1":    "CPU big core 1",
	"cpu-big-core1-2":    "CPU big core 1",
	"cpu-big-core2-1":    "CPU big core 2",
	"cpu-big-core2-2":    "CPU big core 2",
	"cpu-big-core3-1":    "CPU big core 3",
	"cpu-big-core3-2":    "CPU big core 3",
	"cpu-little-core0":   "CPU little core 0",
	"cpu-little-core1":   "CPU little core 1",
	"cpu-little-core2":   "CPU little core 2",
	"cpu-little-core3":   "CPU little core 3",
	"cpu-dsu-1":          "CPU cache (DSU)",
	"cpu-dsu-2":          "CPU cache (DSU)",
	"apu":                "AI processor (APU)",
	"gpu":                "GPU",
	"md1":                "Modem",
	"md2":                "Modem",
	"md3":                "Modem",
	"md4":                "Modem",
	"battery":            "Battery",
	"board_ntc":          "Board (NTC)",
	"ap_ntc":             "Application processor (NTC)",
	"wifi_ntc":           "WiFi module",
	"cam_ntc":            "Camera module",
	"flash_light_ntc":    "Flash / torch",
	"usb_board":          "USB area",
	"usb":                "USB port",
	"shell_front":        "Shell (front)",
	"shell_back":         "Shell (back)",
	"shell_frame":        "Shell (frame)",
	"shell_max":          "Shell max",
	"ambient":            "Ambient",
	"ltepa_ntc":          "LTE PA",
	"nrpa_ntc":           "NR PA",
	"tsx-ntc":            "TSX",
	"sc_buck_ntc":        "SC buck",
	"consys":             "Connectivity subsystem",
	"mtk-master-charger": "Charger (main)",
	"mtk-slave-charger":  "Charger (slave)",
	// Snapdragon (QC)
	"cpu-0-0-usr":        "CPU cluster 0 (efficiency)",
	"cpu-0-1-usr":        "CPU cluster 0 (efficiency)",
	"cpu-1-0-usr":        "CPU cluster 1 (performance)",
	"cpu-1-1-usr":        "CPU cluster 1 (performance)",
	"cpu-1-2-usr":        "CPU cluster 1 (prime)",
	"cpuss-0-usr":        "CPU subsystem",
	"cpuss-2-usr":        "CPU subsystem",
	"aoss-0":             "SoC main",
	"aoss-1":             "SoC secondary",
	"gpuss-0-usr":        "GPU",
	"gpuss-1-usr":        "GPU",
	"skin-therm-usr":     "Skin temperature",
	"skin-therm":         "Skin temperature",
	"quiet-therm-usr":    "Quiet (near camera)",
	"xo-therm-usr":       "Crystal oscillator",
	"mdm-vq6-usr":        "Modem",
	"mdm-lte-usr":        "Modem LTE",
	"pa-therm0-usr":      "Power amp",
	"pm8350b-bcl-lvl0":   "Battery current limit",
}

var priorityZones = map[string]bool{
	"soc_max": true, "shell_max": true, "shell_front": true, "shell_back": true,
	"apu": true, "gpu": true,
	"cpu-big-core0-1": true, "cpu-big-core1-1": true, "cpu-big-core2-1": true, "cpu-big-core3-1": true,
	"cpu-little-core0": true, "battery": true,
	"skin-therm": true, "skin-therm-usr": true, "aoss-0": true, "gpuss-0-usr": true,
	"cpu-1-2-usr": true, "cpu-1-0-usr": true, "cpu-0-0-usr": true,
}

const invalidTemp = -274000

type thermalZone struct {
	path  string
	ztype string
	temp  int
}

func readThermalZones(serial string) []thermalZone {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"su -c 'for d in /sys/class/thermal/thermal_zone*/; do " +
			"  t=$(cat $d/type 2>/dev/null); " +
			"  v=$(cat $d/temp 2>/dev/null); " +
			"  echo \"$d|$t|$v\"; " +
			"done'"})
	var results []thermalZone
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.Split(line, "|")
		if len(parts) == 3 {
			var temp int
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &temp); err == nil {
				if temp <= invalidTemp {
					continue
				}
				results = append(results, thermalZone{
					path:  strings.TrimSpace(parts[0]),
					ztype: strings.TrimSpace(parts[1]),
					temp:  temp,
				})
			}
		}
	}
	return results
}

func formatTemp(milliC int) string {
	c := float64(milliC) / 1000
	barLen := int((c - 20) / 2)
	if barLen < 0 {
		barLen = 0
	}
	if barLen > 30 {
		barLen = 30
	}
	bar := strings.Repeat("\u2588", barLen)
	warn := "  "
	if c >= 60 {
		warn = " !"
	}
	return fmt.Sprintf("%5.1f \u00b0C  %s%s", c, bar, warn)
}

func snapshot(serial string) {
	zones := readThermalZones(serial)
	if len(zones) == 0 {
		fmt.Println("  No thermal zones found (root may be required on some kernels)")
		return
	}

	var priority []thermalZone
	var other []thermalZone
	for _, z := range zones {
		if priorityZones[z.ztype] {
			priority = append(priority, z)
		} else {
			other = append(other, z)
		}
	}
	// Sort priority by temp desc
	for i := 0; i < len(priority); i++ {
		for j := i + 1; j < len(priority); j++ {
			if priority[j].temp > priority[i].temp {
				priority[i], priority[j] = priority[j], priority[i]
			}
		}
	}
	// Sort other by temp desc
	for i := 0; i < len(other); i++ {
		for j := i + 1; j < len(other); j++ {
			if other[j].temp > other[i].temp {
				other[i], other[j] = other[j], other[i]
			}
		}
	}

	fmt.Printf("\n  %-28s %20s\n", "Zone type", "Temperature")
	fmt.Println("  " + strings.Repeat("\u2500", 55))
	if len(priority) > 0 {
		for _, z := range priority {
			label, ok := zoneLabels[z.ztype]
			if !ok {
				label = z.ztype
			}
			fmt.Printf("  %-28s %20s\n", label, formatTemp(z.temp))
		}
		fmt.Println()
	}
	limit := 10
	if len(other) < limit {
		limit = len(other)
	}
	for _, z := range other[:limit] {
		label, ok := zoneLabels[z.ztype]
		if !ok {
			label = z.ztype
		}
		fmt.Printf("  %-28s %20s\n", label, formatTemp(z.temp))
	}
	fmt.Println("\n  ! = above 60\u00b0C (throttling likely)")
}

// ActionThermal displays thermal zone temperatures, optionally watching.
func ActionThermal(serial string, watch bool) error {
	model := func() string {
		s, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
		return strings.TrimSpace(s)
	}()

	if watch {
		adb.WatchLoop(2*time.Second, func() {
			fmt.Printf("  Nothing %s  \u2014  live thermal\n", model)
			snapshot(serial)
		})
	} else {
		snapshot(serial)
	}
	return nil
}
