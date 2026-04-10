package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/arb"
	"github.com/Limplom/nothingctl/internal/backup"
	"github.com/Limplom/nothingctl/internal/firmware"
	"github.com/Limplom/nothingctl/internal/glyph"
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
	flagSkipLogical bool
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
	verifyBackupCmd.Flags().BoolVar(&flagLive, "live", false,
		"compare checksums against live device partitions via adb (requires root)")
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

	// full-flash
	fullFlashCmd.Flags().BoolVar(&flagSkipLogical, "skip-logical", false,
		"skip the ~4 GB logical partition download (flash firmware + boot only)")
	rootCmd.AddCommand(fullFlashCmd)

	// history / status
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(rootStatusCmd)
	rootCmd.AddCommand(checkUpdateCmd)
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
	Use:     "backup",
	GroupID: "backup",
	Short:   "Dump all critical partitions from device to local storage (requires root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		device, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		baseDir := filepath.Join(resolveBaseDir(), device.Codename)
		fb := glyph.NewFeedback(serial, device.Codename)
		fb.StartWithContext(ctx)
		defer fb.Cancel()
		err = backup.ActionBackupCtx(ctx, serial, baseDir)
		if err == nil {
			fb.Done()
		}
		return err
	},
}

// ---------------------------------------------------------------------------
// restore
// ---------------------------------------------------------------------------

var restoreCmd = &cobra.Command{
	Use:     "restore",
	GroupID: "backup",
	Short:   "Flash partitions from a backup back to device",
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
	Use:     "verify-backup",
	GroupID: "backup",
	Short:   "Re-hash backup images and compare against stored checksums",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Always resolve serial — both live and offline paths may need it.
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}

		backupDir := flagRestoreDir
		if backupDir == "" {
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

		if flagLive {
			device, err := adb.DetectDevice(serial)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			fb := glyph.NewFeedback(serial, device.Codename)
			fb.StartWithContext(ctx)
			defer fb.Cancel()
			err = backup.ActionVerifyBackupLive(serial, backupDir)
			if err == nil {
				fb.Done()
			}
			return err
		}
		return backup.ActionVerifyBackup(backupDir)
	},
}

// ---------------------------------------------------------------------------
// flash-firmware
// ---------------------------------------------------------------------------

var flashFirmwareCmd = &cobra.Command{
	Use:     "flash-firmware",
	GroupID: "firmware",
	Short:   "Download latest firmware and flash boot partitions to both A/B slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

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
				if backupErr := backup.ActionBackupWithLabelCtx(ctx, serial, deviceDir,
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

		if err := adb.RebootToBootloaderCtx(ctx, serial); err != nil {
			return err
		}
		slot, _ := adb.QueryCurrentSlot(serial)
		fmt.Printf("Active slot: %s\n", slot)

		for _, part := range partitions {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			imgPath := filepath.Join(fw.ExtractedDir, part+".img")
			if _, err := os.Stat(imgPath); err != nil {
				fmt.Printf("  Skipping %s (image not found in package)\n", part)
				continue
			}
			if err := adb.FastbootFlashABCtx(ctx, serial, part, imgPath); err != nil {
				return err
			}
		}

		fmt.Println("\nFlash complete. Rebooting...")
		if err := adb.FastbootRebootCtx(ctx, serial); err != nil {
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
	Use:     "ota-update",
	GroupID: "firmware",
	Short:   "One-shot: download latest firmware and flash patched boot image (preserves root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

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
			if backupErr := backup.ActionBackupWithLabelCtx(ctx, serial, deviceDir,
				"pre_ota_"+fw.Version); backupErr != nil {
				fmt.Printf("  WARNING: backup failed: %v\n", backupErr)
			}
		}

		fmt.Printf("\nWill flash patched %s to BOTH slots.\n", fw.BootTarget.PartitionBase)
		if !adb.Confirm("Reboot to bootloader and flash?") {
			return fmt.Errorf("cancelled by user")
		}

		if err := adb.RebootToBootloaderCtx(ctx, serial); err != nil {
			return err
		}
		if err := adb.FastbootFlashABCtx(ctx, serial, fw.BootTarget.PartitionBase, localPatched); err != nil {
			return err
		}
		if err := adb.FastbootRebootCtx(ctx, serial); err != nil {
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

	remoteOut := ""
	defer func() {
		toRemove := remoteIn
		if remoteOut != "" {
			toRemove += " " + remoteOut
		}
		adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + toRemove})
	}()

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
		return "", fmt.Errorf("magisk CLI patch failed — ensure Magisk is installed and root is granted.\nOutput: %s", strings.TrimSpace(stdout))
	}

	// Find the output file (Magisk writes to same directory).
	stdout2, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"ls -t " + temp + "/magisk_patched_*.img 2>/dev/null | head -1",
	})
	remoteOut = strings.TrimSpace(stdout2)
	if remoteOut == "" {
		return "", fmt.Errorf("patched image not found in %s after Magisk patch", temp)
	}

	parts := strings.Split(remoteOut, "/")
	patchName := parts[len(parts)-1]
	localPatched := filepath.Join(extractedDir, patchName)

	fmt.Printf("  Pulling %s...\n", patchName)
	if err := adb.AdbPull(serial, remoteOut, localPatched); err != nil {
		return "", err
	}
	return localPatched, nil
}

// ---------------------------------------------------------------------------
// install-magisk
// ---------------------------------------------------------------------------

var installMagiskCmd = &cobra.Command{
	Use:     "install-magisk",
	GroupID: "magisk",
	Short:   "Download and install (or update) the Magisk app on device",
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
	Use:     "update-magisk",
	GroupID: "magisk",
	Short:   "Update Magisk to the latest version (alias for install-magisk)",
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
	Use:     "unroot",
	GroupID: "firmware",
	Short:   "Flash stock boot image to both slots (removes root, enables OTA)",
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
	Use:     "push-for-patch",
	GroupID: "firmware",
	Short:   "Push stock boot/init_boot image to device for manual Magisk patching",
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
	Use:     "flash-patched",
	GroupID: "firmware",
	Short:   "Pull magisk_patched image from device and flash to both A/B slots",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

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
				if backupErr := backup.ActionBackupWithLabelCtx(ctx, serial, deviceDir,
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
	Use:     "fix-biometric",
	GroupID: "magisk",
	Short:   "Force strong auth (PIN/password) for current lock session",
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
	Use:     "history",
	GroupID: "firmware",
	Short:   "Display the flash operation history log",
	RunE: func(cmd *cobra.Command, args []string) error {
		return history.ActionHistory(resolveBaseDir())
	},
}

// ---------------------------------------------------------------------------
// root-status
// ---------------------------------------------------------------------------

var rootStatusCmd = &cobra.Command{
	Use:     "root-status",
	GroupID: "firmware",
	Short:   "Detect and display active root manager (Magisk / KernelSU / APatch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagSerial == "all" {
			return runOnAllDevices(func(s string) error {
				magisk.PrintRootStatus(s)
				return nil
			})
		}
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		magisk.PrintRootStatus(serial)
		return nil
	},
}

// ---------------------------------------------------------------------------
// check-update
// ---------------------------------------------------------------------------

var checkUpdateCmd = &cobra.Command{
	Use:     "check-update",
	GroupID: "firmware",
	Short:   "Check nothing_archive for a firmware update (no download)",
	Long: `Reads the current firmware version from the connected device, queries the
spike0en/nothing_archive GitHub releases for the latest build matching the
device codename, and reports whether an update is available.

No files are downloaded — use ota-update to download and flash.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagSerial == "all" {
			return runOnAllDevices(func(s string) error {
				dev, err := adb.DetectDevice(s)
				if err != nil {
					return err
				}
				return firmware.CheckUpdate(s, dev.Codename)
			})
		}
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		dev, err := adb.DetectDevice(serial)
		if err != nil {
			return err
		}
		return firmware.CheckUpdate(serial, dev.Codename)
	},
}

// ---------------------------------------------------------------------------
// full-flash
// ---------------------------------------------------------------------------

var fullFlashCmd = &cobra.Command{
	Use:     "full-flash",
	GroupID: "firmware",
	Short:   "Download and flash all partitions (firmware + boot + logical ~4 GB)",
	Long: `Full firmware flash: downloads image-boot, image-firmware, and image-logical
archives from nothing_archive and flashes all partitions.

If Magisk root is active, init_boot is patched before flashing to preserve root.

Requires fastboot access. Device must be connected via USB.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		codename := adb.Prop(serial, "ro.product.device")

		// Wire up Magisk patching if root is available.
		var patchFunc firmware.BootPatchFunc
		if adb.CheckAdbRoot(serial) {
			patchFunc = magisk.MagiskCLIPatch
		}

		return firmware.ActionFullFlashCtx(
			ctx,
			serial,
			codename,
			resolveBaseDir(),
			flagForceDownload,
			flagSkipLogical,
			patchFunc,
		)
	},
}
