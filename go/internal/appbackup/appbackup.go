// Package appbackup provides per-app backup and restore: APK + /data/data via root tar.
package appbackup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const remoteTmp = "/data/local/tmp"

func listUserPackages(serial string) []string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages -3"})
	var pkgs []string
	for _, line := range adb.ParseShellLines(stdout) {
		if strings.HasPrefix(line, "package:") {
			pkgs = append(pkgs, strings.TrimPrefix(line, "package:"))
		}
	}
	return pkgs
}

func apkPath(pkg, serial string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm path " + pkg})
	for _, line := range adb.ParseShellLines(stdout) {
		if strings.HasPrefix(line, "package:") {
			return strings.TrimPrefix(line, "package:")
		}
	}
	return ""
}

func appUID(pkg, serial string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys package " + pkg + " | grep -m1 userId="})
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "userId=") {
			for _, tok := range strings.Fields(line) {
				if strings.HasPrefix(tok, "userId=") {
					return strings.TrimPrefix(tok, "userId=")
				}
			}
		}
	}
	return ""
}

// ActionAppBackup backs up APK + data directory for specified packages.
func ActionAppBackup(serial, baseDir string, packages []string) error {
	if len(packages) == 0 {
		// Interactive selection
		fmt.Println("\nFetching user-installed packages...")
		pkgs := listUserPackages(serial)
		if len(pkgs) == 0 {
			return nterrors.AdbError("No user-installed packages found.")
		}
		fmt.Printf("\n%-4s Package\n", "#")
		fmt.Println(strings.Repeat("─", 60))
		for i, p := range pkgs {
			fmt.Printf("  %-3d %s\n", i, p)
		}
		raw, err := adb.Prompt("\nEnter package numbers or names (comma-separated): ")
		if err != nil {
			return err
		}
		for _, tok := range strings.Split(raw, ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			var idx int
			if n, err2 := fmt.Sscanf(tok, "%d", &idx); n == 1 && err2 == nil {
				if idx >= 0 && idx < len(pkgs) {
					packages = append(packages, pkgs[idx])
				}
			} else {
				packages = append(packages, tok)
			}
		}
		if len(packages) == 0 {
			fmt.Println("Nothing selected.")
			return nil
		}
	}

	timestamp := time.Now().Format("20060102_150405")
	apkDir := filepath.Join(baseDir, "Backups", "apk_extract")
	dataDir := filepath.Join(baseDir, "Backups", "app_backups", timestamp)
	if err := os.MkdirAll(apkDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	fmt.Printf("\nBacking up %d app(s)\n", len(packages))
	fmt.Printf("  APKs → %s\n", apkDir)
	fmt.Printf("  Data → %s\n", dataDir)

	for _, pkg := range packages {
		fmt.Printf("\n  [%s]\n", pkg)

		// APK
		remote := apkPath(pkg, serial)
		if remote != "" {
			localAPK := filepath.Join(apkDir, pkg+".apk")
			parts := strings.Split(remote, "/")
			fmt.Printf("    APK  : pulling %s...\n", parts[len(parts)-1])
			if err := adb.AdbPull(serial, remote, localAPK); err != nil {
				fmt.Printf("           failed: %v\n", err)
			} else {
				fmt.Printf("           saved → %s.apk\n", pkg)
			}
		} else {
			fmt.Println("    APK  : not found (system app?)")
		}

		// Data
		remoteTar := fmt.Sprintf("%s/%s_data.tar.gz", remoteTmp, pkg)
		tarCmd := fmt.Sprintf("su -c 'test -d /data/data/%s && tar czf %s -C /data/data %s 2>/dev/null && echo __OK__'", pkg, remoteTar, pkg)
		stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", tarCmd})
		if strings.Contains(stdout, "__OK__") {
			localTar := filepath.Join(dataDir, pkg+"_data.tar.gz")
			fmt.Println("    Data : pulling tar archive...")
			if err := adb.AdbPull(serial, remoteTar, localTar); err != nil {
				fmt.Printf("           failed: %v\n", err)
			} else {
				if info, err := os.Stat(localTar); err == nil {
					sizeMB := float64(info.Size()) / 1024 / 1024
					fmt.Printf("           saved → %s_data.tar.gz  (%.1f MB)\n", pkg, sizeMB)
				}
			}
			adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteTar})
		} else {
			fmt.Printf("    Data : skipped (no root or /data/data/%s not found)\n", pkg)
		}
	}

	fmt.Println("\n[OK] App backup complete")
	fmt.Printf("     APKs  → %s\n", apkDir)
	fmt.Printf("     Data  → %s\n", dataDir)
	return nil
}

// ActionAppRestore restores apps from a backup directory.
func ActionAppRestore(serial, baseDir string, packages []string) error {
	if baseDir == "" {
		return nterrors.AdbError("Specify the backup directory with --base-dir <path>")
	}

	src := baseDir
	if _, err := os.Stat(src); err != nil {
		return nterrors.AdbError(fmt.Sprintf("Restore directory not found: %s", src))
	}

	// Find data tarballs
	var datas []string
	entries, _ := filepath.Glob(filepath.Join(src, "*_data.tar.gz"))
	datas = append(datas, entries...)

	// APKs from sibling apk_extract dir
	apkDir := filepath.Join(filepath.Dir(filepath.Dir(src)), "apk_extract")
	var apks []string
	apkEntries, _ := filepath.Glob(filepath.Join(apkDir, "*.apk"))
	apks = append(apks, apkEntries...)

	if len(apks) == 0 && len(datas) == 0 {
		return nterrors.AdbError(fmt.Sprintf("No .apk or _data.tar.gz files found in %s or %s", src, apkDir))
	}

	fmt.Printf("\nRestore data from : %s\n", src)
	fmt.Printf("APKs from         : %s\n", apkDir)
	fmt.Printf("  APKs  : %d\n", len(apks))
	fmt.Printf("  Data  : %d\n", len(datas))
	if !adb.Confirm("Proceed?") {
		return nil
	}

	// Reinstall APKs
	for _, apk := range apks {
		fmt.Printf("\n  Installing %s...\n", filepath.Base(apk))
		stdout, stderr, _ := adb.Run([]string{"adb", "-s", serial, "install", "-r", apk})
		combined := stdout + stderr
		if strings.Contains(combined, "Success") {
			fmt.Println("    [OK]")
		} else {
			fmt.Printf("    [WARN] %s\n", strings.TrimSpace(combined))
		}
	}

	// Restore data
	for _, tar := range datas {
		pkg := strings.TrimSuffix(filepath.Base(tar), "_data.tar.gz")
		fmt.Printf("\n  Restoring data for %s...\n", pkg)
		remoteTar := fmt.Sprintf("%s/%s", remoteTmp, filepath.Base(tar))
		if err := adb.AdbPush(serial, tar, remoteTar); err != nil {
			fmt.Printf("    [WARN] push failed: %v\n", err)
			continue
		}

		uid := appUID(pkg, serial)
		if uid == "" {
			uid = "1000"
		}
		cmd := fmt.Sprintf("su -c 'tar xzf %s -C /data/data 2>/dev/null && chown -R %s:%s /data/data/%s && rm -f %s && echo __OK__'",
			remoteTar, uid, uid, pkg, remoteTar)
		stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
		if strings.Contains(stdout, "__OK__") {
			fmt.Printf("    [OK] data restored (uid=%s)\n", uid)
		} else {
			fmt.Printf("    [WARN] data restore may have failed: %s\n", strings.TrimSpace(stdout))
			adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remoteTar})
		}
	}

	fmt.Println("\n[OK] Restore complete. You may need to relaunch restored apps.")
	return nil
}
