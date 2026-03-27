package firmware

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// sevenZipCandidates is the ordered list of 7-Zip executable names/paths to try.
var sevenZipCandidates = []string{
	"7z",
	"7zz",
	"7za",
	`C:\Program Files\7-Zip\7z.exe`,
	"/c/Program Files/7-Zip/7z.exe",
}

// Extract7z extracts archivePath into destDir using the first available 7-Zip
// binary. Returns a FirmwareError if no working 7-Zip is found.
func Extract7z(archivePath, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nterrors.FirmwareError("creating extraction directory: " + err.Error())
	}

	for _, z := range sevenZipCandidates {
		cmd := exec.Command(z, "e", archivePath, "-o"+destDir, "-y")
		var outBuf strings.Builder
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
		if err := cmd.Run(); err == nil {
			return nil
		}
		// FileNotFoundError equivalent: the binary simply doesn't exist.
		// exec.Command.Run() returns an error wrapping exec.ErrNotFound in that case.
	}

	return nterrors.FirmwareError(
		"Extraction failed. Ensure 7-Zip is installed.\n" +
			"  Windows : https://www.7-zip.org/download.html  (adds 7z to PATH)\n" +
			"  macOS   : brew install p7zip\n" +
			"  Linux   : sudo apt install p7zip-full",
	)
}

// DetectBootTarget inspects extractedDir and returns the boot image filename
// that should be used for Magisk patching. Returns an error if neither
// init_boot.img nor boot.img is present.
func DetectBootTarget(extractedDir string) (string, error) {
	initBoot := filepath.Join(extractedDir, "init_boot.img")
	boot := filepath.Join(extractedDir, "boot.img")

	if _, err := os.Stat(initBoot); err == nil {
		return "init_boot.img", nil
	}
	if _, err := os.Stat(boot); err == nil {
		return "boot.img", nil
	}
	return "", nterrors.FirmwareError(
		fmt.Sprintf(
			"neither init_boot.img nor boot.img found in %s. "+
				"The firmware package may be incomplete.",
			extractedDir,
		),
	)
}

// BuildPartitionList returns the base partition names (no _a/_b suffix) that
// should be flashed for this firmware. GKI 2.0 devices use init_boot; legacy
// devices use boot.
func BuildPartitionList(extractedDir string) ([]string, error) {
	target, err := DetectBootTarget(extractedDir)
	if err != nil {
		return nil, err
	}

	// Safe flash partitions matching the Python SAFE_FLASH_PARTITIONS constant.
	safe := []string{"boot", "dtbo", "vendor_boot"}

	if target == "init_boot.img" {
		// GKI 2.0: init_boot replaces the patched portion of boot
		return append([]string{"init_boot"}, safe...), nil
	}
	return safe, nil
}
