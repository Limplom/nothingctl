package adb

import (
	"context"
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

var currentSlotRe = regexp.MustCompile(`current-slot:\s*([ab])`)

// ---------------------------------------------------------------------------
// Fastboot helpers
// ---------------------------------------------------------------------------

// FastbootRunCtx runs `fastboot -s <serial> <args...>` and returns combined
// stdout+stderr. Returns a FlashError on non-zero exit.
// The context can be used to cancel the underlying process.
func FastbootRunCtx(ctx context.Context, serial string, args []string) (string, error) {
	cmdArgs := append([]string{"fastboot", "-s", serial}, args...)
	stdout, stderr, code := RunCtx(ctx, cmdArgs)
	combined := stdout + stderr
	if code != 0 {
		return combined, nterrors.FlashError(
			fmt.Sprintf("fastboot %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr)),
		)
	}
	return combined, nil
}

// FastbootRun runs `fastboot -s <serial> <args...>` and returns combined
// stdout+stderr. Returns a FlashError on non-zero exit.
func FastbootRun(serial string, args []string) (string, error) {
	return FastbootRunCtx(context.Background(), serial, args)
}

// FastbootFlashCtx flashes a single partition image.
// The context can be used to cancel the underlying fastboot process.
func FastbootFlashCtx(ctx context.Context, serial, partition, imgPath string) error {
	fmt.Printf("  Flashing %-20s <- %s\n", partition, imgPath)
	_, err := FastbootRunCtx(ctx, serial, []string{"flash", partition, imgPath})
	return err
}

// FastbootFlash flashes a single partition image.
func FastbootFlash(serial, partition, imgPath string) error {
	return FastbootFlashCtx(context.Background(), serial, partition, imgPath)
}

// FastbootFlashABCtx flashes both _a and _b slots for a base partition name.
// The context can be used to cancel the underlying fastboot processes.
func FastbootFlashABCtx(ctx context.Context, serial, partition, imgPath string) error {
	if err := FastbootFlashCtx(ctx, serial, partition+"_a", imgPath); err != nil {
		return err
	}
	return FastbootFlashCtx(ctx, serial, partition+"_b", imgPath)
}

// FastbootFlashAB flashes both _a and _b slots for a base partition name.
func FastbootFlashAB(serial, partition, imgPath string) error {
	return FastbootFlashABCtx(context.Background(), serial, partition, imgPath)
}

// WaitForFastbootCtx polls `fastboot devices` until the device with the given
// serial appears, the timeout elapses, or ctx is cancelled.
func WaitForFastbootCtx(ctx context.Context, serial string, timeoutSec int) error {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeoutSec <= 0 {
		timeout = fastbootPollTimeout
	}

	fmt.Print("Waiting for fastboot device")
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(fastbootPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			fmt.Println()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nterrors.FastbootTimeout(
				"fastboot device not found after timeout.\n" +
					"Check: USB cable and fastboot driver.\n" +
					"  Windows : install WinUSB driver via Zadig (zadig.akeo.ie)\n" +
					"  macOS   : brew install android-platform-tools\n" +
					"  Linux   : add udev rules or run as root",
			)
		case <-ticker.C:
			stdout, _, _ := Run([]string{"fastboot", "devices"})
			if (serial != "" && strings.Contains(stdout, serial)) ||
				(strings.TrimSpace(stdout) != "" && strings.Contains(stdout, "fastboot")) {
				fmt.Println(" OK")
				return nil
			}
			fmt.Print(".")
		}
	}
}

// WaitForFastboot polls `fastboot devices` until the device with the given
// serial appears, or until timeoutSec seconds have elapsed.
func WaitForFastboot(serial string, timeoutSec int) error {
	return WaitForFastbootCtx(context.Background(), serial, timeoutSec)
}

// QueryCurrentSlot returns the active A/B slot suffix ("_a" or "_b") reported
// by fastboot. Returns "unknown" if the variable cannot be parsed.
func QueryCurrentSlot(serial string) (string, error) {
	cmdArgs := []string{"fastboot", "-s", serial, "getvar", "current-slot"}
	stdout, stderr, _ := Run(cmdArgs)
	combined := stdout + stderr
	m := currentSlotRe.FindStringSubmatch(combined)
	if m == nil {
		return "unknown", nil
	}
	return "_" + m[1], nil
}

// RebootToBootloaderCtx reboots the device into fastboot mode and waits for it
// to appear. ctx is forwarded to WaitForFastbootCtx so the caller can cancel.
func RebootToBootloaderCtx(ctx context.Context, serial string) error {
	fmt.Println("Rebooting to bootloader...")
	Run([]string{"adb", "-s", serial, "reboot", "bootloader"})
	return WaitForFastbootCtx(ctx, serial, int(fastbootPollTimeout.Seconds()))
}

// RebootToBootloader reboots the device into fastboot mode and waits for it
// to appear.
func RebootToBootloader(serial string) error {
	return RebootToBootloaderCtx(context.Background(), serial)
}

// FastbootRebootCtx reboots the device from fastboot mode back to Android.
// The context can be used to cancel the underlying fastboot process.
func FastbootRebootCtx(ctx context.Context, serial string) error {
	_, err := FastbootRunCtx(ctx, serial, []string{"reboot"})
	return err
}

// FastbootReboot reboots the device from fastboot mode back to Android.
func FastbootReboot(serial string) error {
	return FastbootRebootCtx(context.Background(), serial)
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

// WaitForFastbootdCtx polls until the device enters userspace fastboot (fastbootd),
// the timeout elapses, or ctx is cancelled.
// It checks `fastboot getvar is-userspace` for the value "yes".
func WaitForFastbootdCtx(ctx context.Context, serial string, timeoutSec int) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("fastbootd device not found after %d seconds", timeoutSec)
		case <-ticker.C:
			args := []string{"fastboot"}
			if serial != "" {
				args = append(args, "-s", serial)
			}
			args = append(args, "getvar", "is-userspace")
			stdout, stderr, _ := Run(args)
			combined := stdout + stderr
			if strings.Contains(combined, "is-userspace: yes") {
				return nil
			}
			fmt.Print(".")
		}
	}
}

// WaitForFastbootd polls until the device enters userspace fastboot (fastbootd).
// It checks `fastboot getvar is-userspace` for the value "yes".
// Returns an error after timeoutSec seconds.
func WaitForFastbootd(serial string, timeoutSec int) error {
	return WaitForFastbootdCtx(context.Background(), serial, timeoutSec)
}

// RebootToFastbootdCtx runs `fastboot reboot fastboot` then waits for the
// device to enter userspace fastboot (fastbootd). ctx is forwarded to
// WaitForFastbootdCtx so the caller can cancel.
func RebootToFastbootdCtx(ctx context.Context, serial string) error {
	fmt.Println("Rebooting to fastbootd (userspace fastboot)...")
	args := []string{"fastboot"}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "reboot", "fastboot")
	Run(args) // ignore exit code — device may disconnect before returning
	return WaitForFastbootdCtx(ctx, serial, 90)
}

// RebootToFastbootd runs `fastboot reboot fastboot` then waits for the device
// to enter userspace fastboot (fastbootd).
func RebootToFastbootd(serial string) error {
	return RebootToFastbootdCtx(context.Background(), serial)
}

// RebootToBootloaderFromFastbootdCtx runs `fastboot reboot bootloader` from
// fastbootd mode, then waits for the device to appear in regular fastboot.
// ctx is forwarded to WaitForFastbootCtx so the caller can cancel.
func RebootToBootloaderFromFastbootdCtx(ctx context.Context, serial string) error {
	fmt.Println("Rebooting to bootloader...")
	args := []string{"fastboot"}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "reboot", "bootloader")
	Run(args)
	return WaitForFastbootCtx(ctx, serial, 60)
}

// RebootToBootloaderFromFastbootd runs `fastboot reboot bootloader` from
// fastbootd mode, then waits for the device to appear in regular fastboot.
func RebootToBootloaderFromFastbootd(serial string) error {
	return RebootToBootloaderFromFastbootdCtx(context.Background(), serial)
}
