package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/arb"
	"github.com/Limplom/nothingctl/internal/backup"
	"github.com/Limplom/nothingctl/internal/firmware"
	"github.com/Limplom/nothingctl/internal/history"
	"github.com/Limplom/nothingctl/internal/magisk"
)

// ---------------------------------------------------------------------------
// Shared flag variables for restore/verify subcommands
// ---------------------------------------------------------------------------

var (
	flagRestoreDir  string
	flagDryRun      bool
	flagPartitions  string // comma-separated list
)

// ---------------------------------------------------------------------------
// init — register all firmware/root subcommands with rootCmd
// ---------------------------------------------------------------------------

func init() {
	// backup
	rootCmd.AddCommand(backupCmd)

	// restore
	restoreCmd.Flags().StringVar(&flagRestoreDir, "restore-dir", "",
		"path to a specific backup directory (skips interactive selection)")
	restoreCmd.Flags().BoolVar(&flagDryRun, "dry-run", false,
		"print what would be flashed without actually flashing")
	restoreCmd.Flags().StringVar(&flagPartitions, "partitions", "",
		"comma-separated partition names to restore (default: all safe partitions)")
	rootCmd.AddCommand(restoreCmd)

	// verify-backup
	verifyBackupCmd.Flags().StringVar(&flagRestoreDir, "restore-dir", "",
		"path to a specific backup directory to verify")
	rootCmd.AddCommand(verifyBackupCmd)

	// firmware commands
	rootCmd.AddCommand(flashFirmwareCmd)
	rootCmd.AddCommand(otaUpdateCmd)

	// magisk commands
	rootCmd.AddCommand(installMagiskCmd)
	rootCmd.AddCommand(updateMagiskCmd)
	rootCmd.AddCommand(unrootCmd)
	rootCmd.AddCommand(pushForPatchCmd)
	rootCmd.AddCommand(flashPatchedCmd)
	rootCmd.AddCommand(fixBiometricCmd)

	// history / status
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(rootStatusCmd)
}

// resolveBaseDir returns the effective base directory, defaulting to
// ~/.nothingctl if --base-dir was not specified.
func resolveBaseDir() string {
	if flagBaseDir != "" {
		return flagBaseDir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".nothingctl")
	}
	return filepath.Join(home, ".nothingctl")
}

// ---------------------------------------------------------------------------
// backup
// ---------------------------------------------------------------------------

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Dump all critical partitions from device to local storage (requires root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := filepath.Join(resolveBaseDir(), device.Codename)
		return backup.ActionBackup(serial, baseDir)
	},
}

// ---------------------------------------------------------------------------
// restore
// ---------------------------------------------------------------------------

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Flash partitions from a backup back to device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := filepath.Join(resolveBaseDir(), device.Codename)

		restoreDir := flagRestoreDir
		if restoreDir == "" {
			// Interactive selection.
			restoreDir, err = backup.PickBackup(baseDir, "")
			if err != nil {
				return err
			}
		}

		var partList []string
		if flagPartitions != "" {
			for _, p := range strings.Split(flagPartitions, ",") {
				if t := strings.TrimSpace(p); t != "" {
					partList = append(partList, t)
				}
			}
		}

		return backup.ActionRestore(serial, restoreDir, flagDryRun, partList)
	},
}

// ---------------------------------------------------------------------------
// verify-backup
// ---------------------------------------------------------------------------

var verifyBackupCmd = &cobra.Command{
	Use:   "verify-backup",
	Short: "Re-hash backup images and compare against stored checksums",
	RunE: func(cmd *cobra.Command, args []string) error {
		backupDir := flagRestoreDir
		if backupDir == "" {
			// Need device to find the codename-specific backup root.
			serial, err := adb.EnsureDevice(flagSerial)
			if err != nil {
				return err
			}
			device, err := adb.DetectDevice(serial)
			if err != nil {
				return err
			}
			baseDir := filepath.Join(resolveBaseDir(), device.Codename)
			backupDir, err = backup.PickBackup(baseDir, "")
			if err != nil {
				return err
			}
		}
		return backup.ActionVerifyBackup(backupDir)
	},
}

// ---------------------------------------------------------------------------
// flash-firmware
// ---------------------------------------------------------------------------

var flashFirmwareCmd = &cobra.Command{
	Use:   "flash-firmware",
	Short: "Download latest firmware and flash boot partitions to both A/B slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := resolveBaseDir()

		fw, err := firmware.ResolveFirmware(serial, device.Codename, baseDir, flagForceDownload)
		if err != nil {
			return err
		}

		partitions, err := firmware.BuildPartitionList(fw.ExtractedDir)
		if err != nil {
			return err
		}
		fmt.Printf("\nWill flash to BOTH slots: %s\n", strings.Join(partitions, ", "))

		fmt.Println("\nAnti-Rollback Protection check:")
		if err := arb.CheckARB(serial, fw.ExtractedDir); err != nil {
			return err
		}

		if !flagNoBackup {
			if adb.CheckAdbRoot(serial) {
				fmt.Println("\nAuto-backup before flash (use --no-backup to skip)...")
				deviceDir := filepath.Join(baseDir, device.Codename)
				if backupErr := backup.ActionBackupWithLabel(serial, deviceDir,
					"pre_flash_"+fw.Version); backupErr != nil {
					fmt.Printf("  WARNING: backup failed: %v\n", backupErr)
				}
			} else {
				fmt.Println("\nWARNING: Root not available — skipping auto-backup.")
				fmt.Println("         Run with --no-backup to suppress this warning.")
			}
		}

		if !adb.Confirm("Reboot to bootloader and flash?") {
			return fmt.Errorf("cancelled by user")
		}

		if err := adb.RebootToBootloader(serial); err != nil {
			return err
		}
		slot, _ := adb.QueryCurrentSlot(serial)
		fmt.Printf("Active slot: %s\n", slot)

		for _, part := range partitions {
			imgPath := filepath.Join(fw.ExtractedDir, part+".img")
			if _, err := os.Stat(imgPath); err != nil {
				fmt.Printf("  Skipping %s (image not found in package)\n", part)
				continue
			}
			if err := adb.FastbootFlashAB(serial, part, imgPath); err != nil {
				return err
			}
		}

		fmt.Println("\nFlash complete. Rebooting...")
		if _, err := adb.FastbootRun(serial, []string{"reboot"}); err != nil {
			fmt.Printf("  WARNING: reboot failed: %v\n", err)
		}

		_ = history.LogFlash(baseDir, map[string]any{
			"operation": "flash-firmware",
			"version":   fw.Version,
			"serial":    serial,
			"arb_index": nil,
		})
		fmt.Println("[OK] Firmware flashed. Now run push-for-patch, patch in Magisk, then flash-patched.")
		return nil
	},
}

// ---------------------------------------------------------------------------
// ota-update
// ---------------------------------------------------------------------------

var otaUpdateCmd = &cobra.Command{
	Use:   "ota-update",
	Short: "One-shot: download latest firmware and flash patched boot image (preserves root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := resolveBaseDir()

		fw, err := firmware.ResolveFirmware(serial, device.Codename, baseDir, flagForceDownload)
		if err != nil {
			return err
		}

		if !fw.IsNewer {
			fmt.Println("\nDevice is already on the latest firmware — nothing to update.")
			fmt.Println("Use --force-download to re-patch the current version anyway.")
			return nil
		}

		localImg := filepath.Join(fw.ExtractedDir, fw.BootTarget.Filename)
		hasRoot := adb.CheckAdbRoot(serial)

		var localPatched string
		if hasRoot {
			fmt.Println("\n[Auto] Root detected — patching with Magisk CLI.")
			localPatched, err = magiskCLIPatch(serial, localImg, fw.ExtractedDir, fw.BootTarget.Filename)
			if err != nil {
				return err
			}
		} else {
			fmt.Println("\n[Manual] No root — pushing image for Magisk app to patch.")
			if err := magisk.ActionPushForPatch(serial, fw.ExtractedDir); err != nil {
				return err
			}
			fmt.Println("\nAfter patching in the Magisk app, run:")
			fmt.Println("  nothingctl flash-patched")
			return nil
		}

		fmt.Println("\nAnti-Rollback Protection check:")
		if err := arb.CheckARB(serial, fw.ExtractedDir); err != nil {
			return err
		}

		if !flagNoBackup {
			fmt.Println("\nAuto-backup before flash (use --no-backup to skip)...")
			deviceDir := filepath.Join(baseDir, device.Codename)
			if backupErr := backup.ActionBackupWithLabel(serial, deviceDir,
				"pre_ota_"+fw.Version); backupErr != nil {
				fmt.Printf("  WARNING: backup failed: %v\n", backupErr)
			}
		}

		fmt.Printf("\nWill flash patched %s to BOTH slots.\n", fw.BootTarget.PartitionBase)
		if !adb.Confirm("Reboot to bootloader and flash?") {
			return fmt.Errorf("cancelled by user")
		}

		if err := adb.RebootToBootloader(serial); err != nil {
			return err
		}
		if err := adb.FastbootFlashAB(serial, fw.BootTarget.PartitionBase, localPatched); err != nil {
			return err
		}
		if _, err := adb.FastbootRun(serial, []string{"reboot"}); err != nil {
			fmt.Printf("  WARNING: reboot failed: %v\n", err)
		}

		_ = history.LogFlash(baseDir, map[string]any{
			"operation": "ota-update",
			"version":   fw.Version,
			"serial":    serial,
			"arb_index": nil,
		})
		fmt.Printf("[OK] OTA complete. Root preserved on both slots. Now on %s.\n", fw.Version)
		return nil
	},
}

// magiskCLIPatch pushes the stock boot image to /data/local/tmp, patches it
// with the Magisk CLI, and pulls the result back. Returns the local path of
// the patched image.
func magiskCLIPatch(serial, localImg, extractedDir, imgName string) (string, error) {
	temp := "/data/local/tmp"
	remoteIn := temp + "/" + imgName

	fmt.Printf("  Pushing %s to device...\n", imgName)
	if err := adb.AdbPush(serial, localImg, remoteIn); err != nil {
		return "", err
	}

	fmt.Println("  Patching with Magisk CLI...")
	stdout, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'magisk --patch-file " + remoteIn + "' && echo __PATCH_OK__",
	})
	if !strings.Contains(stdout, "__PATCH_OK__") {
		adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn})
		return "", fmt.Errorf("magisk CLI patch failed — ensure Magisk is installed and root is granted.\nOutput: %s", strings.TrimSpace(stdout))
	}

	// Find the output file (Magisk writes to same directory).
	stdout2, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"ls -t " + temp + "/magisk_patched_*.img 2>/dev/null | head -1",
	})
	remoteOut := strings.TrimSpace(stdout2)
	if remoteOut == "" {
		adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn})
		return "", fmt.Errorf("patched image not found in %s after Magisk patch", temp)
	}

	parts := strings.Split(remoteOut, "/")
	patchName := parts[len(parts)-1]
	localPatched := filepath.Join(extractedDir, patchName)

	fmt.Printf("  Pulling %s...\n", patchName)
	if err := adb.AdbPull(serial, remoteOut, localPatched); err != nil {
		return "", err
	}
	adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteIn + " " + remoteOut})
	return localPatched, nil
}

// ---------------------------------------------------------------------------
// install-magisk
// ---------------------------------------------------------------------------

var installMagiskCmd = &cobra.Command{
	Use:   "install-magisk",
	Short: "Download and install (or update) the Magisk app on device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return magisk.ActionInstallMagisk(serial, resolveBaseDir())
	},
}

// ---------------------------------------------------------------------------
// update-magisk
// ---------------------------------------------------------------------------

var updateMagiskCmd = &cobra.Command{
	Use:   "update-magisk",
	Short: "Update Magisk to the latest version (alias for install-magisk)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return magisk.ActionUpdateMagisk(serial, resolveBaseDir())
	},
}

// ---------------------------------------------------------------------------
// unroot
// ---------------------------------------------------------------------------

var unrootCmd = &cobra.Command{
	Use:   "unroot",
	Short: "Flash stock boot image to both slots (removes root, enables OTA)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		fw, err := firmware.ResolveFirmware(serial, device.Codename, resolveBaseDir(), flagForceDownload)
		if err != nil {
			return err
		}
		return magisk.ActionUnroot(serial, fw.ExtractedDir)
	},
}

// ---------------------------------------------------------------------------
// push-for-patch
// ---------------------------------------------------------------------------

var pushForPatchCmd = &cobra.Command{
	Use:   "push-for-patch",
	Short: "Push stock boot/init_boot image to device for manual Magisk patching",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		fw, err := firmware.ResolveFirmware(serial, device.Codename, resolveBaseDir(), flagForceDownload)
		if err != nil {
			return err
		}
		return magisk.ActionPushForPatch(serial, fw.ExtractedDir)
	},
}

// ---------------------------------------------------------------------------
// flash-patched
// ---------------------------------------------------------------------------

var flashPatchedCmd = &cobra.Command{
	Use:   "flash-patched",
	Short: "Pull magisk_patched image from device and flash to both A/B slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := resolveBaseDir()

		fw, err := firmware.ResolveFirmware(serial, device.Codename, baseDir, flagForceDownload)
		if err != nil {
			return err
		}

		if !flagNoBackup {
			if adb.CheckAdbRoot(serial) {
				fmt.Println("Auto-backup before flash (use --no-backup to skip)...")
				deviceDir := filepath.Join(baseDir, device.Codename)
				if backupErr := backup.ActionBackupWithLabel(serial, deviceDir,
					"pre_patch_flash"); backupErr != nil {
					fmt.Printf("  WARNING: backup failed: %v\n", backupErr)
				}
			} else {
				fmt.Println("WARNING: Root not available — skipping auto-backup.")
			}
		}

		return magisk.ActionFlashPatched(serial, fw.ExtractedDir)
	},
}

// ---------------------------------------------------------------------------
// fix-biometric
// ---------------------------------------------------------------------------

var fixBiometricCmd = &cobra.Command{
	Use:   "fix-biometric",
	Short: "Force strong auth (PIN/password) for current lock session",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return magisk.ActionFixBiometric(serial)
	},
}

// ---------------------------------------------------------------------------
// history
// ---------------------------------------------------------------------------

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Display the flash operation history log",
	RunE: func(cmd *cobra.Command, args []string) error {
		return history.ActionHistory(resolveBaseDir())
	},
}

// ---------------------------------------------------------------------------
// root-status
// ---------------------------------------------------------------------------

var rootStatusCmd = &cobra.Command{
	Use:   "root-status",
	Short: "Detect and display active root manager (Magisk / KernelSU / APatch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		magisk.PrintRootStatus(serial)
		return nil
	},
}
