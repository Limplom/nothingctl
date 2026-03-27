// Package diagnostics provides logcat, bugreport, and ANR/tombstone collection.
package diagnostics

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

func timestamp() string {
	return time.Now().Format("20060102_150405")
}

// ActionLogcat dumps the current logcat buffer to a local file with optional filters.
func ActionLogcat(serial, baseDir, packageName, tag, level string, lines int) error {
	// Build filter spec
	filterSpec := "*:V"
	if tag != "" && level != "" {
		filterSpec = tag + ":" + strings.ToUpper(level) + " *:S"
	} else if tag != "" {
		filterSpec = tag + ":V *:S"
	} else if level != "" {
		filterSpec = "*:" + strings.ToUpper(level)
	}

	pidFilter := ""
	if packageName != "" {
		pidOut, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pidof " + packageName})
		pidParts := strings.Fields(strings.TrimSpace(pidOut))
		if len(pidParts) > 0 {
			pidFilter = "--pid=" + pidParts[0]
			fmt.Printf("  Package %s \u2192 PID %s\n", packageName, pidParts[0])
		} else {
			fmt.Printf("  [WARN] %s not running \u2014 capturing full buffer\n", packageName)
		}
	}

	ts := timestamp()
	destDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	label := packageName
	if label == "" {
		label = tag
	}
	if label == "" {
		label = "full"
	}
	dest := filepath.Join(destDir, fmt.Sprintf("logcat_%s_%s.txt", label, ts))

	cmd := []string{"adb", "-s", serial, "logcat", "-d", "-v", "threadtime", "-t", fmt.Sprintf("%d", lines)}
	if pidFilter != "" {
		cmd = append(cmd, pidFilter)
	}
	cmd = append(cmd, filterSpec)

	fmt.Printf("  Capturing logcat (max %d lines, filter: %s)...\n", lines, filterSpec)
	stdout, _, _ := adb.Run(cmd)

	if strings.TrimSpace(stdout) == "" {
		fmt.Println("  [WARN] Empty logcat \u2014 buffer may have been cleared.")
		return nil
	}

	if err := os.WriteFile(dest, []byte(stdout), 0644); err != nil {
		return err
	}
	lineCount := strings.Count(stdout, "\n")
	fmt.Printf("[OK] %d lines \u2192 %s\n", lineCount, dest)
	return nil
}

// ActionBugreport triggers adb bugreport and saves the ZIP to baseDir/bugreports/.
func ActionBugreport(serial, baseDir string) error {
	ts := timestamp()
	destDir := filepath.Join(baseDir, "bugreports")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	dest := filepath.Join(destDir, fmt.Sprintf("bugreport_%s_%s.zip", serial, ts))

	fmt.Println("  Generating bugreport (this takes 30\u201390 seconds)...")
	fmt.Printf("  Saving to: %s\n", dest)

	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "bugreport", dest})

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return nterrors.AdbError(fmt.Sprintf("Bugreport not created.\nOutput: %s",
			strings.TrimSpace(stdout+stderr)))
	}
	_ = code
	info, err := os.Stat(dest)
	if err != nil {
		return err
	}
	sizeMB := float64(info.Size()) / 1024 / 1024
	fmt.Printf("[OK] Bugreport saved \u2014 %.1f MB \u2192 %s\n", sizeMB, dest)
	fmt.Printf("     Open the ZIP with any archive manager or 'unzip -l \"%s\"' on macOS/Linux\n", dest)
	return nil
}

// ActionANRDump pulls ANR traces and tombstones from the device (requires root).
func ActionANRDump(serial, baseDir string) error {
	ts := timestamp()
	dest := filepath.Join(baseDir, "diagnostics", ts)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	sources := []struct{ path, label string }{
		{"/data/anr", "anr"},
		{"/data/tombstones", "tombstones"},
	}

	anyFound := false

	for _, src := range sources {
		countOut, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("su -c 'ls %s/ 2>/dev/null | wc -l'", src.path)})
		var count int
		fmt.Sscanf(strings.TrimSpace(countOut), "%d", &count)

		if count == 0 {
			fmt.Printf("  %-12s: empty (no crashes recorded)\n", src.label)
			continue
		}
		fmt.Printf("  %-12s: %d file(s) \u2014 copying...\n", src.label, count)

		tmp := fmt.Sprintf("%s/%s_dump_%s", remoteTmp, src.label, ts)
		r2out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("su -c 'cp -r %s %s && chmod -R 644 %s/* 2>/dev/null && echo __OK__'",
				src.path, tmp, tmp)})

		if !strings.Contains(r2out, "__OK__") {
			fmt.Printf("  [WARN] Could not copy %s (root needed?)\n", src.path)
			continue
		}

		localDest := filepath.Join(dest, src.label)
		if err := os.MkdirAll(localDest, 0755); err != nil {
			fmt.Printf("  [WARN] mkdir failed: %v\n", err)
			continue
		}

		if err := adb.AdbPull(serial, tmp, localDest); err != nil {
			fmt.Printf("  [WARN] Pull failed: %v\n", err)
		} else {
			anyFound = true
			fmt.Printf("         saved \u2192 %s/\n", localDest)
		}
		adb.Run([]string{"adb", "-s", serial, "shell", "su -c 'rm -rf " + tmp + "'"})
	}

	if anyFound {
		fmt.Printf("\n[OK] Diagnostics saved \u2192 %s\n", dest)
	} else {
		fmt.Println("\n[OK] No ANR traces or tombstones found \u2014 device is clean.")
		os.Remove(dest)
	}
	return nil
}
