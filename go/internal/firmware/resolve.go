package firmware

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"github.com/Limplom/nothingctl/internal/models"
)

const (
	nothingArchiveOwner = "spike0en"
	nothingArchiveRepo  = "nothing_archive"
	sdcardDownload      = "/sdcard/Download"
)

// ResolveFirmware checks the nothing_archive for the latest firmware for the
// given codename, downloads and extracts it if needed, and returns a populated
// FirmwareState with the path to the extracted directory.
//
// baseDir is the root storage directory (e.g. ~/.nothingctl). Firmware is
// cached under baseDir/<codename>/<tag>/. forceDownload re-downloads even when
// a cached copy exists.
func ResolveFirmware(serial, codename, baseDir string, forceDownload bool) (*models.FirmwareState, error) {
	// Read current build version from device.
	currentVersion, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell", "getprop ro.build.display.id",
	})
	currentVersion = strings.TrimSpace(currentVersion)

	fmt.Printf("  Codename: %s\n", codename)
	fmt.Printf("  Current : %s\n", func() string {
		if currentVersion == "" {
			return "unknown"
		}
		return currentVersion
	}())

	fmt.Println("\nChecking nothing_archive...")

	releases, err := FetchReleases(nothingArchiveOwner, nothingArchiveRepo)
	if err != nil {
		return nil, nterrors.FirmwareError("cannot reach GitHub API: " + err.Error())
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
		return nil, nterrors.FirmwareError(
			fmt.Sprintf("no releases found for codename '%s'.", codename),
		)
	}

	latest := latestFromList(matched)
	latestTag, _ := latest["tag_name"].(string)
	fmt.Printf("  Latest  : %s\n", latestTag)

	// Determine whether an update is available.
	var currentTag string
	if currentVersion != "" {
		// Capitalize first letter to match Nothing Archive tag convention.
		upperCodename := strings.ToUpper(codename[:1]) + codename[1:]
		currentTag = upperCodename + "_" + currentVersion
	}
	isNewer := currentTag != latestTag
	if isNewer {
		fmt.Println("  Status  : UPDATE AVAILABLE")
	} else {
		fmt.Println("  Status  : up to date")
	}

	destDir := filepath.Join(baseDir, codename, latestTag)
	initBootPath := filepath.Join(destDir, "init_boot.img")
	bootPath := filepath.Join(destDir, "boot.img")

	_, errInit := os.Stat(initBootPath)
	_, errBoot := os.Stat(bootPath)
	cached := errInit == nil || errBoot == nil

	if cached && !forceDownload {
		fmt.Printf("  Cached  : %s\n", destDir)
	} else {
		assetName, assetURL, found := FindAsset(latest, "-image-boot.7z")
		if !found {
			return nil, nterrors.FirmwareError("no image-boot.7z asset in release.")
		}

		// Determine size for human-readable output.
		var sizeMB int
		assets, _ := latest["assets"].([]any)
		for _, a := range assets {
			asset, _ := a.(map[string]any)
			name, _ := asset["name"].(string)
			if name == assetName {
				size, _ := asset["size"].(float64)
				sizeMB = int(size) / 1024 / 1024
			}
		}
		fmt.Printf("\nDownloading %s (%d MB)...\n", assetName, sizeMB)

		archivePath := filepath.Join(destDir, assetName)
		if err := DownloadFile(assetURL, archivePath); err != nil {
			return nil, err
		}

		fmt.Println("Extracting...")
		if err := Extract7z(archivePath, destDir); err != nil {
			return nil, err
		}
		// Remove the archive after extraction to save space.
		os.Remove(archivePath)
	}

	bootTarget, err := DetectBootTarget(destDir)
	if err != nil {
		return nil, err
	}

	isGKI2 := bootTarget == "init_boot.img"
	partBase := "boot"
	if isGKI2 {
		partBase = "init_boot"
	}

	return &models.FirmwareState{
		ExtractedDir: destDir,
		Version:      latestTag,
		IsNewer:      isNewer,
		BootTarget: models.BootTarget{
			Filename:      bootTarget,
			PartitionBase: partBase,
			IsGKI2:        isGKI2,
		},
	}, nil
}

// findAssetBySuffix searches a slice of GitHub asset objects (each a
// map[string]any with "name" and "browser_download_url" keys) for the first
// asset whose name ends with suffix. Returns the download URL, asset name, and
// an error if not found.
func findAssetBySuffix(assets []map[string]any, suffix string) (url, name string, err error) {
	for _, asset := range assets {
		assetName, _ := asset["name"].(string)
		if strings.HasSuffix(assetName, suffix) {
			dl, _ := asset["browser_download_url"].(string)
			return dl, assetName, nil
		}
	}
	return "", "", fmt.Errorf("no asset with suffix %q found in release", suffix)
}

// DownloadFirmwareArchive downloads and extracts the image-firmware.7z asset
// for the given release into destDir. Skips if modem.img already exists.
func DownloadFirmwareArchive(assets []map[string]any, destDir string, force bool) error {
	// Check if already extracted
	if !force {
		if _, err := os.Stat(filepath.Join(destDir, "modem.img")); err == nil {
			fmt.Println("  Firmware archive already extracted.")
			return nil
		}
	}
	// Find the asset
	url, name, err := findAssetBySuffix(assets, "-image-firmware.7z")
	if err != nil {
		return err
	}
	archivePath := filepath.Join(destDir, name)
	if err := DownloadFile(url, archivePath); err != nil {
		return err
	}
	fmt.Printf("Extracting %s...\n", name)
	if err := Extract7z(archivePath, destDir); err != nil {
		return err
	}
	os.Remove(archivePath)
	return nil
}

// DownloadLogicalArchive downloads all image-logical.7z.001/.002/... parts
// and extracts them into destDir. Skips if system.img already exists.
func DownloadLogicalArchive(assets []map[string]any, destDir string, force bool) error {
	// Check if already extracted
	if !force {
		if _, err := os.Stat(filepath.Join(destDir, "system.img")); err == nil {
			fmt.Println("  Logical archive already extracted.")
			return nil
		}
	}
	// Collect all parts in order
	var parts []struct{ url, name string }
	for i := 1; ; i++ {
		suffix := fmt.Sprintf("-image-logical.7z.%03d", i)
		url, name, err := findAssetBySuffix(assets, suffix)
		if err != nil {
			break // no more parts
		}
		parts = append(parts, struct{ url, name string }{url, name})
	}
	if len(parts) == 0 {
		return fmt.Errorf("no logical partition archive found in release assets")
	}
	fmt.Printf("  Logical archive: %d parts to download (~4 GB)...\n", len(parts))
	for _, p := range parts {
		archivePath := filepath.Join(destDir, p.name)
		if _, err := os.Stat(archivePath); err == nil && !force {
			fmt.Printf("  %s already downloaded.\n", p.name)
			continue
		}
		if err := DownloadFile(p.url, archivePath); err != nil {
			return err
		}
	}
	// Extract from the .001 part (7z handles multi-part automatically)
	part001 := filepath.Join(destDir, parts[0].name)
	fmt.Printf("Extracting logical partitions (this may take several minutes)...\n")
	if err := Extract7z(part001, destDir); err != nil {
		return err
	}
	// Delete downloaded parts to reclaim disk space
	for _, p := range parts {
		os.Remove(filepath.Join(destDir, p.name))
	}
	return nil
}

// FindMagiskPatched finds the most-recently-created magisk_patched*.img on the
// device's sdcard download folder. Returns the device path or an error if none
// is found.
func FindMagiskPatched(serial string) (string, error) {
	stdout, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"ls -t " + sdcardDownload + "/magisk_patched*.img",
	})
	stdout = strings.TrimSpace(stdout)
	lines := strings.SplitN(stdout, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if first == "" || strings.Contains(first, "No such") {
		return "", nterrors.FirmwareError(
			"no magisk_patched*.img found in " + sdcardDownload + ".\n" +
				"Patch the image in the Magisk app first.",
		)
	}
	return first, nil
}
