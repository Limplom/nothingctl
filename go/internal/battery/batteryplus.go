// Battery statistics and charging control for Nothing phones.
package battery

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

func secondsToHMS(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type appDrain struct {
	pkg   string
	secs  float64
	count int
}

var timeUnitRe = regexp.MustCompile(`(\d+)\s*(h|m|s|ms)`)

func parseTimeExpr(expr string) float64 {
	var total float64
	for _, m := range timeUnitRe.FindAllStringSubmatch(expr, -1) {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		switch m[2] {
		case "h":
			total += float64(n) * 3600
		case "m":
			total += float64(n) * 60
		case "s":
			total += float64(n)
		case "ms":
			total += float64(n) / 1000
		}
	}
	return total
}

var wlRe = regexp.MustCompile(`(?i)Wake lock\s+([^\s:][^:]*?):\s+((?:\d+h\s*)?(?:\d+m\s*)?(?:\d+s\s*)?(?:\d+ms\s*)?)\((\d+)\s+times?\)`)
var uidRe = regexp.MustCompile(`(?i)^\s*UID\s+(\d+):`)

func parseBatterystats(output string) []appDrain {
	type pkgData struct {
		secs  float64
		count int
	}
	uidData := make(map[int]map[string]*pkgData)
	currentUID := -1

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := uidRe.FindStringSubmatch(line); m != nil {
			var uid int
			fmt.Sscanf(m[1], "%d", &uid)
			currentUID = uid
			if _, ok := uidData[uid]; !ok {
				uidData[uid] = make(map[string]*pkgData)
			}
			continue
		}
		if currentUID < 10000 {
			continue
		}
		if m := wlRe.FindStringSubmatch(line); m != nil {
			pkg := strings.TrimSpace(m[1])
			secs := parseTimeExpr(m[2])
			var count int
			fmt.Sscanf(m[3], "%d", &count)
			if uidData[currentUID] == nil {
				uidData[currentUID] = make(map[string]*pkgData)
			}
			if _, ok := uidData[currentUID][pkg]; !ok {
				uidData[currentUID][pkg] = &pkgData{}
			}
			uidData[currentUID][pkg].secs += secs
			uidData[currentUID][pkg].count += count
		}
	}

	var flat []appDrain
	for _, pkgs := range uidData {
		for pkg, d := range pkgs {
			flat = append(flat, appDrain{pkg, d.secs, d.count})
		}
	}
	// Sort descending by secs
	sort.Slice(flat, func(i, j int) bool { return flat[i].secs > flat[j].secs })
	return flat
}

// ActionBatteryStats shows per-app battery drain (wakelock times) and charge cycles.
func ActionBatteryStats(serial string) error {
	model := adb.ShellStr(serial, "getprop ro.product.model")

	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "dumpsys battery"})
	fields := parseDumpsysBattery(stdout)

	levelRaw, hasLevel := intField(fields, "level")
	statusRaw, hasStatus := intField(fields, "status")
	healthRaw, hasHealth := intField(fields, "health")
	tempRaw, hasTemp := intField(fields, "temperature")
	voltageRaw, hasVoltage := intField(fields, "voltage")
	pluggedRaw, hasPlugged := intField(fields, "plugged")

	levelStr := "n/a"
	if hasLevel {
		levelStr = fmt.Sprintf("%d %%", levelRaw)
	}
	healthStr := "n/a"
	if hasHealth {
		if l, ok := healthLabels[healthRaw]; ok {
			healthStr = l
		} else {
			healthStr = fmt.Sprintf("unknown (%d)", healthRaw)
		}
	}

	statusStr := "n/a"
	if hasStatus {
		base := "n/a"
		if l, ok := statusLabels[statusRaw]; ok {
			base = l
		}
		if statusRaw == 2 && hasPlugged {
			if l, ok := pluggedLabels[pluggedRaw]; ok && l != "Not plugged" {
				base = fmt.Sprintf("%s (%s)", base, l)
			}
		}
		statusStr = base
	} else if fields["AC powered"] == "true" {
		statusStr = "Charging (AC)"
	} else if fields["USB powered"] == "true" {
		statusStr = "Charging (USB)"
	} else if fields["Wireless powered"] == "true" {
		statusStr = "Charging (Wireless)"
	}

	tempStr := "n/a"
	if hasTemp {
		tempStr = fmt.Sprintf("%.1f \u00b0C", float64(tempRaw)/10)
	}
	voltageStr := "n/a"
	if hasVoltage {
		voltageStr = fmt.Sprintf("%.2f V", float64(voltageRaw)/1000)
	}

	cycleRaw := adb.ShellStr(serial, "cat /sys/class/power_supply/battery/cycle_count 2>/dev/null")
	cycleStr := "(not available on this device)"
	if cycleRaw != "" {
		var n int
		if _, err := fmt.Sscanf(cycleRaw, "%d", &n); err == nil {
			cycleStr = fmt.Sprintf("%d", n)
		} else {
			cycleStr = "(not available)"
		}
	}

	baseband := adb.ShellStr(serial, "getprop gsm.version.baseband")
	if strings.Contains(baseband, ",") {
		for _, b := range strings.Split(baseband, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				baseband = b
				break
			}
		}
	}

	statsOut, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "dumpsys batterystats --charged"})
	var drainList []appDrain
	if strings.TrimSpace(statsOut) != "" {
		drainList = parseBatterystats(statsOut)
	}
	topApps := drainList
	if len(topApps) > 10 {
		topApps = topApps[:10]
	}

	fmt.Printf("\n  Battery Stats \u2014 %s\n\n", model)
	fmt.Printf("  %-14s: %s\n", "Level", levelStr)
	fmt.Printf("  %-14s: %s\n", "Status", statusStr)
	fmt.Printf("  %-14s: %s\n", "Temperature", tempStr)
	fmt.Printf("  %-14s: %s\n", "Voltage", voltageStr)
	fmt.Printf("  %-14s: %s\n", "Health", healthStr)
	fmt.Printf("  %-14s: %s\n", "Charge Cycles", cycleStr)
	if baseband != "" {
		fmt.Printf("  %-14s: %s\n", "Baseband", baseband)
	}
	fmt.Println()

	fmt.Println("  Top App Drain (since last charge):")
	if len(topApps) > 0 {
		fmt.Printf("  %-4s  %-36s  %-12s  %s\n", "#", "App", "Wake Time", "Wakelocks")
		fmt.Printf("  %-4s  %-36s  %-12s  %s\n", "----", "------------------------------------", "------------", "---------")
		for i, d := range topApps {
			fmt.Printf("  %-4d  %-36s  %-12s  %d\n", i+1, d.pkg, secondsToHMS(d.secs), d.count)
		}
	} else {
		fmt.Println("  (no wakelock data available — try again after using the device)")
	}
	fmt.Println()
	return nil
}

var chargeLimitPaths = []string{
	"/sys/class/power_supply/battery/charge_control_end_threshold",
	"/sys/class/power_supply/battery/charge_cutoff_percent",
}

// ActionChargingControl reads or sets the charge limit via sysfs.
func ActionChargingControl(serial string, limit int) error {
	model := adb.ShellStr(serial, "getprop ro.product.model")

	// Detect which sysfs path exists
	activePath := ""
	for _, path := range chargeLimitPaths {
		probe := adb.ShellStr(serial, fmt.Sprintf("[ -f %s ] && echo yes || echo no", path))
		if probe == "yes" {
			activePath = path
			break
		}
	}

	if activePath == "" {
		fmt.Printf("\n  Charge limit control is not supported on %s.\n", model)
		fmt.Println("  The required sysfs node was not found:")
		for _, path := range chargeLimitPaths {
			fmt.Printf("    %s\n", path)
		}
		fmt.Println("\n  This feature requires a kernel that exposes one of the above nodes.")
		fmt.Println("  Custom kernels (e.g. Sultan, Asus) sometimes add this capability.")
		fmt.Println()
		return nil
	}

	// Read-only mode (limit == 0 means read)
	if limit == 0 {
		current := adb.ShellStr(serial, fmt.Sprintf("cat %s 2>/dev/null", activePath))
		fmt.Printf("\n  Charge Limit \u2014 %s\n\n", model)
		if current != "" {
			var pct int
			if _, err := fmt.Sscanf(current, "%d", &pct); err == nil {
				fmt.Printf("  %-16s: %d %%\n", "Current limit", pct)
			} else {
				fmt.Printf("  %-16s: %s (raw)\n", "Current limit", current)
			}
		} else {
			fmt.Printf("  %-16s: (could not read)\n", "Current limit")
		}
		fmt.Printf("  %-16s: %s\n", "Sysfs path", activePath)
		fmt.Println()
		return nil
	}

	if limit < 20 || limit > 100 {
		return nterrors.AdbError(fmt.Sprintf("Charge limit must be between 20 and 100 (got %d).", limit))
	}

	writeCmd := fmt.Sprintf("su -c 'echo %d > %s'", limit, activePath)
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", writeCmd})
	if code != 0 || strings.Contains(strings.ToLower(stderr), "not found") {
		err := strings.TrimSpace(stderr)
		if err == "" {
			err = "(no details)"
		}
		return nterrors.AdbError(fmt.Sprintf("Failed to set charge limit to %d %% on %s.\n  Root access is required. Error: %s", limit, model, err))
	}

	verify := adb.ShellStr(serial, fmt.Sprintf("cat %s 2>/dev/null", activePath))
	var written int
	hasWritten := false
	if _, err := fmt.Sscanf(verify, "%d", &written); err == nil {
		hasWritten = true
	}

	fmt.Printf("\n  Charge Limit \u2014 %s\n\n", model)
	if hasWritten && written == limit {
		fmt.Printf("  Charge limit set to %d %% successfully.\n", limit)
	} else if hasWritten {
		fmt.Printf("  Write appeared to succeed but device reports %d %%\n", written)
		fmt.Printf("  (requested %d %%). The kernel may have clamped the value.\n", limit)
	} else {
		fmt.Printf("  Write command returned success but could not verify the new value.\n")
		fmt.Printf("  Requested limit: %d %%\n", limit)
	}
	fmt.Printf("  Sysfs path: %s\n", activePath)
	fmt.Println()
	return nil
}
