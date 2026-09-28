package backup

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// backupPartitions is the list of partitions dumped by ActionBackup.
// Excludes super, userdata (too large), and raw block device aliases.
var backupPartitions = []string{
	"init_boot_a", "init_boot_b",
	"boot_a", "boot_b",
	"dtbo_a", "dtbo_b",
	"vendor_boot_a", "vendor_boot_b",
	"vbmeta_a", "vbmeta_b",
	"vbmeta_system_a", "vbmeta_system_b",
	"vbmeta_vendor_a", "vbmeta_vendor_b",
	"lk_a", "lk_b",
	"logo_a", "logo_b",
	"preloader_raw_a", "preloader_raw_b",
	"modem_a", "modem_b",
	"tee_a", "tee_b",
	"nvram", "nvdata", "nvcfg",
	"factory", "persist", "seccfg", "proinfo",
}

// restoreSafe is the set of partitions safe to flash via fastboot restore.
var restoreSafe = map[string]bool{
	"init_boot_a": true, "init_boot_b": true,
	"boot_a": true, "boot_b": true,
	"dtbo_a": true, "dtbo_b": true,
	"vendor_boot_a": true, "vendor_boot_b": true,
	"vbmeta_a": true, "vbmeta_b": true,
	"vbmeta_system_a": true, "vbmeta_system_b": true,
	"vbmeta_vendor_a": true, "vbmeta_vendor_b": true,
	"logo_a": true, "logo_b": true,
	"modem_a": true, "modem_b": true,
}

// restoreRisky is the set of partitions that carry calibration data or
// very-early boot code. Only flashed with full restore.
var restoreRisky = map[string]bool{
	"lk_a": true, "lk_b": true,
	"preloader_raw_a": true, "preloader_raw_b": true,
	"tee_a": true, "tee_b": true,
	"nvram": true, "nvdata": true, "nvcfg": true,
	"factory": true, "persist": true, "seccfg": true, "proinfo": true,
}

const backupTempDir = "/sdcard/Download/partition_backup"

// ActionBackupCtx dumps all backupPartitions from the device to a timestamped
// local directory under baseDir/Backups/partition-backup/. Requires ADB root.
// ctx is honoured for cancellation between and during per-partition dd calls.
func ActionBackupCtx(ctx context.Context, serial, baseDir string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError(
			"root not available via ADB shell.\n" +
				"Enable in Magisk: Settings -> Superuser access -> Apps and ADB.",
		)
	}

	timestamp := time.Now().Format("20060102_150405")
	return actionBackupWithLabelCtx(ctx, serial, baseDir, timestamp)
}

// ActionBackupWithLabelCtx is like ActionBackupCtx but accepts a custom label
// for the backup directory name (e.g. "pre_flash_v2.6"). Used by flash operations.
func ActionBackupWithLabelCtx(ctx context.Context, serial, baseDir, label string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError(
			"root not available via ADB shell.\n" +
				"Enable in Magisk: Settings -> Superuser access -> Apps and ADB.",
		)
	}
	return actionBackupWithLabelCtx(ctx, serial, baseDir, label)
}

func actionBackupWithLabelCtx(ctx context.Context, serial, baseDir, label string) error {
	localDir := filepath.Join(baseDir, "Backups", "partition-backup", "backup_"+label)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return nterrors.AdbError("creating backup directory: " + err.Error())
	}

	fmt.Printf("\nBacking up partitions to: %s\n", localDir)
	fmt.Println("Checking which partitions exist on device...")

	stdout, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell", "su -c 'ls /dev/block/by-name/'",
	})
	existingSet := make(map[string]bool)
	for _, line := range adb.ParseShellLines(stdout) {
		existingSet[line] = true
	}

	var toDump []string
	var skipped []string
	for _, p := range backupPartitions {
		if existingSet[p] {
			toDump = append(toDump, p)
		} else {
			skipped = append(skipped, p)
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("  Skipping (not present): %s\n", strings.Join(skipped, ", "))
	}

	// Create temp dir on device.
	adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'mkdir -p " + backupTempDir + "'",
	})

	var failed []string
	for _, part := range toDump {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Printf("  Dumping %s...", part)
		_, _, code := adb.RunCtx(ctx, []string{
			"adb", "-s", serial, "shell",
			"su -c 'dd if=/dev/block/by-name/" + part +
				" of=" + backupTempDir + "/" + part + ".img bs=4096 2>/dev/null'",
		})
		if code == 0 {
			fmt.Println(" OK")
		} else {
			fmt.Println(" FAIL")
			failed = append(failed, part)
			// Remove the partial image so it is never pulled, checksummed
			// and later restored as if it were complete.
			adb.Run([]string{
				"adb", "-s", serial, "shell",
				"su -c 'rm -f " + backupTempDir + "/" + part + ".img'",
			})
		}
	}

	if len(failed) > 0 {
		fmt.Printf("  WARNING: Failed to dump: %s\n", strings.Join(failed, ", "))
	}

	successCount := len(toDump) - len(failed)
	fmt.Printf("\nPulling %d images to PC...\n", successCount)
	if err := adb.AdbPullCtx(ctx, serial, backupTempDir+"/.", localDir); err != nil {
		return nterrors.AdbError("adb pull failed: " + err.Error())
	}

	// Clean up temp dir on device.
	adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'rm -rf " + backupTempDir + "'",
	})

	// Enumerate pulled images.
	entries, _ := os.ReadDir(localDir)
	var images []string
	var totalBytes int64
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".img") {
			p := filepath.Join(localDir, e.Name())
			images = append(images, p)
			if info, err := os.Stat(p); err == nil {
				totalBytes += info.Size()
			}
		}
	}
	totalMB := float64(totalBytes) / 1024 / 1024

	// Save checksums.
	if err := saveChecksums(images, localDir); err != nil {
		fmt.Printf("  WARNING: could not save checksums: %v\n", err)
	}

	fmt.Printf("[OK] %d partitions backed up (%.0f MB) -> %s\n",
		len(images), totalMB, localDir)
	return nil
}

// sha256File computes the SHA-256 hex digest of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// saveChecksums writes a checksums.sha256 file alongside the images.
func saveChecksums(images []string, destDir string) error {
	sort.Strings(images)
	var lines []string
	for _, imgPath := range images {
		hash, err := sha256File(imgPath)
		if err != nil {
			return err
		}
		lines = append(lines, hash+"  "+filepath.Base(imgPath))
	}
	content := strings.Join(lines, "\n") + "\n"
	checksumFile := filepath.Join(destDir, "checksums.sha256")
	if err := os.WriteFile(checksumFile, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Printf("  Checksums : checksums.sha256 (%d entries)\n", len(lines))
	return nil
}

// ActionRestore flashes partition images from backupDir back to the device via
// fastboot. partitions is an optional filter list; if empty all safe (and
// optionally risky) partitions are flashed. dryRun prints what would be done
// without actually flashing.
func ActionRestore(serial, backupDir string, dryRun bool, partitions []string) error {
	if backupDir == "" {
		return nterrors.FirmwareError("--restore-dir is required for restore")
	}
	if _, err := os.Stat(backupDir); err != nil {
		return nterrors.FirmwareError("restore directory not found: " + backupDir)
	}

	entries, _ := os.ReadDir(backupDir)
	images := make(map[string]string) // partition name -> full path
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".img") {
			partName := strings.TrimSuffix(e.Name(), ".img")
			images[partName] = filepath.Join(backupDir, e.Name())
		}
	}

	safe := make(map[string]string)
	risky := make(map[string]string)
	unknown := make(map[string]string)
	for k, v := range images {
		if restoreSafe[k] {
			safe[k] = v
		} else if restoreRisky[k] {
			risky[k] = v
		} else {
			unknown[k] = v
		}
	}

	fmt.Printf("\nBackup : %s\n", filepath.Base(backupDir))
	fmt.Printf("Safe   (%2d): %s\n", len(safe), joinMapKeys(safe))
	if len(risky) > 0 {
		fmt.Printf("Risky  (%2d): %s\n", len(risky), joinMapKeys(risky))
	}
	if len(unknown) > 0 {
		fmt.Printf("Unknown (%2d): %s  — skipped\n", len(unknown), joinMapKeys(unknown))
	}

	toFlash := make(map[string]string)
	for k, v := range safe {
		toFlash[k] = v
	}

	// Apply partition filter if provided.
	if len(partitions) > 0 {
		filterSet := make(map[string]bool)
		for _, p := range partitions {
			filterSet[p] = true
		}
		filtered := make(map[string]string)
		for k, v := range toFlash {
			if filterSet[k] {
				filtered[k] = v
			}
		}
		toFlash = filtered
	}

	if len(toFlash) == 0 {
		return nterrors.FirmwareError("no partitions to flash after filtering")
	}

	fmt.Printf("\nWill flash %d partitions to device.\n", len(toFlash))

	if dryRun {
		fmt.Println("\n[DRY RUN] Would flash:")
		keys := sortedKeys(toFlash)
		for _, k := range keys {
			fmt.Printf("  fastboot flash %-24s %s\n", k, toFlash[k])
		}
		return nil
	}

	if !adb.Confirm("Reboot to bootloader and restore?") {
		return nterrors.MagiskError("cancelled by user")
	}

	if err := adb.RebootToBootloader(serial); err != nil {
		return err
	}

	var flashFailed []string
	for _, part := range sortedKeys(toFlash) {
		if err := adb.FastbootFlash(serial, part, toFlash[part]); err != nil {
			fmt.Printf("  WARN: %v\n", err)
			flashFailed = append(flashFailed, part)
		}
	}

	if _, err := adb.FastbootRun(serial, []string{"reboot"}); err != nil {
		fmt.Printf("  WARN: reboot failed: %v\n", err)
	}

	if len(flashFailed) > 0 {
		fmt.Printf("\nWARNING: Failed to flash: %s\n", strings.Join(flashFailed, ", "))
	}
	ok := len(toFlash) - len(flashFailed)
	fmt.Printf("[OK] Restore complete — %d/%d partitions flashed.\n", ok, len(toFlash))
	return nil
}

// backupTime returns when the backup in dir was taken. Timestamped backups
// (backup_YYYYMMDD_HHMMSS) use the time in their name; labelled auto-backups
// (backup_pre_flash_<tag>, backup_pre_patch_flash, …) carry no date, so the
// modification time of checksums.sha256 — written last, when the backup
// completed — is used, falling back to the directory's own mtime.
func backupTime(dir string) time.Time {
	name := strings.TrimPrefix(filepath.Base(dir), "backup_")
	if t, err := time.ParseInLocation("20060102_150405", name, time.Local); err == nil {
		return t
	}
	if info, err := os.Stat(filepath.Join(dir, "checksums.sha256")); err == nil {
		return info.ModTime()
	}
	if info, err := os.Stat(dir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// ListBackups returns all backup_* directories under baseDir/Backups/partition-backup/,
// sorted newest-first by backup date (see backupTime), then by name.
func ListBackups(baseDir string) ([]string, error) {
	backupRoot := filepath.Join(baseDir, "Backups", "partition-backup")
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "backup_") {
			dirs = append(dirs, filepath.Join(backupRoot, e.Name()))
		}
	}
	// Sort newest first by date: labelled backups have no timestamp in their
	// name, so sorting by name alone put them ahead of newer timestamped ones.
	times := make(map[string]time.Time, len(dirs))
	for _, d := range dirs {
		times[d] = backupTime(d)
	}
	sort.SliceStable(dirs, func(i, j int) bool {
		ti, tj := times[dirs[i]], times[dirs[j]]
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return dirs[i] > dirs[j]
	})
	return dirs, nil
}

// PickBackup prompts the user to pick a backup if restoreDir is empty.
func PickBackup(baseDir, restoreDir string) (string, error) {
	if restoreDir != "" {
		if _, err := os.Stat(restoreDir); err != nil {
			return "", nterrors.FirmwareError("restore directory not found: " + restoreDir)
		}
		return restoreDir, nil
	}

	backups, err := ListBackups(baseDir)
	if err != nil || len(backups) == 0 {
		return "", nterrors.FirmwareError(
			"no partition backups found in " + filepath.Join(baseDir, "Backups", "partition-backup"),
		)
	}

	fmt.Println("\nAvailable backups:")
	for i, b := range backups {
		entries, _ := os.ReadDir(b)
		var imgCount int
		var totalBytes int64
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".img") {
				imgCount++
				if info, err := e.Info(); err == nil {
					totalBytes += info.Size()
				}
			}
		}
		fmt.Printf("  [%d] %s  %s  (%d partitions, %.0f MB)\n",
			i, backupTime(b).Format("2006-01-02 15:04"), filepath.Base(b),
			imgCount, float64(totalBytes)/1024/1024)
	}

	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		return "", fmt.Errorf("no interactive terminal — use --restore-dir to specify backup directory directly")
	}
	fmt.Print("\nSelect backup [0]: ")
	scanner := bufio.NewScanner(os.Stdin)
	idx := 0
	if scanner.Scan() {
		choice := strings.TrimSpace(scanner.Text())
		if choice != "" {
			if n, err := strconv.Atoi(choice); err == nil {
				idx = n
			}
		}
	}
	if idx < 0 || idx >= len(backups) {
		return "", nterrors.FirmwareError(fmt.Sprintf("invalid selection: %d", idx))
	}
	return backups[idx], nil
}

// helpers

func joinMapKeys(m map[string]string) string {
	keys := sortedKeys(m)
	return strings.Join(keys, ", ")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
