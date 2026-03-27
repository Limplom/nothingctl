// Package storage provides storage reports and APK extraction for Nothing phones.
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

func parseDU(line string) (int, string, bool) {
	line = strings.TrimRight(line, "\r")
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	var kb int
	if _, err := fmt.Sscanf(parts[0], "%d", &kb); err != nil {
		return 0, "", false
	}
	return kb, strings.TrimSpace(parts[1]), true
}

func fmtSize(kb int) string {
	switch {
	case kb >= 1024*1024:
		return fmt.Sprintf("%6.1f GB", float64(kb)/1024/1024)
	case kb >= 1024:
		return fmt.Sprintf("%6.1f MB", float64(kb)/1024)
	default:
		return fmt.Sprintf("%6d KB", kb)
	}
}

func duSorted(serial, path string, topN int, useRoot bool) [][2]interface{} {
	cmd := fmt.Sprintf("du -sk %s/*/ 2>/dev/null | sort -rn | head -%d", path, topN)
	if useRoot {
		cmd = fmt.Sprintf("su -c '%s'", cmd)
	}
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
	var results [][2]interface{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		kb, fullPath, ok := parseDU(line)
		if ok {
			results = append(results, [2]interface{}{kb, fullPath})
		}
	}
	return results
}

func freeSpace(serial, mount string) string {
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("df -h %s 2>/dev/null | tail -1", mount)})
	parts := strings.Fields(strings.TrimRight(out, "\r\n"))
	if len(parts) >= 4 {
		return fmt.Sprintf("free %s of %s", parts[3], parts[1])
	}
	return "unknown"
}

// ActionStorageReport shows top-N largest directories in key paths.
func ActionStorageReport(serial string, topN int) error {
	type section struct {
		path, title string
		needsRoot   bool
	}
	sections := []section{
		{"/data/data", "App data  (/data/data/)", true},
		{"/sdcard/Android/data", "App cache (/sdcard/Android/data/)", false},
		{"/sdcard", "SD card   (/sdcard/)", false},
	}

	for _, sec := range sections {
		fmt.Printf("\n  %s\n", sec.title)
		free := freeSpace(serial, sec.path)
		fmt.Printf("  %s\n", free)
		fmt.Println("  " + strings.Repeat("\u2500", 52))

		entries := duSorted(serial, sec.path, topN, sec.needsRoot)
		if len(entries) == 0 {
			if sec.needsRoot {
				fmt.Println("  (no data \u2014 root required)")
			} else {
				fmt.Println("  (empty or not accessible)")
			}
			continue
		}

		for _, e := range entries {
			kb := e[0].(int)
			fullPath := e[1].(string)
			name := fullPath
			if i := strings.LastIndex(strings.TrimRight(fullPath, "/"), "/"); i >= 0 {
				name = strings.TrimRight(fullPath, "/")[i+1:]
			}
			fmt.Printf("  %s  %s\n", fmtSize(kb), name)
		}
	}
	fmt.Println()
	return nil
}

// ActionAPKExtract pulls APKs for all user-installed (or all) apps to baseDir/Backups/apk_extract/.
func ActionAPKExtract(serial, baseDir string, includeSystem bool) error {
	flag := "-3"
	if includeSystem {
		flag = ""
	}

	var args []string
	if flag != "" {
		args = []string{"adb", "-s", serial, "shell", "pm", "list", "packages", flag}
	} else {
		args = []string{"adb", "-s", serial, "shell", "pm", "list", "packages"}
	}
	out, _, _ := adb.Run(args)

	var packages []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "package:") {
			pkg := strings.TrimPrefix(line, "package:")
			pkg = strings.TrimSpace(pkg)
			if pkg != "" {
				packages = append(packages, pkg)
			}
		}
	}

	if len(packages) == 0 {
		return nterrors.AdbError("No packages found.")
	}

	ts := time.Now().Format("20060102_150405")
	_ = ts
	destDir := filepath.Join(baseDir, "Backups", "apk_extract")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	label := "user"
	if includeSystem {
		label = "all"
	}
	fmt.Printf("\nExtracting %d APKs (%s) \u2192 %s\n", len(packages), label, destDir)

	ok := 0
	failed := 0

	for _, pkg := range packages {
		pathOut, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm", "path", pkg})
		apkPath := ""
		for _, line := range strings.Split(pathOut, "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.HasPrefix(line, "package:") {
				apkPath = strings.TrimSpace(strings.TrimPrefix(line, "package:"))
				break
			}
		}

		if apkPath == "" {
			fmt.Printf("  [SKIP] %s \u2014 path not found\n", pkg)
			failed++
			continue
		}

		localPath := filepath.Join(destDir, pkg+".apk")
		if err := adb.AdbPull(serial, apkPath, localPath); err != nil {
			fmt.Printf("  [FAIL] %s: %v\n", pkg, err)
			failed++
		} else {
			fmt.Printf("  [OK]   %s\n", pkg)
			ok++
		}
	}

	fmt.Printf("\n[OK] %d/%d APKs extracted \u2192 %s\n", ok, len(packages), destDir)
	if failed > 0 {
		fmt.Printf("     %d skipped/failed\n", failed)
	}
	return nil
}
