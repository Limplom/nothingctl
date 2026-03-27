// Package capture provides screenshot and screen recording for Nothing phones.
package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const maxRecordDuration = 180 // seconds; hard limit imposed by screenrecord

func timestamp() string {
	return time.Now().Format("20060102_150405")
}

// ActionScreenshot captures a screenshot and pulls it to baseDir/screenshots/.
func ActionScreenshot(serial, baseDir string) error {
	ts := timestamp()
	remotePath := fmt.Sprintf("/sdcard/Download/screenshot_%s.png", ts)
	destDir := filepath.Join(baseDir, "screenshots")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	localPath := filepath.Join(destDir, fmt.Sprintf("screenshot_%s.png", ts))

	fmt.Println("  Taking screenshot...")
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "screencap", "-p", remotePath})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("screencap failed: %s", strings.TrimSpace(stderr)))
	}

	pullErr := adb.AdbPull(serial, remotePath, localPath)
	// Always clean up remote file
	adb.Run([]string{"adb", "-s", serial, "shell", "rm", "-f", remotePath})

	if pullErr != nil {
		return pullErr
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return nterrors.AdbError(fmt.Sprintf("Screenshot file not found after pull: %s", localPath))
	}
	sizeKB := float64(info.Size()) / 1024
	fmt.Printf("[OK] Screenshot saved: %s\n", localPath)
	fmt.Printf("     Size: %.1f KB\n", sizeKB)
	return nil
}

var wmSizeRe = regexp.MustCompile(`Physical size:\s*(\d+)x(\d+)`)

// ActionScreenrecord records the screen and pulls the video to baseDir/recordings/.
func ActionScreenrecord(serial, baseDir string, duration int) error {
	if duration > maxRecordDuration {
		fmt.Printf("[WARN] Requested duration %ds exceeds maximum %ds — clamping.\n",
			duration, maxRecordDuration)
		duration = maxRecordDuration
	}

	ts := timestamp()
	remotePath := fmt.Sprintf("/sdcard/Download/screenrecord_%s.mp4", ts)
	destDir := filepath.Join(baseDir, "recordings")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	localPath := filepath.Join(destDir, fmt.Sprintf("screenrecord_%s.mp4", ts))

	// Determine --size arg to avoid encoder failures (e.g. Nothing Phone 1)
	var sizeArgs []string
	sizeOut, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "wm size"})
	if m := wmSizeRe.FindStringSubmatch(sizeOut); m != nil {
		var w, h int
		fmt.Sscanf(m[1], "%d", &w)
		fmt.Sscanf(m[2], "%d", &h)
		if w > 720 {
			h = int(float64(h) * 720 / float64(w))
			w = 720
		}
		sizeArgs = []string{"--size", fmt.Sprintf("%dx%d", w, h)}
	}

	fmt.Printf("  Recording for %d seconds (Ctrl-C to stop early)...\n", duration)

	cmdArgs := []string{"adb", "-s", serial, "shell",
		"screenrecord", "--time-limit", fmt.Sprintf("%d", duration)}
	cmdArgs = append(cmdArgs, sizeArgs...)
	cmdArgs = append(cmdArgs, remotePath)

	timeout := time.Duration(duration+10) * time.Second
	c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	var outBuf, errBuf strings.Builder
	c.Stdout = &outBuf
	c.Stderr = &errBuf

	done := make(chan error, 1)
	if err := c.Start(); err != nil {
		return nterrors.AdbError(fmt.Sprintf("could not start screenrecord: %v", err))
	}
	go func() { done <- c.Wait() }()

	var runErr error
	select {
	case runErr = <-done:
		// command finished
	case <-time.After(timeout):
		c.Process.Kill()
		fmt.Println("[WARN] screenrecord timed out — attempting to pull partial recording.")
		runErr = nil
	}

	if runErr != nil {
		combined := strings.ToLower(outBuf.String() + errBuf.String())
		if strings.Contains(combined, "not found") {
			return nterrors.AdbError(
				"screenrecord is not available on this device or Android version. " +
					"Requires Android 4.4+ and is absent on some low-RAM or Go devices.")
		}
	}

	pullErr := adb.AdbPull(serial, remotePath, localPath)
	// Always clean up remote file
	adb.Run([]string{"adb", "-s", serial, "shell", "rm", "-f", remotePath})

	if pullErr != nil {
		return nterrors.AdbError(fmt.Sprintf("Failed to pull recording: %v", pullErr))
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return nterrors.AdbError(fmt.Sprintf("Recording file not found after pull: %s", localPath))
	}
	sizeMB := float64(info.Size()) / (1024 * 1024)
	fmt.Printf("[OK] Recording saved: %s\n", localPath)
	fmt.Printf("     Size: %.2f MB\n", sizeMB)
	return nil
}
