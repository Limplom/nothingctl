package sysmon

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
)

func mhz(hz int64) string {
	if hz >= 1_000_000 {
		return fmt.Sprintf("%.2f GHz", float64(hz)/1_000_000)
	}
	return fmt.Sprintf("%.0f MHz", float64(hz)/1_000)
}

type coreInfo struct {
	core   int
	curHz  int64
	maxHz  int64
	online bool
}

func readCPUFreqs(serial string) []coreInfo {
	script := "for i in 0 1 2 3 4 5 6 7; do " +
		"  p=/sys/devices/system/cpu/cpu$i; " +
		"  [ -d $p ] || continue; " +
		"  cur=$(cat $p/cpufreq/scaling_cur_freq 2>/dev/null || echo 0); " +
		"  mx=$(cat $p/cpufreq/cpuinfo_max_freq 2>/dev/null || echo 0); " +
		"  onl=$(cat $p/online 2>/dev/null || echo 1); " +
		"  echo \"$i|$cur|$mx|$onl\"; " +
		"done"
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", script})
	var cores []coreInfo
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.Split(line, "|")
		if len(parts) == 4 {
			var core int
			var cur, mx int64
			if _, err := fmt.Sscanf(parts[0], "%d", &core); err != nil {
				continue
			}
			fmt.Sscanf(parts[1], "%d", &cur)
			fmt.Sscanf(parts[2], "%d", &mx)
			online := strings.TrimSpace(parts[3]) != "0"
			cores = append(cores, coreInfo{core, cur, mx, online})
		}
	}
	return cores
}

func classifySnapdragonCluster(core int) string {
	if core <= 3 {
		return "Silver"
	}
	if core == 7 {
		return "Prime"
	}
	return "Gold"
}

func classifyMTKCluster(maxHz int64) string {
	if maxHz <= 2_100_000 {
		return "Efficiency"
	}
	return "Performance"
}

func detectSOC(serial string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.board.platform"})
	platform := strings.ToLower(strings.TrimSpace(stdout))
	if strings.HasPrefix(platform, "mt") || strings.HasPrefix(platform, "dimensity") {
		return "mediatek"
	}
	if platform != "" && platform != "unknown" {
		return "snapdragon"
	}
	return "unknown"
}

func classifyCluster(soc string, core int, maxHz int64) string {
	if soc == "mediatek" {
		return classifyMTKCluster(maxHz)
	}
	return classifySnapdragonCluster(core)
}

type procInfo struct {
	cpuPct float64
	pid    int
	name   string
}

var headerRe = regexp.MustCompile(`^\s*PID\s+USER`)

func readTopProcesses(serial string, topN int) []procInfo {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"top -b -n 1 -o PID,USER,%CPU,%MEM,ARGS 2>/dev/null"})
	var procs []procInfo
	headerSeen := false
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if headerRe.MatchString(line) {
			headerSeen = true
			continue
		}
		if !headerSeen {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		var pid int
		var cpu float64
		if _, err := fmt.Sscanf(parts[0], "%d", &pid); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(parts[2], "%f", &cpu); err != nil {
			continue
		}
		procs = append(procs, procInfo{cpu, pid, parts[4]})
	}
	// Sort descending
	for i := 0; i < len(procs); i++ {
		for j := i + 1; j < len(procs); j++ {
			if procs[j].cpuPct > procs[i].cpuPct {
				procs[i], procs[j] = procs[j], procs[i]
			}
		}
	}
	if len(procs) > topN {
		procs = procs[:topN]
	}
	return procs
}

func cpuSnapshot(serial, model, soc string, topN int) {
	cores := readCPUFreqs(serial)
	if len(cores) == 0 {
		fmt.Println("  Could not read CPU frequency data.")
		return
	}

	fmt.Printf("\n  CPU Cores \u2014 %s  (SoC: %s)\n\n", model, soc)
	fmt.Printf("  %-8s %-13s %-8s %10s  %10s  %s\n",
		"Core", "Cluster", "Status", "Current", "Max", "Load")
	fmt.Println("  " + strings.Repeat("\u2500", 72))

	for _, c := range cores {
		cluster := classifyCluster(soc, c.core, c.maxHz)
		status := "online "
		if !c.online {
			status = "offline"
		}
		curStr := "\u2014"
		if c.online && c.curHz > 0 {
			curStr = mhz(c.curHz)
		}
		maxStr := "\u2014"
		if c.maxHz > 0 {
			maxStr = mhz(c.maxHz)
		}
		bar := strings.Repeat(" ", 20)
		pctStr := "      "
		if c.online && c.curHz > 0 && c.maxHz > 0 {
			bar = asciiBar(float64(c.curHz), float64(c.maxHz), 20)
			pctStr = fmt.Sprintf("(%4.0f%%)", float64(c.curHz)/float64(c.maxHz)*100)
		}
		fmt.Printf("  cpu%-5d %-13s %-8s %10s  %10s  %s %s\n",
			c.core, cluster, status, curStr, maxStr, bar, pctStr)
	}

	// Overall CPU utilisation from top header
	topOut, _, topCode := adb.Run([]string{"adb", "-s", serial, "shell",
		"top -b -n 1 2>/dev/null | grep -E '%cpu'"})
	if topCode == 0 && strings.TrimSpace(topOut) != "" {
		lines := strings.Split(strings.TrimSpace(topOut), "\n")
		if len(lines) > 0 {
			fmt.Printf("\n  Overall: %s\n", strings.TrimSpace(lines[0]))
		}
	}

	procs := readTopProcesses(serial, topN)
	if len(procs) > 0 {
		fmt.Printf("\n  Top %d processes by CPU:\n\n", topN)
		fmt.Printf("  %6s  %7s  %s\n", "%CPU", "PID", "Name")
		fmt.Println("  " + strings.Repeat("\u2500", 50))
		for _, p := range procs {
			bar := asciiBar(p.cpuPct, 100, 14)
			fmt.Printf("  %5.1f%%  %7d  %-30s  %s\n", p.cpuPct, p.pid, p.name, bar)
		}
	}
	fmt.Println()
}

// ActionCPUUsage displays CPU frequency per core and top-N processes by CPU usage.
func ActionCPUUsage(serial string, topN int, watch bool) error {
	model := func() string {
		s, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
		return strings.TrimSpace(s)
	}()
	soc := detectSOC(serial)

	if watch {
		fmt.Println("  CPU monitor (Ctrl-C to stop, refresh every 2s)")
		adb.WatchLoop(2*time.Second, func() {
			fmt.Printf("  Nothing %s  \u2014  live CPU\n", model)
			cpuSnapshot(serial, model, soc, topN)
		})
	} else {
		cpuSnapshot(serial, model, soc, topN)
	}
	return nil
}
