package adb

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const (
	fastbootPollInterval = 2 * time.Second
	fastbootPollTimeout  = 40 * time.Second
)

// ---------------------------------------------------------------------------
// Fastboot helpers
// ---------------------------------------------------------------------------

// FastbootRun runs `fastboot -s <serial> <args...>` and returns combined
// stdout+stderr. Returns a FlashError on non-zero exit.
func FastbootRun(serial string, args []string) (string, error) {
	cmdArgs := append([]string{"fastboot", "-s", serial}, args...)
	stdout, stderr, code := Run(cmdArgs)
	combined := stdout + stderr
	if code != 0 {
		return combined, nterrors.FlashError(
			fmt.Sprintf("fastboot %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr)),
		)
	}
	return combined, nil
}

// FastbootFlash flashes a single partition image.
func FastbootFlash(serial, partition, imgPath string) error {
	fmt.Printf("  Flashing %-20s <- %s\n", partition, imgPath)
	_, err := FastbootRun(serial, []string{"flash", partition, imgPath})
	return err
}

// FastbootFlashAB flashes both _a and _b slots for a base partition name.
func FastbootFlashAB(serial, partition, imgPath string) error {
	if err := FastbootFlash(serial, partition+"_a", imgPath); err != nil {
		return err
	}
	return FastbootFlash(serial, partition+"_b", imgPath)
}

// WaitForFastboot polls `fastboot devices` until the device with the given
// serial appears, or until timeoutSec seconds have elapsed.
func WaitForFastboot(serial string, timeoutSec int) error {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeoutSec <= 0 {
		timeout = fastbootPollTimeout
	}

	fmt.Print("Waiting for fastboot device")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stdout, _, _ := Run([]string{"fastboot", "devices"})
		if (serial != "" && strings.Contains(stdout, serial)) ||
			(strings.TrimSpace(stdout) != "" && strings.Contains(stdout, "fastboot")) {
			fmt.Println(" OK")
			return nil
		}
		fmt.Print(".")
		time.Sleep(fastbootPollInterval)
	}
	fmt.Println()
	return nterrors.FastbootTimeout(
		"fastboot device not found after timeout.\n" +
			"Check: USB cable and fastboot driver.\n" +
			"  Windows : install WinUSB driver via Zadig (zadig.akeo.ie)\n" +
			"  macOS   : brew install android-platform-tools\n" +
			"  Linux   : add udev rules or run as root",
	)
}

// QueryCurrentSlot returns the active A/B slot suffix ("_a" or "_b") reported
// by fastboot. Returns "unknown" if the variable cannot be parsed.
func QueryCurrentSlot(serial string) (string, error) {
	cmdArgs := []string{"fastboot", "-s", serial, "getvar", "current-slot"}
	stdout, stderr, _ := Run(cmdArgs)
	combined := stdout + stderr
	re := regexp.MustCompile(`current-slot:\s*([ab])`)
	m := re.FindStringSubmatch(combined)
	if m == nil {
		return "unknown", nil
	}
	return "_" + m[1], nil
}

// RebootToBootloader reboots the device into fastboot mode and waits for it
// to appear.
func RebootToBootloader(serial string) error {
	fmt.Println("Rebooting to bootloader...")
	Run([]string{"adb", "-s", serial, "reboot", "bootloader"})
	return WaitForFastboot(serial, int(fastbootPollTimeout.Seconds()))
}

// FastbootReboot reboots the device from fastboot mode back to Android.
func FastbootReboot(serial string) error {
	_, err := FastbootRun(serial, []string{"reboot"})
	return err
}

// FastbootGetVar queries a fastboot variable and returns its value string.
func FastbootGetVar(serial, variable string) (string, error) {
	cmdArgs := []string{"fastboot", "-s", serial, "getvar", variable}
	stdout, stderr, code := Run(cmdArgs)
	combined := stdout + stderr
	if code != 0 {
		return "", nterrors.FlashError(
			fmt.Sprintf("fastboot getvar %s failed: %s", variable, strings.TrimSpace(stderr)),
		)
	}
	// fastboot prints variable output to stderr; search both buffers.
	re := regexp.MustCompile(regexp.QuoteMeta(variable) + `:\s*(\S+)`)
	m := re.FindStringSubmatch(combined)
	if m != nil {
		return m[1], nil
	}
	return strings.TrimSpace(stdout), nil
}
