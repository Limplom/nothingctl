package firmware

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/models"
)

// BootPatchFunc is a function that patches a boot image for root preservation.
// It receives the ADB serial and the local path to the stock boot image, and
// returns the local path of the patched image. Pass nil to skip patching.
type BootPatchFunc func(serial, localImg string) (string, error)

// readDeviceProps fetches display model name and current firmware version from
// the connected device. If codename is empty it is also resolved from the device.
func readDeviceProps(serial, codename string) (model, currentVersion, resolvedCodename string) {
	model = adb.Prop(serial, "ro.product.model")
	currentVersion = adb.Prop(serial, "ro.build.display.id")

	// Fall back to prop if codename is empty.
	resolvedCodename = codename
	if resolvedCodename == "" {
		resolvedCodename = adb.Prop(serial, "ro.product.device")
	}
	return
}

// printFlashBanner prints the pre-flash summary and returns true if the user
// confirms they want to proceed.
func printFlashBanner(model, currentVersion, latestTag string, patchBoot BootPatchFunc, skipLogical bool) bool {
	// 7. Print current vs latest.
	fmt.Printf("  Current : %s\n", func() string {
		if currentVersion == "" {
			return "unknown"
		}
		return currentVersion
	}())
	fmt.Printf("  Latest  : %s\n", latestTag)

	// 8. Indicate patching mode.
	if patchBoot != nil {
		fmt.Println("  Root    : detected (boot will be patched with Magisk)")
	} else {
		fmt.Println("  Root    : not detected (stock boot will be flashed)")
	}

	// 10. Confirm size warning if !skipLogical.
	if !skipLogical {
		fmt.Println("\n  WARNING: ~4.2 GB download required for logical partitions.")
	}

	// 11. Confirmation prompt.
	return adb.Confirm("This replaces ALL firmware partitions. Continue?")
}

// downloadFlashArchives downloads the boot, firmware, and optional logical
// archives for the given release into baseDir. Returns the firmware/logical
// extract directory, the boot images directory, and the detected boot target.
func downloadFlashArchives(
	release map[string]any,
	serial, codename, baseDir, latestTag string,
	forceDownload, skipLogical bool,
) (destDir, bootDir string, bootTarget models.BootTarget, err error) {
	// Extract destination directory: baseDir/<codename>/<tag>
	destDir = filepath.Join(baseDir, codename, latestTag)

	// Get assets from the release as []map[string]any.
	assets, _ := release["assets"].([]any)
	assetMaps := make([]map[string]any, 0, len(assets))
	for _, a := range assets {
		if m, ok := a.(map[string]any); ok {
			assetMaps = append(assetMaps, m)
		}
	}

	// 13. Download boot archive (image-boot) via ResolveFirmware.
	fmt.Println("\nResolving boot images...")
	fw, fwErr := ResolveFirmware(serial, codename, baseDir, forceDownload)
	if fwErr != nil {
		err = fwErr
		return
	}
	bootDir = fw.ExtractedDir
	bootTarget = fw.BootTarget

	// 14. Download firmware archive.
	fmt.Println("\nDownloading firmware archive...")
	firmwareExtractDir := destDir
	if dlErr := DownloadFirmwareArchive(assetMaps, firmwareExtractDir, forceDownload); dlErr != nil {
		err = fmt.Errorf("firmware archive download failed: %w", dlErr)
		return
	}

	// 15. Download logical archive if not skipping.
	logicalExtractDir := destDir
	if !skipLogical {
		fmt.Println("\nDownloading logical partition archive (~4 GB)...")
		if dlErr := DownloadLogicalArchive(assetMaps, logicalExtractDir, forceDownload); dlErr != nil {
			err = fmt.Errorf("logical archive download failed: %w", dlErr)
			return
		}
	}
	return
}

// patchBootIfNeeded runs the boot patch function if provided, returning the
// path to the image that should be flashed (patched or stock).
func patchBootIfNeeded(serial, bootDir string, bootTarget models.BootTarget, patchBoot BootPatchFunc) (imgToFlash string, err error) {
	// bootTarget.Filename is e.g. "init_boot.img"; strip the extension for ImgPath.
	bootTargetBase := strings.TrimSuffix(bootTarget.Filename, ".img")
	var patchedBootPath string
	if patchBoot != nil {
		bootImg := ImgPath(bootDir, bootTargetBase)
		patchedBootPath, err = patchBoot(serial, bootImg)
		if err != nil {
			err = fmt.Errorf("Magisk patch failed: %w", err)
			return
		}
		fmt.Printf("  Patched image: %s\n", filepath.Base(patchedBootPath))
	}

	// Determine which image to flash.
	imgToFlash = ImgPath(bootDir, bootTargetBase)
	if patchedBootPath != "" {
		imgToFlash = patchedBootPath
	}
	return
}

// flashAllPartitions flashes all partitions from the extracted archives.
// The device must already be in ADB mode before calling (it will reboot to
// bootloader internally). Returns an error on any flash failure.
func flashAllPartitions(serial, destDir, bootDir string, bootTarget models.BootTarget, bootImgToFlash string, skipLogical bool) error {
	// 17. Reboot to bootloader.
	if err := adb.RebootToBootloader(serial); err != nil {
		return err
	}

	// 18. Flash firmware partitions (skip any not exposed by this bootloader).
	fmt.Println("\n  Flashing firmware partitions...")
	firmwareExtractDir := destDir
	for _, part := range ScanAvailableImages(firmwareExtractDir, firmwarePartitions) {
		fmt.Printf("    %s...", part)
		if err := adb.FastbootFlashAB(serial, part, ImgPath(firmwareExtractDir, part)); err != nil {
			if strings.Contains(err.Error(), "partition does not exist") {
				fmt.Println(" skipped (not exposed by bootloader)")
				continue
			}
			return err
		}
		fmt.Println(" OK")
	}

	// 19. Flash boot partitions (excluding the main boot target handled separately).
	fmt.Println("\n  Flashing boot partitions...")
	for _, part := range ScanAvailableImages(bootDir, bootPartitions) {
		// Skip the boot target partition here; it is handled with Magisk in step 20.
		if part == bootTarget.PartitionBase {
			continue
		}
		fmt.Printf("    %s...", part)
		if err := adb.FastbootFlashAB(serial, part, ImgPath(bootDir, part)); err != nil {
			return err
		}
		fmt.Println(" OK")
	}

	// 20. Flash init_boot (patched or stock).
	if bootImgToFlash == ImgPath(bootDir, strings.TrimSuffix(bootTarget.Filename, ".img")) {
		fmt.Printf("\n  Flashing stock %s...\n", bootTarget.Filename)
	} else {
		fmt.Printf("\n  Flashing patched %s (root preserved)...\n", bootTarget.Filename)
	}
	if err := adb.FastbootFlashAB(serial, bootTarget.PartitionBase, bootImgToFlash); err != nil {
		return err
	}

	// 21. Flash logical partitions if not skipping.
	logicalExtractDir := destDir
	if !skipLogical {
		if err := adb.RebootToFastbootd(serial); err != nil {
			return err
		}
		fmt.Println("\n  Flashing logical partitions (fastbootd)...")
		for _, part := range ScanAvailableImages(logicalExtractDir, logicalPartitions) {
			fmt.Printf("    %s...", part)
			if err := adb.FastbootFlash(serial, part, ImgPath(logicalExtractDir, part)); err != nil {
				return err
			}
			fmt.Println(" OK")
		}
		if err := adb.RebootToBootloaderFromFastbootd(serial); err != nil {
			return err
		}
	}

	// 22. Reboot to system.
	fmt.Println("\nRebooting to system...")
	if err := adb.FastbootReboot(serial); err != nil {
		fmt.Printf("  WARNING: reboot failed: %v\n", err)
	}
	return nil
}

// ActionFullFlash performs a complete firmware flash: downloads and flashes
// firmware partitions, boot partitions, optionally logical partitions, and
// preserves Magisk root if the device is rooted.
//
// serial:        ADB serial of the connected device
// codename:      device codename (e.g. "Galaxian")
// baseDir:       root storage directory (e.g. ~/.nothingctl)
// forceDownload: re-download even if cached archives exist
// skipLogical:   skip the ~4 GB logical partition download/flash
// patchBoot:     optional function to Magisk-patch the boot image; pass nil to flash stock
func ActionFullFlash(serial, codename, baseDir string, forceDownload, skipLogical bool, patchBoot BootPatchFunc) error {
	// 1–2. Get model, current firmware, and resolved codename from device.
	model, currentVersion, codename := readDeviceProps(serial, codename)

	// 4. Print banner.
	fmt.Printf("\n  Full Flash — %s (%s)\n\n", model, codename)

	// 5–6. Fetch latest release from nothing_archive.
	latest, latestTag, err := FetchLatestRelease(codename)
	if err != nil {
		return err
	}

	if !printFlashBanner(model, currentVersion, latestTag, patchBoot, skipLogical) {
		return fmt.Errorf("cancelled by user")
	}

	destDir, bootDir, bootTarget, err := downloadFlashArchives(latest, serial, codename, baseDir, latestTag, forceDownload, skipLogical)
	if err != nil {
		return err
	}

	bootImgToFlash, err := patchBootIfNeeded(serial, bootDir, bootTarget, patchBoot)
	if err != nil {
		return err
	}

	if err := flashAllPartitions(serial, destDir, bootDir, bootTarget, bootImgToFlash, skipLogical); err != nil {
		return err
	}

	// 23. Print success summary.
	fmt.Printf("\n[OK] Full flash complete. Device is now on %s.\n", latestTag)
	if patchBoot != nil {
		fmt.Println("     Root (Magisk) preserved on both slots.")
	}
	return nil
}
