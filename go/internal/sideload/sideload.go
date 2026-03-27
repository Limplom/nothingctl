// Package sideload provides APK / split-APK sideload helpers.
package sideload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// ActionSideload installs a single APK or a directory of split APKs.
func ActionSideload(serial, apkPath string, downgrade bool) error {
	info, err := os.Stat(apkPath)
	if err != nil {
		return nterrors.AdbError(fmt.Sprintf("Path not found: %s", apkPath))
	}

	var cmd []string

	if info.IsDir() {
		apks, err := filepath.Glob(filepath.Join(apkPath, "*.apk"))
		if err != nil || len(apks) == 0 {
			return nterrors.AdbError(fmt.Sprintf("No .apk files found in %s", apkPath))
		}
		fmt.Printf("\nSplit APK install (%d parts):\n", len(apks))
		for _, a := range apks {
			fmt.Printf("  %s\n", filepath.Base(a))
		}
		cmd = []string{"adb", "-s", serial, "install-multiple", "-r"}
		if downgrade {
			cmd = append(cmd, "-d")
		}
		cmd = append(cmd, apks...)
	} else {
		ext := strings.ToLower(filepath.Ext(apkPath))
		if ext != ".apk" {
			return nterrors.AdbError(fmt.Sprintf("Expected a .apk file or directory, got: %s", filepath.Base(apkPath)))
		}
		fmt.Printf("\nInstalling %s (%d KB)...\n", filepath.Base(apkPath), info.Size()/1024)
		cmd = []string{"adb", "-s", serial, "install", "-r"}
		if downgrade {
			cmd = append(cmd, "-d")
		}
		cmd = append(cmd, apkPath)
	}

	stdout, stderr, code := adb.Run(cmd)
	output := strings.TrimSpace(stdout + stderr)

	if code == 0 && strings.Contains(output, "Success") {
		fmt.Println("[OK] Installed successfully.")
		return nil
	}
	if strings.Contains(output, "INSTALL_FAILED_VERSION_DOWNGRADE") {
		return nterrors.AdbError("Install blocked: target version is older than installed.\nUse --downgrade to allow lower versionCode installs.")
	}
	if strings.Contains(output, "INSTALL_FAILED_ALREADY_EXISTS") {
		return nterrors.AdbError("Package already installed. Use -r flag (already included) — if still failing, uninstall first.")
	}
	return nterrors.AdbError(fmt.Sprintf("Install failed: %s", output))
}
