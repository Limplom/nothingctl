// Package inputctl provides input event control for Nothing phones.
package inputctl

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

type keycode struct {
	name string
	code int
}

var keycodes = []keycode{
	{"KEYCODE_HOME", 3},
	{"KEYCODE_BACK", 4},
	{"KEYCODE_POWER", 26},
	{"KEYCODE_VOLUME_UP", 24},
	{"KEYCODE_VOLUME_DOWN", 25},
	{"KEYCODE_WAKEUP", 224},
	{"KEYCODE_SLEEP", 223},
	{"KEYCODE_MENU", 82},
	{"KEYCODE_CAMERA", 27},
	{"KEYCODE_MEDIA_PLAY_PAUSE", 85},
	{"KEYCODE_BRIGHTNESS_UP", 221},
	{"KEYCODE_BRIGHTNESS_DOWN", 220},
}

func printKeycodeReference() {
	fmt.Println("\n  Android Keycode Reference\n")
	fmt.Printf("  %-30s  %4s\n", "Keycode name", "Code")
	fmt.Printf("  %-30s  %4s\n", strings.Repeat("-", 30), "----")
	for _, k := range keycodes {
		fmt.Printf("  %-30s  %4d\n", k.name, k.code)
	}
	fmt.Println()
	fmt.Println("  Usage:  --keyevent KEYCODE_HOME  or  --keyevent 3")
	fmt.Println()
}

// escapeInputText wraps text in single quotes and escapes internal single quotes.
func escapeInputText(text string) string {
	escaped := strings.ReplaceAll(text, "'", "'\\''")
	return "'" + escaped + "'"
}

// ActionInput sends input events to the device via ADB.
// Empty strings mean "not provided". If all are empty, prints keycode reference.
func ActionInput(serial, model, tap, swipe, text, keyevent string) error {
	if tap == "" && swipe == "" && text == "" && keyevent == "" {
		printKeycodeReference()
		return nil
	}

	if tap != "" {
		parts := strings.Split(tap, ",")
		if len(parts) != 2 {
			return nterrors.AdbError(fmt.Sprintf(
				"Invalid tap format '%s'. Expected 'x,y' (e.g. '540,1200').", tap))
		}
		x := strings.TrimSpace(parts[0])
		y := strings.TrimSpace(parts[1])
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "input", "tap", x, y})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("input tap failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Printf("  Tap sent: (%s, %s)  [%s]\n", x, y, model)
	}

	if swipe != "" {
		parts := strings.Split(swipe, ",")
		switch len(parts) {
		case 4:
			x1 := strings.TrimSpace(parts[0])
			y1 := strings.TrimSpace(parts[1])
			x2 := strings.TrimSpace(parts[2])
			y2 := strings.TrimSpace(parts[3])
			_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
				"input", "swipe", x1, y1, x2, y2})
			if code != 0 {
				return nterrors.AdbError(fmt.Sprintf("input swipe failed: %s", strings.TrimSpace(stderr)))
			}
			fmt.Printf("  Swipe sent: (%s,%s) -> (%s,%s)  [%s]\n", x1, y1, x2, y2, model)
		case 5:
			x1 := strings.TrimSpace(parts[0])
			y1 := strings.TrimSpace(parts[1])
			x2 := strings.TrimSpace(parts[2])
			y2 := strings.TrimSpace(parts[3])
			dur := strings.TrimSpace(parts[4])
			_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
				"input", "swipe", x1, y1, x2, y2, dur})
			if code != 0 {
				return nterrors.AdbError(fmt.Sprintf("input swipe failed: %s", strings.TrimSpace(stderr)))
			}
			fmt.Printf("  Swipe sent: (%s,%s) -> (%s,%s), duration %s ms  [%s]\n",
				x1, y1, x2, y2, dur, model)
		default:
			return nterrors.AdbError(fmt.Sprintf(
				"Invalid swipe format '%s'. Expected 'x1,y1,x2,y2' or 'x1,y1,x2,y2,duration_ms'.", swipe))
		}
	}

	if text != "" {
		escaped := escapeInputText(text)
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"input text " + escaped})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("input text failed: %s", strings.TrimSpace(stderr)))
		}
		preview := text
		if len(preview) > 40 {
			preview = preview[:40] + "\u2026"
		}
		fmt.Printf("  Text sent: %q  [%s]\n", preview, model)
	}

	if keyevent != "" {
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"input", "keyevent", keyevent})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("input keyevent failed: %s", strings.TrimSpace(stderr)))
		}
		fmt.Printf("  Keyevent sent: %s  [%s]\n", keyevent, model)
	}

	return nil
}
