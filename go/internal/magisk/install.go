package magisk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"github.com/Limplom/nothingctl/internal/firmware"
)

const sdcardDownload = "/sdcard/Download"

// ActionInstallMagisk downloads the latest Magisk APK and installs it on the
// device. If Magisk is already installed this acts as an update.
func ActionInstallMagisk(serial, baseDir string) error {
	ms, err := CheckMagisk(serial)
	if err != nil {
		return err
	}

	if ms.LatestApkURL == nil || *ms.LatestApkURL == "" {
		return nterrors.MagiskError(
			"could not fetch latest Magisk release from GitHub.\n" +
				"Check internet connection or install manually from " +
				"https://github.com/topjohnwu/Magisk/releases",
		)
	}

	action := "Install"
	if ms.AppInstalled {
		action = "Update"
	}

	latestStr := ""
	if ms.LatestVersionStr != nil {
		latestStr = *ms.LatestVersionStr
	}
	fmt.Printf("\n%s Magisk v%s\n", action, latestStr)

	if !ms.AppInstalled {
		fmt.Println("\nFeatures enabled after installation + patching boot image:")
		fmt.Println("  --backup, auto-backup, performance tweaks, system cert install, app data access")
	}

	if !adb.Confirm(action + " Magisk APK on device?") {
		return nterrors.MagiskError("cancelled by user")
	}

	apkURL := *ms.LatestApkURL
	parts := strings.Split(apkURL, "/")
	apkName := parts[len(parts)-1]
	apkPath := filepath.Join(baseDir, apkName)

	if _, err := os.Stat(apkPath); err != nil {
		fmt.Printf("Downloading %s...\n", apkName)
		if err := firmware.DownloadFile(apkURL, apkPath); err != nil {
			return err
		}
	}

	fmt.Printf("Installing %s...\n", apkName)
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "install", "-r", apkPath})
	if code != 0 {
		return nterrors.MagiskError("adb install failed: " + strings.TrimSpace(stderr))
	}

	fmt.Printf("[OK] Magisk %sd.\n", action)

	if !ms.RootActive {
		fmt.Println("\nNext steps to activate root:")
		fmt.Println("  1. nothingctl push-for-patch   (push boot image to device)")
		fmt.Println("  2. Open Magisk app -> Install -> Patch an Image -> select the file")
		fmt.Println("  3. nothingctl flash-patched    (flash patched image)")
	} else {
		fmt.Println("\nOpen Magisk and tap 'Update' if prompted to update the daemon.")
	}
	return nil
}

// ActionUpdateMagisk is an alias for ActionInstallMagisk — it downloads and
// installs the latest Magisk APK regardless of current install state.
func ActionUpdateMagisk(serial, baseDir string) error {
	return ActionInstallMagisk(serial, baseDir)
}

// ActionUnroot flashes the stock boot image back to both A/B slots, removing
// root. The firmware directory must be pre-resolved by the caller.
func ActionUnroot(serial, firmwareDir string) error {
	bootTarget, err := firmware.DetectBootTarget(firmwareDir)
	if err != nil {
		return err
	}

	imgPath := filepath.Join(firmwareDir, bootTarget)
	partBase := strings.TrimSuffix(bootTarget, ".img")

	fmt.Printf("\nWill flash STOCK %s to both slots (removes root).\n", bootTarget)
	if !adb.Confirm("Proceed?") {
		return nterrors.MagiskError("cancelled by user")
	}

	if err := adb.RebootToBootloader(serial); err != nil {
		return err
	}
	if err := adb.FastbootFlashAB(serial, partBase, imgPath); err != nil {
		return err
	}
	if err := adb.FastbootReboot(serial); err != nil {
		return err
	}
	fmt.Println("[OK] Device is now unrooted. OTA update can proceed.")
	return nil
}

// ActionPushForPatch pushes the stock boot/init_boot image from the extracted
// firmware directory to /sdcard/Download/ on the device for manual patching
// inside the Magisk app.
func ActionPushForPatch(serial, firmwareDir string) error {
	bootTarget, err := firmware.DetectBootTarget(firmwareDir)
	if err != nil {
		return err
	}

	localImg := filepath.Join(firmwareDir, bootTarget)
	remote := sdcardDownload + "/" + bootTarget

	fmt.Printf("\nPushing %s to %s...\n", bootTarget, remote)
	if err := adb.AdbPush(serial, localImg, remote); err != nil {
		return err
	}

	// Trigger media scanner so the file picker sees the new file.
	adb.Run([]string{
		"adb", "-s", serial, "shell",
		"am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE -d file://" + remote,
	})

	fmt.Println("[OK] File ready on device.")
	fmt.Printf("\nNow open Magisk -> Install -> Patch an Image -> Downloads -> %s\n", bootTarget)
	fmt.Println("Then run: nothingctl flash-patched")
	return nil
}

// ActionFlashPatched pulls the most-recently-created magisk_patched*.img from
// the device's sdcard, then flashes it to both A/B slots via fastboot.
//
// firmwareDir is used to determine the correct partition base name.
func ActionFlashPatched(serial, firmwareDir string) error {
	bootTarget, err := firmware.DetectBootTarget(firmwareDir)
	if err != nil {
		return err
	}
	partBase := strings.TrimSuffix(bootTarget, ".img")

	fmt.Println("\nLooking for magisk_patched*.img on device...")

	remote, err := firmware.FindMagiskPatched(serial)
	if err != nil {
		return err
	}

	parts := strings.Split(remote, "/")
	filename := parts[len(parts)-1]
	localPath := filepath.Join(firmwareDir, filename)

	fmt.Printf("  Found: %s\n", filename)
	fmt.Printf("Pulling to: %s\n", localPath)
	if err := adb.AdbPull(serial, remote, localPath); err != nil {
		return err
	}

	fmt.Printf("\nWill flash patched %s to BOTH slots.\n", partBase)
	if !adb.Confirm("Reboot to bootloader and flash?") {
		return nterrors.MagiskError("cancelled by user")
	}

	if err := adb.RebootToBootloader(serial); err != nil {
		return err
	}
	if err := adb.FastbootFlashAB(serial, partBase, localPath); err != nil {
		return err
	}
	if _, err := adb.FastbootRun(serial, []string{"reboot"}); err != nil {
		return err
	}
	fmt.Println("[OK] Patched image flashed. Device is rooted on both slots.")
	return nil
}

// ActionFixBiometric forces strong authentication (PIN/password) instead of
// fingerprint for the current lock session.
func ActionFixBiometric(serial string) error {
	_, stderr, code := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"locksettings require-strong-auth STRONG_AUTH_REQUIRED_AFTER_USER_LOCKDOWN",
	})
	if code != 0 {
		return nterrors.AdbError("locksettings failed: " + strings.TrimSpace(stderr))
	}
	fmt.Println("[OK] Strong auth enforced — PIN/password will be used instead of fingerprint.")
	fmt.Println("     Effect lasts until next reboot. Run again if needed after restart.")
	return nil
}
