// Package magisk implements Magisk, KernelSU, and APatch root detection and
// management for Nothing Phone devices.
package magisk

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/models"
)

// HasRoot returns true if any root manager (Magisk, KernelSU, APatch) is
// active on the device and `su -c id` reports uid=0.
func HasRoot(serial string) bool {
	stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "su -c id"})
	return code == 0 && strings.Contains(stdout, "uid=0")
}

// DetectRootManager probes the device for known root managers and returns the
// first one found. Returns RootManagerNone if the device is not rooted.
func DetectRootManager(serial string) models.RootManager {
	// KernelSU: look for ksud binary or data directory.
	stdout, _, code := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"which ksud 2>/dev/null || ls /data/adb/ksud 2>/dev/null",
	})
	if code == 0 && strings.TrimSpace(stdout) != "" {
		return models.RootManagerKernelSU
	}

	// APatch: look for apd daemon binary.
	stdout, _, code = adb.Run([]string{
		"adb", "-s", serial, "shell", "which apd 2>/dev/null",
	})
	if code == 0 && strings.TrimSpace(stdout) != "" {
		return models.RootManagerAPatch
	}

	// Magisk: verify daemon is alive via `su -c 'magisk -V'`.
	stdout, _, code = adb.Run([]string{
		"adb", "-s", serial, "shell", "su -c 'magisk -V 2>/dev/null'",
	})
	if code == 0 && isDigit(strings.TrimSpace(stdout)) {
		return models.RootManagerMagisk
	}

	return models.RootManagerNone
}

// CheckKernelSU queries KernelSU version and manager-app installation status.
// Returns (installed, version, managerAppInstalled).
func CheckKernelSU(serial string) (installed bool, version string, managerApp bool) {
	stdout, _, code := adb.Run([]string{
		"adb", "-s", serial, "shell", "su -c 'ksud --version 2>/dev/null'",
	})
	if code == 0 && strings.TrimSpace(stdout) != "" {
		installed = true
		version = strings.TrimSpace(stdout)
	}

	stdout, _, _ = adb.Run([]string{
		"adb", "-s", serial, "shell", "pm list packages me.weishu.kernelsu",
	})
	managerApp = strings.Contains(stdout, "me.weishu.kernelsu")
	return
}

// CheckAPatch returns true when the APatch daemon (apd) is present on the device.
func CheckAPatch(serial string) bool {
	stdout, _, code := adb.Run([]string{
		"adb", "-s", serial, "shell", "which apd 2>/dev/null",
	})
	return code == 0 && strings.TrimSpace(stdout) != ""
}

// PrintRootStatus detects which root manager is active and prints a status
// summary to stdout.
func PrintRootStatus(serial string) {
	manager := DetectRootManager(serial)

	switch manager {
	case models.RootManagerMagisk:
		ms, err := CheckMagisk(serial)
		if err != nil {
			fmt.Printf("  Root manager : Magisk (status check failed: %v)\n", err)
			return
		}
		PrintMagiskStatus(ms)

	case models.RootManagerKernelSU:
		installed, version, managerApp := CheckKernelSU(serial)
		versionStr := "unknown"
		if installed && version != "" {
			versionStr = version
		}
		appStr := "not installed"
		if managerApp {
			appStr = "installed"
		}
		fmt.Println()
		fmt.Println("  Root manager : KernelSU")
		fmt.Printf("  Version      : %s\n", versionStr)
		fmt.Printf("  Manager app  : %s\n", appStr)

	case models.RootManagerAPatch:
		fmt.Println()
		fmt.Println("  Root manager : APatch")
		fmt.Println("  Status       : Active")

	default:
		fmt.Println("  Root : NOT ACTIVE")
	}
}

// isDigit returns true if s is a non-empty string of ASCII digits.
func isDigit(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
