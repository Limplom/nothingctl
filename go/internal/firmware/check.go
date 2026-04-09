package firmware

import (
	"fmt"
	"os"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// CheckUpdate queries nothing_archive for the latest firmware release for the
// device attached to serial and prints a status summary. It never downloads
// anything — use ResolveFirmware for that.
func CheckUpdate(serial, codename string) error {
	currentVersion, stderr, exitCode := adb.Run([]string{
		"adb", "-s", serial, "shell", "getprop ro.build.display.id",
	})
	currentVersion = strings.TrimSpace(currentVersion)
	if exitCode != 0 || currentVersion == "" {
		fmt.Fprintf(os.Stderr, "WARNING: could not read device firmware version (exit %d: %s)\n",
			exitCode, strings.TrimSpace(stderr))
		fmt.Println("         Version comparison will be skipped — update check continues.")
	}

	fmt.Printf("Device    : %s\n", codename)
	fmt.Printf("Firmware  : %s\n", func() string {
		if currentVersion == "" {
			return "unknown"
		}
		return currentVersion
	}())

	fmt.Println("\nChecking nothing_archive for updates...")

	releases, err := FetchReleases(nothingArchiveOwner, nothingArchiveRepo)
	if err != nil {
		return nterrors.FirmwareError("cannot reach GitHub API: " + err.Error())
	}

	prefix := strings.ToLower(codename) + "_"
	var matched []map[string]any
	for _, r := range releases {
		tag, _ := r["tag_name"].(string)
		if strings.HasPrefix(strings.ToLower(tag), prefix) {
			matched = append(matched, r)
		}
	}
	if len(matched) == 0 {
		return nterrors.FirmwareError(
			fmt.Sprintf("no releases found for codename '%s' in nothing_archive.", codename),
		)
	}

	latest := latestFromList(matched)
	latestTag, _ := latest["tag_name"].(string)
	htmlURL, _ := latest["html_url"].(string)

	fmt.Printf("Latest    : %s\n", latestTag)

	var currentTag string
	if currentVersion != "" {
		upper := strings.ToUpper(codename[:1]) + codename[1:]
		currentTag = upper + "_" + currentVersion
	}

	isNewer := currentTag != latestTag
	if isNewer {
		fmt.Println("Status    : UPDATE AVAILABLE")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  nothingctl ota-update          (auto: download + Magisk patch + flash)")
		fmt.Println("  — or step-by-step —")
		fmt.Println("  nothingctl flash-firmware      (flash new firmware)")
		fmt.Println("  nothingctl push-for-patch       (push image to device for Magisk app)")
		fmt.Println("  nothingctl flash-patched        (flash patched image)")
	} else {
		fmt.Println("Status    : up to date")
		fmt.Println()
		fmt.Println("When an update arrives, run:")
		fmt.Println("  nothingctl ota-update          (one-shot root-preserving update)")
	}

	if htmlURL != "" {
		fmt.Printf("\nRelease   : %s\n", htmlURL)
	}

	return nil
}
