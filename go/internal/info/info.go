// Package info provides a comprehensive device dashboard for Nothing phones.
package info

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

var socNames = map[string]string{
	// MediaTek
	"mt6886": "Dimensity 7200 Pro",
	"mt6878": "Dimensity 7300 Pro",
	"mt6893": "Dimensity 1200",
	"mt6983": "Dimensity 9000",
	// Qualcomm — SM model numbers
	"sm6375": "Snapdragon 778G+",
	"sm7325": "Snapdragon 778G",
	"sm7435": "Snapdragon 7s Gen 3",
	"sm8475": "Snapdragon 8+ Gen 1",
	"sm8550": "Snapdragon 8 Gen 2",
	"sm8650": "Snapdragon 8 Gen 3",
	// Qualcomm — codename platform strings
	"lahaina":   "Snapdragon 778G+",
	"taro":      "Snapdragon 8+ Gen 1",
	"kalama":    "Snapdragon 8 Gen 2",
	"pineapple": "Snapdragon 8 Gen 3",
}

func kbToGB(kb int64) string {
	return fmt.Sprintf("%.1f GB", float64(kb)/1024/1024)
}

func parseMeminfo(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var kb int64
				if _, err := fmt.Sscanf(parts[1], "%d", &kb); err == nil {
					return kbToGB(kb)
				}
			}
		}
	}
	return "not available"
}

func parseDf(output string) string {
	var lines []string
	for _, l := range strings.Split(output, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return "not available"
	}
	parts := strings.Fields(lines[len(lines)-1])
	if len(parts) < 4 {
		return "not available"
	}
	var total, used int64
	if _, err := fmt.Sscanf(parts[1], "%d", &total); err != nil {
		return "not available"
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &used); err != nil {
		return "not available"
	}
	return fmt.Sprintf("%s used of %s", kbToGB(used), kbToGB(total))
}

func resolveSOC(serial string) string {
	platform := strings.ToLower(adb.Prop(serial, "ro.board.platform"))
	board := strings.ToLower(adb.Prop(serial, "ro.product.board"))
	for _, raw := range []string{platform, board} {
		if name, ok := socNames[raw]; ok {
			return fmt.Sprintf("%s (%s)", raw, name)
		}
	}
	if platform != "" {
		return platform
	}
	if board != "" {
		return board
	}
	return "not available"
}

func imei(serial string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"service call iphonesubinfo 1 | grep -o \"'[^']*'\" | tr -d \"' \\n\""})
	candidate := strings.TrimSpace(stdout)
	var digits strings.Builder
	for _, c := range candidate {
		if c >= '0' && c <= '9' {
			digits.WriteRune(c)
		}
	}
	d := digits.String()
	if len(d) >= 14 {
		if len(d) > 15 {
			d = d[:15]
		}
		return d
	}
	sn := adb.Prop(serial, "ro.serialno")
	if sn != "" {
		return sn + "  (serial number, IMEI unavailable)"
	}
	return "not available"
}

// ActionInfo prints a comprehensive device dashboard for the connected Nothing phone.
func ActionInfo(serial string) error {
	model := adb.Prop(serial, "ro.product.model")
	if model == "" {
		model = "Unknown"
	}
	codename := adb.Prop(serial, "ro.product.device")
	if codename == "" {
		codename = adb.Prop(serial, "ro.build.product")
	}

	androidVer := adb.Prop(serial, "ro.build.version.release")
	if androidVer == "" {
		androidVer = "not available"
	}
	firmware := adb.Prop(serial, "ro.build.display.id")
	if firmware == "" {
		firmware = "not available"
	}
	securityPatch := adb.Prop(serial, "ro.build.version.security_patch")
	if securityPatch == "" {
		securityPatch = "not available"
	}
	kernel := adb.ShellStr(serial, "uname -r")
	if kernel == "" {
		kernel = "not available"
	}

	soc := resolveSOC(serial)

	meminfoRaw := adb.ShellStr(serial, "cat /proc/meminfo | grep MemTotal")
	ram := "not available"
	if meminfoRaw != "" {
		ram = parseMeminfo(meminfoRaw)
	}

	dfData := adb.ShellStr(serial, "df /data | tail -1")
	dfSdcard := adb.ShellStr(serial, "df /sdcard | tail -1")
	storageData := "not available"
	storageSdcard := "not available"
	if dfData != "" {
		storageData = parseDf(dfData)
	}
	if dfSdcard != "" {
		storageSdcard = parseDf(dfSdcard)
	}

	serialNum := adb.Prop(serial, "ro.serialno")
	if serialNum == "" {
		serialNum = "not available"
	}
	bootloader := adb.Prop(serial, "ro.bootloader")
	if bootloader == "" {
		bootloader = "not available"
	}
	imeiVal := imei(serial)

	adbMode := "USB"
	if strings.Contains(serial, ":") {
		adbMode = "Wireless (TCP/IP)"
	}

	currentSlot := adb.Prop(serial, "ro.boot.slot_suffix")
	if currentSlot == "" {
		currentSlot = "not available"
	} else {
		currentSlot = strings.TrimPrefix(currentSlot, "_")
	}

	lockStatus := "Run in fastboot mode to check (fastboot getvar unlocked)"

	sep := strings.Repeat("\u2500", 49)

	fmt.Printf("\n  Device Info \u2014 %s  [%s]\n\n", model, codename)

	fmt.Printf("  \u2500\u2500 Software %s\n", sep)
	fmt.Printf("  %-15s: %s\n", "Android", androidVer)
	fmt.Printf("  %-15s: %s\n", "Firmware", firmware)
	fmt.Printf("  %-15s: %s\n", "Security patch", securityPatch)
	fmt.Printf("  %-15s: %s\n", "Kernel", kernel)

	fmt.Printf("\n  \u2500\u2500 Hardware %s\n", sep)
	fmt.Printf("  %-15s: %s\n", "SoC", soc)
	fmt.Printf("  %-15s: %s\n", "RAM", ram)
	fmt.Printf("  %-15s: %s\n", "Storage /data", storageData)
	fmt.Printf("  %-15s: %s\n", "Storage /sdcard", storageSdcard)

	fmt.Printf("\n  \u2500\u2500 Identity %s\n", sep)
	fmt.Printf("  %-15s: %s\n", "Serial (ADB)", serialNum)
	fmt.Printf("  %-15s: %s\n", "Bootloader", bootloader)
	fmt.Printf("  %-15s: %s\n", "IMEI", imeiVal)

	fmt.Printf("\n  \u2500\u2500 Connection %s\n", sep)
	fmt.Printf("  %-15s: %s\n", "ADB mode", adbMode)
	fmt.Printf("  %-15s: %s\n", "Active slot", currentSlot)

	fmt.Printf("\n  \u2500\u2500 Bootloader %s\n", sep)
	fmt.Printf("  %-15s: %s\n", "Lock status", lockStatus)
	fmt.Println()
	return nil
}
