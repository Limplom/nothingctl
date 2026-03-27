// Package battery provides battery health reporting for Nothing phones.
package battery

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

var healthLabels = map[int]string{
	1: "Unknown",
	2: "Good",
	3: "Overheat",
	4: "Dead",
	5: "Over voltage",
	6: "Unspecified failure",
	7: "Cold",
}

var statusLabels = map[int]string{
	1: "Unknown",
	2: "Charging",
	3: "Discharging",
	4: "Not charging",
	5: "Full",
}

var pluggedLabels = map[int]string{
	0: "Not plugged",
	1: "AC",
	2: "USB",
	4: "Wireless",
}

func parseDumpsysBattery(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if idx := strings.Index(line, ": "); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+2:])
			result[key] = val
		}
	}
	return result
}

func getCycleCount(serial string) string {
	// Method 1: batterystats
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys batterystats | grep -E 'Charge cycle count'"})
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "Charge cycle count") && strings.Contains(line, "=") {
			parts := strings.Split(line, "=")
			raw := strings.Fields(parts[len(parts)-1])
			if len(raw) > 0 {
				var n int
				if _, err := fmt.Sscanf(raw[0], "%d", &n); err == nil {
					return fmt.Sprintf("%d", n)
				}
			}
		}
	}
	// Method 2: sysfs
	stdout2, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"cat /sys/class/power_supply/battery/cycle_count 2>/dev/null"})
	raw := strings.TrimSpace(stdout2)
	if raw != "" {
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err == nil {
			return fmt.Sprintf("%d", n)
		}
	}
	return "(not available on this kernel)"
}

func intField(fields map[string]string, key string) (int, bool) {
	v, ok := fields[key]
	if !ok {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

// ActionBattery displays a battery health report for the connected Nothing phone.
func ActionBattery(serial string) error {
	model := strings.TrimSpace(func() string {
		s, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
		return s
	}())

	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "dumpsys battery"})
	fields := parseDumpsysBattery(stdout)

	levelRaw, hasLevel := intField(fields, "level")
	statusRaw, hasStatus := intField(fields, "status")
	healthRaw, hasHealth := intField(fields, "health")
	tempRaw, hasTemp := intField(fields, "temperature")
	voltageRaw, hasVoltage := intField(fields, "voltage")
	pluggedRaw, hasPlugged := intField(fields, "plugged")

	levelStr := "not available"
	if hasLevel {
		levelStr = fmt.Sprintf("%d %%", levelRaw)
	}
	statusStr := "not available"
	if hasStatus {
		if label, ok := statusLabels[statusRaw]; ok {
			statusStr = label
		} else {
			statusStr = fmt.Sprintf("unknown (%d)", statusRaw)
		}
	}
	healthStr := "not available"
	if hasHealth {
		if label, ok := healthLabels[healthRaw]; ok {
			healthStr = label
		} else {
			healthStr = fmt.Sprintf("unknown (%d)", healthRaw)
		}
	}

	pluggedStr := "Not plugged"
	if hasPlugged {
		if label, ok := pluggedLabels[pluggedRaw]; ok {
			pluggedStr = label
		} else {
			pluggedStr = fmt.Sprintf("unknown (%d)", pluggedRaw)
		}
	} else if fields["AC powered"] == "true" {
		pluggedStr = "AC"
	} else if fields["USB powered"] == "true" {
		pluggedStr = "USB"
	} else if fields["Wireless powered"] == "true" {
		pluggedStr = "Wireless"
	} else if fields["Dock powered"] == "true" {
		pluggedStr = "Dock"
	}

	tempStr := "not available"
	if hasTemp {
		tempStr = fmt.Sprintf("%.1f \u00b0C", float64(tempRaw)/10)
	}
	voltageStr := "not available"
	if hasVoltage {
		voltageStr = fmt.Sprintf("%.2f V", float64(voltageRaw)/1000)
	}

	cycleStr := getCycleCount(serial)

	// Battery capacity
	capacityStr := ""
	for _, node := range []struct{ path, label string }{
		{"/sys/class/power_supply/battery/charge_full", "current max"},
		{"/sys/class/power_supply/battery/charge_full_design", "design"},
	} {
		out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			"cat " + node.path + " 2>/dev/null"})
		raw := strings.TrimSpace(out)
		if raw != "" {
			var uah int64
			if _, err := fmt.Sscanf(raw, "%d", &uah); err == nil {
				mah := uah / 1000
				if mah >= 500 {
					capacityStr = fmt.Sprintf("%d mAh (%s)", mah, node.label)
					break
				}
			}
		}
	}

	fmt.Printf("\n  Battery Report \u2014 %s\n\n", model)
	fmt.Printf("  %-13s: %s\n", "Level", levelStr)
	fmt.Printf("  %-13s: %s\n", "Status", statusStr)
	fmt.Printf("  %-13s: %s\n", "Health", healthStr)
	fmt.Printf("  %-13s: %s\n", "Temperature", tempStr)
	fmt.Printf("  %-13s: %s\n", "Voltage", voltageStr)
	fmt.Printf("  %-13s: %s\n", "Plugged", pluggedStr)
	fmt.Println()
	fmt.Printf("  %-13s: %s  (estimated \u2014 varies by kernel)\n", "Cycle count", cycleStr)
	if capacityStr != "" {
		fmt.Printf("  %-13s: %s\n", "Capacity", capacityStr)
	}
	fmt.Println()
	return nil
}
