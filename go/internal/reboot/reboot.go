// Package reboot provides reboot target selection for Nothing phones.
package reboot

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const menu = `  Reboot target:
    [0] System (normal reboot)
    [1] Bootloader / Fastboot
    [2] Recovery
    [3] Safe mode
    [4] Download mode  (MediaTek only)
    [5] ADB Sideload
`

var targetMap = map[string]string{
	"0": "system",
	"1": "bootloader",
	"2": "recovery",
	"3": "safe",
	"4": "download",
	"5": "sideload",
}

func isMediatek(serial string) bool {
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.board.platform"})
	return strings.Contains(strings.ToLower(strings.TrimSpace(out)), "mt")
}

// ActionReboot reboots the device to the specified target.
// target="" shows an interactive menu.
// Valid targets: system, bootloader, recovery, safe, download, sideload.
func ActionReboot(serial, target string) error {
	if target == "" {
		fmt.Print(menu)
		choice, err := adb.Prompt("  Select [0]: ")
		if err != nil {
			fmt.Println("\nAborted.")
			return nil
		}
		if choice == "" {
			choice = "0"
		}
		mapped, ok := targetMap[choice]
		if !ok {
			return nterrors.AdbError(fmt.Sprintf("Invalid selection: %q. Choose 0\u20135.", choice))
		}
		target = mapped
	}

	target = strings.ToLower(target)

	switch target {
	case "system":
		fmt.Println("Rebooting to system...")
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "reboot"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Reboot failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Println("[OK] Reboot command sent.")

	case "bootloader":
		fmt.Println("Rebooting to bootloader...")
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "reboot", "bootloader"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Reboot to bootloader failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Println("[OK] Reboot command sent.")

	case "recovery":
		fmt.Println("Rebooting to recovery...")
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "reboot", "recovery"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Reboot to recovery failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Println("[OK] Reboot command sent.")

	case "safe":
		fmt.Println("Rebooting to safe mode...")
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"setprop persist.sys.safemode 1 && reboot"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Reboot to safe mode failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Println("[OK] Reboot command sent.")
		fmt.Println("[WARN] Safe mode disables itself automatically after the next reboot.")

	case "download":
		if isMediatek(serial) {
			fmt.Println("Rebooting to download mode (MediaTek)...")
			_, stderr, code := adb.Run([]string{"adb", "-s", serial, "reboot", "download"})
			if code != 0 {
				return nterrors.AdbError(fmt.Sprintf("Reboot to download mode failed: %s", strings.TrimSpace(stderr)))
			}
			fmt.Println("[OK] Reboot command sent.")
		} else {
			fmt.Println("[WARN] Download mode is only natively supported on MediaTek devices.")
			fmt.Println("[WARN] For Qualcomm devices, use EDL (Emergency Download) mode instead:")
			fmt.Println("         Power off the device, then hold Volume Down + connect USB.")
		}

	case "sideload":
		fmt.Println("Rebooting to ADB sideload mode...")
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "reboot", "sideload"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Reboot to sideload failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Println("[OK] Reboot command sent.")

	default:
		return nterrors.AdbError(fmt.Sprintf(
			"Unknown reboot target: %q. Valid targets: system, bootloader, recovery, safe, download, sideload.",
			target))
	}
	return nil
}
