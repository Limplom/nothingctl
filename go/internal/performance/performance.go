// Package performance manages CPU governor, I/O scheduler, and thermal profile.
package performance

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var deadlineSchedulers = []string{"sda", "sdb", "sdc", "mmcblk0"}

func getCPUGovernor(serial string) string {
	out, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
		"su -c 'cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_governor'"})
	if code == 0 && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return "unknown"
}

func getIOScheduler(serial string) string {
	for _, dev := range deadlineSchedulers {
		out, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("su -c 'cat /sys/block/%s/queue/scheduler 2>/dev/null'", dev)})
		if code == 0 && strings.TrimSpace(out) != "" {
			return strings.TrimSpace(out)
		}
	}
	return "unknown"
}

func getThermalProfile(serial string) string {
	out, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
		"getprop vendor.powerhal.profile 2>/dev/null"})
	if code == 0 && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return "(not available)"
}

func printCurrentState(serial string) {
	gov := getCPUGovernor(serial)
	scheduler := getIOScheduler(serial)
	thermal := getThermalProfile(serial)
	fmt.Printf("\n  Current CPU governor : %s\n", gov)
	fmt.Printf("  Current I/O scheduler: %s\n", scheduler)
	fmt.Printf("  Thermal profile      : %s\n", thermal)
}

func countCPUs(serial string) int {
	out, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
		"su -c 'ls /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor 2>/dev/null'"})
	if code != 0 || strings.TrimSpace(out) == "" {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(out), "\n"))
}

func applyGovernorLoop(serial, governor string) int {
	cmd := fmt.Sprintf(
		"for CPU in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do "+
			"  su -c 'echo %s > $CPU' 2>/dev/null; "+
			"done", governor)
	adb.Run([]string{"adb", "-s", serial, "shell", cmd})
	return countCPUs(serial)
}

func applyIOScheduler(serial, scheduler string) []string {
	var applied []string
	for _, dev := range deadlineSchedulers {
		adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("su -c 'echo %s > /sys/block/%s/queue/scheduler 2>/dev/null'", scheduler, dev)})
		check, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("su -c 'cat /sys/block/%s/queue/scheduler 2>/dev/null'", dev)})
		if code == 0 && strings.TrimSpace(check) != "" {
			applied = append(applied, dev)
		}
	}
	return applied
}

func detectBalancedGovernor(serial string) string {
	out, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.board.platform"})
	if code == 0 && strings.Contains(strings.ToLower(strings.TrimSpace(out)), "mt") {
		avail, _, avCode := adb.Run([]string{"adb", "-s", serial, "shell",
			"su -c 'cat /sys/devices/system/cpu/cpu0/cpufreq/scaling_available_governors 2>/dev/null'"})
		if avCode == 0 && strings.Contains(strings.ToLower(avail), "walt") {
			return "walt"
		}
	}
	return "schedutil"
}

func applyProfile(serial, profile string) error {
	fmt.Printf("\nApplying '%s' profile...\n", profile)

	switch profile {
	case "performance":
		cpuCount := applyGovernorLoop(serial, "performance")
		fmt.Printf("  Set governor to 'performance' on %d CPUs.\n", cpuCount)
		applied := applyIOScheduler(serial, "deadline")
		if len(applied) > 0 {
			fmt.Printf("  Set I/O scheduler to 'deadline' on: %s\n", strings.Join(applied, ", "))
		} else {
			fmt.Println("  [WARN] Could not apply I/O scheduler (block devices not writable).")
		}

	case "balanced":
		governor := detectBalancedGovernor(serial)
		cpuCount := applyGovernorLoop(serial, governor)
		fmt.Printf("  Set governor to '%s' on %d CPUs.\n", governor, cpuCount)
		applied := applyIOScheduler(serial, "cfq")
		if len(applied) == 0 {
			fmt.Println("  I/O scheduler unchanged (cfq not available on this kernel).")
		} else {
			fmt.Printf("  Set I/O scheduler to 'cfq' on: %s\n", strings.Join(applied, ", "))
		}

	case "powersave":
		cpuCount := applyGovernorLoop(serial, "powersave")
		fmt.Printf("  Set governor to 'powersave' on %d CPUs.\n", cpuCount)
		fmt.Println("  (I/O scheduler left unchanged for powersave.)")

	default:
		return nterrors.AdbError(fmt.Sprintf(
			"Unknown profile '%s'. Valid profiles: performance, balanced, powersave.", profile))
	}

	fmt.Println()
	fmt.Println("[WARN] Changes are NOT persistent \u2014 they will reset on reboot.")
	return nil
}

const menuText = `  Select profile:
    [0] Performance  (max clocks, deadline I/O)
    [1] Balanced     (schedutil, default)
    [2] Powersave    (min clocks)
    [3] Show current state only
`

var profileMap = map[string]string{
	"0": "performance",
	"1": "balanced",
	"2": "powersave",
}

// ActionPerformance manages CPU governor / I/O scheduler / thermal profile.
// profile="" shows current state and interactive menu.
// Valid profiles: performance, balanced, powersave.
// Requires Magisk root.
func ActionPerformance(serial, profile string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError(
			"Root not available via ADB shell.\n" +
				"Enable in Magisk: Settings -> Superuser access -> Apps and ADB.")
	}

	if profile == "" {
		printCurrentState(serial)
		fmt.Println()
		fmt.Print(menuText)

		choice, err := adb.Prompt("  Select [1]: ")
		if err != nil {
			return nil
		}
		if choice == "" {
			choice = "1"
		}
		if choice == "3" {
			return nil
		}
		mapped, ok := profileMap[choice]
		if !ok {
			fmt.Printf("[WARN] Invalid selection: %q. Aborted.\n", choice)
			return nil
		}
		profile = mapped
	}

	valid := map[string]bool{"performance": true, "balanced": true, "powersave": true}
	if !valid[profile] {
		return nterrors.AdbError(fmt.Sprintf(
			"Unknown profile '%s'. Valid options: balanced, performance, powersave", profile))
	}

	if err := applyProfile(serial, profile); err != nil {
		return err
	}
	fmt.Printf("[OK] Profile '%s' applied.\n", profile)
	return nil
}
