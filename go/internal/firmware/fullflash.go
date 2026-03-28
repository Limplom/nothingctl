package firmware

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

// BootPatchFunc is a function that patches a boot image for root preservation.
// It receives the ADB serial and the local path to the stock boot image, and
// returns the local path of the patched image. Pass nil to skip patching.
type BootPatchFunc func(serial, localImg string) (string, error)

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
	// 1. Get model and current firmware from device.
	model := adb.Prop(serial, "ro.product.model")
	currentVersion := adb.Prop(serial, "ro.build.display.id")

	// 2. Fall back to prop if codename is empty.
	if codename == "" {
		codename = adb.Prop(serial, "ro.product.device")
	}

	// 4. Print banner.
	fmt.Printf("\n  Full Flash — %s (%s)\n\n", model, codename)

	// 5. Fetch releases from nothing_archive.
	releases, err := FetchReleases(nothingArchiveOwner, nothingArchiveRepo)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub API: %w", err)
	}

	// Filter to releases for this codename.
	prefix := strings.ToLower(codename) + "_"
	var matched []map[string]any
	for _, r := range releases {
		tag, _ := r["tag_name"].(string)
		if strings.HasPrefix(strings.ToLower(tag), prefix) {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return fmt.Errorf("no releases found for codename '%s' in nothing_archive", codename)
	}

	// 6. Find latest release.
	latest := latestFromList(matched)
	latestTag, _ := latest["tag_name"].(string)

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
	if !adb.Confirm("This replaces ALL firmware partitions. Continue?") {
		return fmt.Errorf("cancelled by user")
	}

	// Extract destination directory: baseDir/<codename>/<tag>
	destDir := filepath.Join(baseDir, codename, latestTag)

	// Get assets from the release as []map[string]any.
	assets, _ := latest["assets"].([]any)
	assetMaps := make([]map[string]any, 0, len(assets))
	for _, a := range assets {
		if m, ok := a.(map[string]any); ok {
			assetMaps = append(assetMaps, m)
		}
	}

	// 13. Download boot archive (image-boot) via ResolveFirmware.
	fmt.Println("\nResolving boot images...")
	fw, err := ResolveFirmware(serial, codename, baseDir, forceDownload)
	if err != nil {
		return err
	}
	bootDir := fw.ExtractedDir
	bootTarget := fw.BootTarget

	// 14. Download firmware archive.
	fmt.Println("\nDownloading firmware archive...")
	firmwareExtractDir := destDir
	if err := DownloadFirmwareArchive(assetMaps, firmwareExtractDir, forceDownload); err != nil {
		return fmt.Errorf("firmware archive download failed: %w", err)
	}

	// 15. Download logical archive if not skipping.
	logicalExtractDir := destDir
	if !skipLogical {
		fmt.Println("\nDownloading logical partition archive (~4 GB)...")
		if err := DownloadLogicalArchive(assetMaps, logicalExtractDir, forceDownload); err != nil {
			return fmt.Errorf("logical archive download failed: %w", err)
		}
	}

	// 16. Magisk patch BEFORE fastboot (device still in ADB mode).
	// bootTarget.Filename is e.g. "init_boot.img"; strip the extension for ImgPath.
	bootTargetBase := strings.TrimSuffix(bootTarget.Filename, ".img")
	var patchedBootPath string
	if patchBoot != nil {
		bootImg := ImgPath(bootDir, bootTargetBase)
		patchedBootPath, err = patchBoot(serial, bootImg)
		if err != nil {
			return fmt.Errorf("Magisk patch failed: %w", err)
		}
		fmt.Printf("  Patched image: %s\n", filepath.Base(patchedBootPath))
	}

	// 17. Reboot to bootloader.
	if err := adb.RebootToBootloader(serial); err != nil {
		return err
	}

	// 18. Flash firmware partitions (skip any not exposed by this bootloader).
	fmt.Println("\n  Flashing firmware partitions...")
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
	bootImgToFlash := ImgPath(bootDir, bootTargetBase)
	if patchedBootPath != "" {
		bootImgToFlash = patchedBootPath
		fmt.Printf("\n  Flashing patched %s (root preserved)...\n", bootTarget.Filename)
	} else {
		fmt.Printf("\n  Flashing stock %s...\n", bootTarget.Filename)
	}
	if err := adb.FastbootFlashAB(serial, bootTarget.PartitionBase, bootImgToFlash); err != nil {
		return err
	}

	// 21. Flash logical partitions if not skipping.
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
	adb.FastbootReboot(serial)

	// 23. Print success summary.
	fmt.Printf("\n[OK] Full flash complete. Device is now on %s.\n", latestTag)
	if patchedBootPath != "" {
		fmt.Println("     Root (Magisk) preserved on both slots.")
	}
	return nil
}
