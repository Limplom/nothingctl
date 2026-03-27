package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// ActionVerifyBackup re-hashes all .img files in backupDir and compares them
// against the stored checksums.sha256. Reports match/changed/missing per
// partition.
func ActionVerifyBackup(backupDir string) error {
	if backupDir == "" {
		return nterrors.FirmwareError("--restore-dir is required for verify-backup")
	}
	if _, err := os.Stat(backupDir); err != nil {
		return nterrors.FirmwareError("backup directory not found: " + backupDir)
	}

	checksumFile := filepath.Join(backupDir, "checksums.sha256")
	raw, err := os.ReadFile(checksumFile)
	if err != nil {
		return nterrors.FirmwareError(
			"no checksums.sha256 found in " + filepath.Base(backupDir) + ".\n" +
				"This backup was created before health-check support was added.\n" +
				"Run backup again to create a new backup with checksums.",
		)
	}

	// Parse "hash  filename" lines.
	stored := make(map[string]string) // filename -> expected hash
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 {
			stored[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
		}
	}

	fmt.Printf("\nVerifying %d files against checksums...\n", len(stored))
	fmt.Printf("Backup: %s\n\n", filepath.Base(backupDir))
	fmt.Printf("  %-22s %-10s Details\n", "Partition", "Result")
	fmt.Println("  " + strings.Repeat("─", 60))

	filenames := make([]string, 0, len(stored))
	for fn := range stored {
		filenames = append(filenames, fn)
	}
	sort.Strings(filenames)

	var matchCount, changedCount, missingCount int
	var changedParts []string

	for _, filename := range filenames {
		expectedHash := stored[filename]
		partName := strings.TrimSuffix(filename, ".img")
		fmt.Printf("  %-22s", partName)

		imgPath := filepath.Join(backupDir, filename)
		if _, err := os.Stat(imgPath); err != nil {
			fmt.Printf(" %-10s file not found in backup directory\n", "MISSING")
			missingCount++
			continue
		}

		liveHash, hashErr := sha256File(imgPath)
		if hashErr != nil {
			fmt.Printf(" %-10s could not hash file: %v\n", "ERROR", hashErr)
			missingCount++
			continue
		}

		if liveHash == expectedHash {
			fmt.Printf(" %-10s\n", "MATCH")
			matchCount++
		} else {
			fmt.Printf(" %-10s hash differs from stored checksum\n", "CHANGED")
			changedCount++
			changedParts = append(changedParts, partName)
		}
	}

	fmt.Println()
	fmt.Printf("  Results: %d match  /  %d changed  /  %d missing\n",
		matchCount, changedCount, missingCount)

	if changedCount > 0 {
		fmt.Printf("\n  Changed files: %s\n", strings.Join(changedParts, ", "))
	} else if missingCount > 0 {
		fmt.Println("\n  Some files are not present in the backup directory.")
	} else {
		fmt.Println("\n[OK] All files match the stored checksums.")
	}
	return nil
}

// ActionVerifyBackupLive compares live device partition hashes against a stored
// backup's checksums.sha256. Hashing runs on-device via dd|sha256sum — only the
// 64-char hash is transferred over USB. Requires ADB root.
func ActionVerifyBackupLive(serial, backupDir string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError(
			"root not available via ADB shell.\n" +
				"Enable in Magisk: Settings -> Superuser access -> Apps and ADB.",
		)
	}

	checksumFile := filepath.Join(backupDir, "checksums.sha256")
	raw, err := os.ReadFile(checksumFile)
	if err != nil {
		return nterrors.FirmwareError(
			"no checksums.sha256 found in " + filepath.Base(backupDir) + ".\n" +
				"This backup was created before health-check support was added.\n" +
				"Run backup again to create a new backup with checksums.",
		)
	}

	stored := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 {
			stored[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[0])
		}
	}

	fmt.Printf("\nVerifying %d partitions against live device...\n", len(stored))
	fmt.Printf("Backup: %s\n\n", filepath.Base(backupDir))
	fmt.Printf("  %-22s %-10s Details\n", "Partition", "Result")
	fmt.Println("  " + strings.Repeat("─", 60))

	filenames := make([]string, 0, len(stored))
	for fn := range stored {
		filenames = append(filenames, fn)
	}
	sort.Strings(filenames)

	var matchCount, changedCount, missingCount int
	var changedParts []string

	for _, filename := range filenames {
		expectedHash := stored[filename]
		partName := strings.TrimSuffix(filename, ".img")
		fmt.Printf("  %-22s", partName)

		stdout, _, code := adb.Run([]string{
			"adb", "-s", serial, "shell",
			"su -c 'dd if=/dev/block/by-name/" + partName + " bs=4096 2>/dev/null | sha256sum'",
		})
		liveOutput := strings.TrimSpace(stdout)

		if code != 0 || liveOutput == "" || strings.Contains(liveOutput, "No such") {
			fmt.Printf(" %-10s partition not found on device\n", "MISSING")
			missingCount++
			continue
		}

		liveHash := strings.Fields(liveOutput)[0]
		if liveHash == expectedHash {
			fmt.Printf(" %-10s\n", "MATCH")
			matchCount++
		} else {
			fmt.Printf(" %-10s live hash differs from backup\n", "CHANGED")
			changedCount++
			changedParts = append(changedParts, partName)
		}
	}

	fmt.Println()
	fmt.Printf("  Results: %d match  /  %d changed  /  %d missing\n",
		matchCount, changedCount, missingCount)

	if changedCount > 0 {
		fmt.Printf("\n  Changed partitions: %s\n", strings.Join(changedParts, ", "))
		fmt.Println("  NOTE: init_boot change is expected if Magisk is installed.")
		fmt.Println("        Other changes may indicate OTA updates or unexpected modifications.")
	} else if missingCount > 0 {
		fmt.Println("\n  Some partitions are not present on this device (model-specific).")
	} else {
		fmt.Println("\n[OK] All partitions match the backup.")
	}
	return nil
}
