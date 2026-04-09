// Package adb provides low-level ADB/fastboot wrappers and device interaction helpers.
package adb

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// ---------------------------------------------------------------------------
// Subprocess helpers
// ---------------------------------------------------------------------------

// RunCtx executes an ADB command with the given context, allowing cancellation
// and timeout. stdout, stderr, and exit code are returned.
func RunCtx(ctx context.Context, args []string) (stdout, stderr string, exitCode int) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return outBuf.String(), errBuf.String(), -1
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		return outBuf.String(), errBuf.String(), 1
	}
	return outBuf.String(), errBuf.String(), 0
}

// Run executes args as an external command and returns stdout, stderr, and exit
// code. It never returns a non-nil error for non-zero exit codes; callers
// inspect exitCode directly.
func Run(args []string) (stdout, stderr string, exitCode int) {
	return RunCtx(context.Background(), args)
}

// AdbShell runs `adb -s <serial> shell <cmd>` and returns trimmed stdout.
// Returns an AdbError if the command exits non-zero and stderr is non-empty.
func AdbShell(serial, cmd string) (string, error) {
	stdout, stderr, code := Run([]string{"adb", "-s", serial, "shell", cmd})
	if code != 0 && strings.TrimSpace(stderr) != "" {
		return "", nterrors.AdbError(fmt.Sprintf("adb shell '%s' failed: %s", cmd, strings.TrimSpace(stderr)))
	}
	return strings.TrimSpace(stdout), nil
}

// AdbShellLines runs `adb -s <serial> shell <cmd>` and returns non-empty output
// lines. Each line is fully trimmed (via ParseShellLines).
func AdbShellLines(serial, cmd string) ([]string, error) {
	out, err := AdbShell(serial, cmd)
	if err != nil {
		return nil, err
	}
	return ParseShellLines(out), nil
}

// ParseShellLines splits raw ADB shell output on "\n", strips trailing "\r",
// trims surrounding whitespace, and returns only non-empty lines.
func ParseShellLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

var modelCache sync.Map

// Model returns ro.product.model for the given serial, caching the result
// for the lifetime of the process.
func Model(serial string) string {
	if v, ok := modelCache.Load(serial); ok {
		return v.(string)
	}
	m := Prop(serial, "ro.product.model")
	modelCache.Store(serial, m)
	return m
}

// msysEnv returns a copy of the current environment with MSYS_NO_PATHCONV=1
// appended on Windows so that MSYS/Git-Bash does not mangle device paths.
func msysEnv() []string {
	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, "MSYS_NO_PATHCONV=1")
	}
	return env
}

// AdbPushCtx runs `adb -s <serial> push <localPath> <remotePath>` with the
// given context, allowing cancellation and timeout.
// On Windows, MSYS_NO_PATHCONV=1 is set to prevent path mangling.
func AdbPushCtx(ctx context.Context, serial, localPath, remotePath string) error {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "push", localPath, remotePath)
	cmd.Env = msysEnv()
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nterrors.AdbError(fmt.Sprintf("adb push failed: %s", strings.TrimSpace(errBuf.String())))
	}
	return nil
}

// AdbPush runs `adb -s <serial> push <localPath> <remotePath>`.
// On Windows, MSYS_NO_PATHCONV=1 is set to prevent path mangling.
func AdbPush(serial, localPath, remotePath string) error {
	return AdbPushCtx(context.Background(), serial, localPath, remotePath)
}

// AdbPullCtx runs `adb -s <serial> pull <remotePath> <localPath>` with the
// given context, allowing cancellation and timeout.
// On Windows, MSYS_NO_PATHCONV=1 is set to prevent path mangling.
func AdbPullCtx(ctx context.Context, serial, remotePath, localPath string) error {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "pull", remotePath, localPath)
	cmd.Env = msysEnv()
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nterrors.AdbError(fmt.Sprintf("adb pull '%s' failed: %s", remotePath, strings.TrimSpace(errBuf.String())))
	}
	return nil
}

// AdbPull runs `adb -s <serial> pull <remotePath> <localPath>`.
// On Windows, MSYS_NO_PATHCONV=1 is set to prevent path mangling.
func AdbPull(serial, remotePath, localPath string) error {
	return AdbPullCtx(context.Background(), serial, remotePath, localPath)
}

// ---------------------------------------------------------------------------
// User interaction
// ---------------------------------------------------------------------------

// Confirm prints prompt with "[y/N]: " and returns true only if the user
// types exactly "y". EOF or interrupt counts as "no".
func Confirm(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		ans := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return ans == "y"
	}
	return false
}

// Prompt prints text and reads a single line from stdin.
// Returns the trimmed input or an error if stdin is closed.
func Prompt(text string) (string, error) {
	fmt.Print(text)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("stdin closed")
}

// ---------------------------------------------------------------------------
// Watch loop
// ---------------------------------------------------------------------------

// WatchLoop clears the terminal, calls fn, then waits for the next tick of
// interval before repeating. It exits cleanly on Ctrl-C (SIGINT/SIGTERM).
func WatchLoop(interval time.Duration, fn func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Clear screen: ANSI escape sequence works on all platforms.
		fmt.Print("\033[H\033[2J")
		fn()

		select {
		case <-sigs:
			return
		case <-ticker.C:
		}
	}
}

// ---------------------------------------------------------------------------
// Root check
// ---------------------------------------------------------------------------

// CheckAdbRoot returns true if `adb shell su -c id` succeeds and reports uid=0.
func CheckAdbRoot(serial string) bool {
	stdout, _, code := Run([]string{"adb", "-s", serial, "shell", "su -c id"})
	return code == 0 && strings.Contains(stdout, "uid=0")
}

// ---------------------------------------------------------------------------
// Device serial resolution
// ---------------------------------------------------------------------------

// EnsureDevice resolves which ADB serial to use. If serial is non-empty it is
// returned as-is (after verifying at least one device is reachable). If serial
// is empty and exactly one device is attached its serial is returned. Multiple
// devices without a specified serial is an error.
func EnsureDevice(serial string) (string, error) {
	stdout, _, _ := Run([]string{"adb", "devices", "-l"})

	var lines []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.Contains(line, " device") && !strings.HasPrefix(line, "List") {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return "", nterrors.AdbError("no ADB device found. Check cable and USB debugging.")
	}

	if serial != "" {
		return serial, nil
	}

	if len(lines) > 1 {
		var serials []string
		for _, l := range lines {
			serials = append(serials, strings.Fields(l)[0])
		}
		return "", nterrors.AdbError(fmt.Sprintf("multiple devices found: %v. Use --serial to specify one.", serials))
	}

	return strings.Fields(lines[0])[0], nil
}

// ---------------------------------------------------------------------------
// Convenience helpers — error-ignoring single-value wrappers
// ---------------------------------------------------------------------------

// ShellStr runs a shell command and returns trimmed stdout. Returns "" on
// non-zero exit. Use AdbShell when you need the error.
func ShellStr(serial, cmd string) string {
	stdout, _, code := Run([]string{"adb", "-s", serial, "shell", cmd})
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(stdout, "\r\n"))
}

// Prop reads a single system property via getprop. Returns "" if unset.
func Prop(serial, key string) string {
	return ShellStr(serial, "getprop "+key)
}

// Setting reads a value from the Android settings provider (system/secure/global).
// Returns "" if unset or if the value is the literal string "null".
func Setting(serial, namespace, key string) string {
	val := ShellStr(serial, "settings get "+namespace+" "+key)
	if val == "null" {
		return ""
	}
	return val
}

// PutSetting writes a value to the Android settings provider (system/secure/global).
func PutSetting(serial, namespace, key, value string) error {
	_, stderr, code := Run([]string{"adb", "-s", serial, "shell",
		"settings", "put", namespace, key, value})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("settings put %s %s failed: %s",
			namespace, key, strings.TrimSpace(stderr)))
	}
	return nil
}

// DumpsysPackage runs `dumpsys package <pkg>` and returns the output.
func DumpsysPackage(serial, pkg string) string {
	return ShellStr(serial, "dumpsys package "+pkg)
}
