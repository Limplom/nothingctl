// Package sysmon provides RAM and CPU usage monitoring for Nothing phones.
package sysmon

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
)

func asciiBar(value, total float64, width int) string {
	if total <= 0 {
		return strings.Repeat(" ", width)
	}
	pct := value / total
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

func mib(kb int64) string {
	m := float64(kb) / 1024
	if m >= 1024 {
		return fmt.Sprintf("%.2f GiB", m/1024)
	}
	return fmt.Sprintf("%.0f MiB", m)
}

var memKeyRe = regexp.MustCompile(`^(\w[\w()]+):\s+(\d+)`)

func readProcMeminfo(serial string) map[string]int64 {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "cat /proc/meminfo"})
	result := make(map[string]int64)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := memKeyRe.FindStringSubmatch(line); m != nil {
			var v int64
			if _, err := fmt.Sscanf(m[2], "%d", &v); err == nil {
				result[m[1]] = v
			}
		}
	}
	return result
}

var rssSectionRe = regexp.MustCompile(`^\s*([\d,]+)K:\s+(.+)$`)
var pidSuffixRe = regexp.MustCompile(`\s*\(pid\s+\d+.*?\)\s*$`)

func parseRssSummary(output string) [][2]interface{} {
	var entries [][2]interface{}
	inSection := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.Contains(line, "Total RSS by process") {
			inSection = true
			continue
		}
		if inSection {
			stripped := strings.TrimSpace(line)
			if stripped == "" {
				break
			}
			if m := rssSectionRe.FindStringSubmatch(stripped); m != nil {
				kbStr := strings.ReplaceAll(m[1], ",", "")
				var kb int64
				if _, err := fmt.Sscanf(kbStr, "%d", &kb); err == nil {
					name := pidSuffixRe.ReplaceAllString(m[2], "")
					name = strings.TrimSpace(name)
					entries = append(entries, [2]interface{}{kb, name})
				}
			}
		}
	}
	return entries
}

var appMemRe = regexp.MustCompile(`^\s{0,10}([\w ]+?):\s{1,10}(\d+)`)

var interestingMemLabels = map[string]bool{
	"Java Heap": true, "Native Heap": true, "Code": true, "Stack": true,
	"Graphics": true, "Private Other": true, "System": true, "Unknown": true,
	"TOTAL PSS": true, "TOTAL RSS": true, "TOTAL SWAP": true,
}

func parseAppMeminfo(output string) [][2]interface{} {
	var rows [][2]interface{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := appMemRe.FindStringSubmatch(line); m != nil {
			label := strings.TrimSpace(m[1])
			if interestingMemLabels[label] {
				var kb int64
				if _, err := fmt.Sscanf(m[2], "%d", &kb); err == nil {
					rows = append(rows, [2]interface{}{label, kb})
				}
			}
		}
	}
	return rows
}

func snapshotSystem(serial, model string) {
	mem := readProcMeminfo(serial)
	total := mem["MemTotal"]
	free := mem["MemFree"]
	avail := mem["MemAvailable"]
	bufs := mem["Buffers"]
	cached := mem["Cached"]
	used := total - avail

	fmt.Printf("\n  RAM Summary \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %10s\n", "Total", mib(total))
	fmt.Printf("  %-16s: %10s  %s\n", "Used (est.)", mib(used), asciiBar(float64(used), float64(total), 24))
	fmt.Printf("  %-16s: %10s  %s\n", "Available", mib(avail), asciiBar(float64(avail), float64(total), 24))
	fmt.Printf("  %-16s: %10s\n", "Free", mib(free))
	fmt.Printf("  %-16s: %10s\n", "Buffers", mib(bufs))
	fmt.Printf("  %-16s: %10s\n", "Cached", mib(cached))

	swapTotal := mem["SwapTotal"]
	swapFree := mem["SwapFree"]
	if swapTotal > 0 {
		swapUsed := swapTotal - swapFree
		fmt.Printf("\n  %-16s: %10s\n", "Swap total", mib(swapTotal))
		fmt.Printf("  %-16s: %10s  %s\n", "Swap used", mib(swapUsed), asciiBar(float64(swapUsed), float64(swapTotal), 24))
	}

	r2out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys meminfo 2>/dev/null | head -120"})
	entries := parseRssSummary(r2out)
	if len(entries) > 0 {
		fmt.Printf("\n  Top processes by RSS:\n\n")
		fmt.Printf("  %-48s %10s\n", "Process", "RSS")
		fmt.Println("  " + strings.Repeat("\u2500", 62))
		limit := 10
		if len(entries) < limit {
			limit = len(entries)
		}
		topKB := entries[0][0].(int64)
		for _, e := range entries[:limit] {
			kb := e[0].(int64)
			name := e[1].(string)
			bar := asciiBar(float64(kb), float64(topKB), 12)
			fmt.Printf("  %-48s %10s  %s\n", name, mib(kb), bar)
		}
	}

	r3out, _, r3code := adb.Run([]string{"adb", "-s", serial, "shell",
		"cat /sys/module/lowmemorykiller/parameters/minfree 2>/dev/null"})
	if r3code == 0 && strings.TrimSpace(r3out) != "" && strings.TrimSpace(r3out) != "N/A" {
		pages := strings.Split(strings.TrimSpace(r3out), ",")
		adjLevels := []string{"foreground", "visible", "secondary_server", "hidden", "content_provider", "empty"}
		fmt.Println("\n  LowMemoryKiller minfree thresholds (pages \u00d7 4 kB):")
		for i, p := range pages {
			var pg int64
			if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &pg); err == nil {
				label := fmt.Sprintf("level%d", i)
				if i < len(adjLevels) {
					label = adjLevels[i]
				}
				fmt.Printf("    %-22s: %s\n", label, mib(pg*4))
			}
		}
	}
	fmt.Println()
}

func snapshotPackage(serial, model, pkg string) {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys meminfo " + pkg + " 2>/dev/null"})
	output := strings.TrimSpace(stdout)
	if output == "" || strings.Contains(output, "No process found") || strings.Contains(output, "No services found") {
		fmt.Printf("\n  Package '%s' not found or not running on %s.\n\n", pkg, model)
		return
	}

	rows := parseAppMeminfo(output)
	fmt.Printf("\n  Memory detail \u2014 %s\n  Device: %s\n\n", pkg, model)
	if len(rows) > 0 {
		fmt.Printf("  %-20s %10s  %s\n", "Category", "PSS total", "bar")
		fmt.Println("  " + strings.Repeat("\u2500", 52))
		var totalPSS int64
		for _, r := range rows {
			if r[0].(string) == "TOTAL PSS" {
				totalPSS = r[1].(int64)
				break
			}
		}
		scale := totalPSS
		if scale == 0 {
			for _, r := range rows {
				v := r[1].(int64)
				if v > scale {
					scale = v
				}
			}
		}
		for _, r := range rows {
			label := r[0].(string)
			kb := r[1].(int64)
			bar := ""
			if scale > 0 {
				bar = asciiBar(float64(kb), float64(scale), 24)
			}
			fmt.Printf("  %-20s %10s  %s\n", label, mib(kb), bar)
		}
	} else {
		lines := strings.Split(output, "\n")
		if len(lines) > 40 {
			lines = lines[:40]
		}
		for _, l := range lines {
			fmt.Printf("  %s\n", l)
		}
	}
	fmt.Println()
}

// ActionMemory displays RAM usage, optionally watching, optionally for a specific package.
func ActionMemory(serial, packageName string, watch bool) error {
	model := func() string {
		s, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
		return strings.TrimSpace(s)
	}()

	if watch {
		label := "live memory"
		if packageName != "" {
			label = "live memory \u2014 " + packageName
		}
		fmt.Printf("  Memory monitor (Ctrl-C to stop, refresh every 2s)\n")
		adb.WatchLoop(2*time.Second, func() {
			fmt.Printf("  Nothing %s  \u2014  %s\n", model, label)
			if packageName != "" {
				snapshotPackage(serial, model, packageName)
			} else {
				snapshotSystem(serial, model)
			}
		})
	} else {
		if packageName != "" {
			snapshotPackage(serial, model, packageName)
		} else {
			snapshotSystem(serial, model)
		}
	}
	return nil
}
