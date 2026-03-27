// Package display provides display settings and color profile management.
package display

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

func getSetting(serial, namespace, key string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "settings", "get", namespace, key})
	val := strings.TrimSpace(strings.TrimRight(stdout, "\r\n"))
	if val == "null" || val == "null\r" {
		return ""
	}
	return val
}

func putSetting(serial, namespace, key, value string) error {
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "settings", "put", namespace, key, value})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("settings put %s %s failed: %s", namespace, key, strings.TrimSpace(stderr)))
	}
	return nil
}

func fmtTimeout(msStr string) string {
	var ms int
	if _, err := fmt.Sscanf(msStr, "%d", &ms); err != nil {
		if msStr != "" {
			return msStr
		}
		return "n/a"
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes == 1 {
		return "1 min"
	}
	return fmt.Sprintf("%d min", minutes)
}

func fmtRotation(val string) string {
	m := map[string]string{"0": "portrait", "1": "landscape", "2": "reverse-portrait", "3": "reverse-landscape"}
	if l, ok := m[val]; ok {
		return l
	}
	if val == "" {
		return "n/a"
	}
	return val
}

func fmtOnOff(val string) string {
	if val == "1" {
		return "on"
	}
	if val == "0" {
		return "off"
	}
	if val == "" {
		return "n/a"
	}
	return val
}

var wmSizePhysRe = regexp.MustCompile(`Physical size:\s*(\d+x\d+)`)
var wmSizeRe = regexp.MustCompile(`(\d+x\d+)`)
var wmDensityRe = regexp.MustCompile(`(?:Override|Physical) density:\s*(\d+)`)
var wmDensitySimpleRe = regexp.MustCompile(`(\d+)`)
var refreshRateRe = regexp.MustCompile(`(?:mRefreshRate|refreshRate)[=:\s"]+([0-9]+(?:\.[0-9]+)?)`)

func parseWMSize(raw string) string {
	if m := wmSizePhysRe.FindStringSubmatch(raw); m != nil {
		return m[1] + " (physical)"
	}
	if m := wmSizeRe.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	if raw == "" {
		return "n/a"
	}
	return raw
}

func parseWMDensity(raw string) string {
	if m := wmDensityRe.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	if m := wmDensitySimpleRe.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	if raw == "" {
		return "n/a"
	}
	return raw
}

func parseRefreshRate(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		if m := refreshRateRe.FindStringSubmatch(line); m != nil {
			return m[1] + " Hz"
		}
	}
	return "n/a"
}

func colorProfileLabel(val string) string {
	m := map[string]string{"0": "Natural (sRGB)", "1": "Vivid (P3)", "256": "Custom"}
	if l, ok := m[val]; ok {
		return l
	}
	if val == "" {
		return "n/a"
	}
	return fmt.Sprintf("Unknown (%s)", val)
}

var displaySettings = map[string][2]string{
	"brightness":      {"system", "screen_brightness"},
	"brightness_auto": {"system", "screen_brightness_mode"},
	"timeout":         {"system", "screen_off_timeout"},
	"rotation":        {"system", "user_rotation"},
	"rotation_auto":   {"system", "accelerometer_rotation"},
	"font_scale":      {"system", "font_scale"},
}

// ActionDisplay reads or sets display settings.
func ActionDisplay(serial, model, key, value string) error {
	if key != "" && value != "" {
		key = strings.ToLower(key)
		if key == "dpi" {
			_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "wm", "density", value})
			if code != 0 {
				return nterrors.AdbError(fmt.Sprintf("wm density %s failed: %s", value, strings.TrimSpace(stderr)))
			}
			fmt.Printf("  DPI set to %s on %s.\n", value, model)
			return nil
		}
		ns, ok := displaySettings[key]
		if !ok {
			keys := []string{"dpi"}
			for k := range displaySettings {
				keys = append(keys, k)
			}
			return nterrors.AdbError(fmt.Sprintf("Unknown display key '%s'. Valid keys: %s", key, strings.Join(keys, ", ")))
		}
		if err := putSetting(serial, ns[0], ns[1], value); err != nil {
			return err
		}
		fmt.Printf("  %s set to %s on %s.\n", key, value, model)
		return nil
	}

	sizeRaw := adb.ShellStr(serial, "wm size")
	resolution := parseWMSize(sizeRaw)
	densityRaw := adb.ShellStr(serial, "wm density")
	dpi := parseWMDensity(densityRaw)
	displayDump := adb.ShellStr(serial, "dumpsys display | grep -E 'mRefreshRate|refreshRate'")
	refreshRate := parseRefreshRate(displayDump)

	brightness := getSetting(serial, "system", "screen_brightness")
	brightnessMode := getSetting(serial, "system", "screen_brightness_mode")
	timeoutRaw := getSetting(serial, "system", "screen_off_timeout")
	fontScale := getSetting(serial, "system", "font_scale")
	rotationVal := getSetting(serial, "system", "user_rotation")
	rotationAuto := getSetting(serial, "system", "accelerometer_rotation")

	brightnessDisplay := "n/a"
	if brightness != "" {
		brightnessDisplay = fmt.Sprintf("%s / 255 (auto: %s)", brightness, fmtOnOff(brightnessMode))
	}
	timeoutDisplay := "n/a"
	if timeoutRaw != "" {
		timeoutDisplay = fmtTimeout(timeoutRaw)
	}
	fontDisplay := fontScale
	if fontDisplay == "" {
		fontDisplay = "n/a"
	}
	rotationDisplay := fmtRotation(rotationVal)
	if rotationAuto != "" {
		rotationDisplay += fmt.Sprintf(" (auto: %s)", fmtOnOff(rotationAuto))
	}

	fmt.Printf("\n  Display \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %s\n", "Resolution", resolution)
	fmt.Printf("  %-16s: %s\n", "DPI", dpi)
	fmt.Printf("  %-16s: %s\n", "Refresh Rate", refreshRate)
	fmt.Println()
	fmt.Printf("  %-16s: %s\n", "Brightness", brightnessDisplay)
	fmt.Printf("  %-16s: %s\n", "Font Scale", fontDisplay)
	fmt.Printf("  %-16s: %s\n", "Rotation", rotationDisplay)
	fmt.Printf("  %-16s: %s\n", "Screen Timeout", timeoutDisplay)
	fmt.Println()
	return nil
}

var profileAliases = map[string]string{
	"natural": "0",
	"vivid":   "1",
	"custom":  "256",
}

// ActionColorProfile reads or sets the display color profile.
func ActionColorProfile(serial, model, profile string) error {
	if profile != "" {
		canonical, ok := profileAliases[strings.ToLower(profile)]
		if !ok {
			canonical = profile
		}
		if err := putSetting(serial, "system", "display_color_mode", canonical); err != nil {
			return err
		}
		label := colorProfileLabel(canonical)
		fmt.Printf("  Color profile set to %s on %s.\n", label, model)
		return nil
	}

	colorMode := getSetting(serial, "system", "display_color_mode")
	nightActive := getSetting(serial, "secure", "night_display_activated")
	nightTemp := getSetting(serial, "secure", "night_display_color_temperature")

	modeDisplay := "n/a"
	if colorMode != "" {
		modeDisplay = fmt.Sprintf("%s (%s)", colorProfileLabel(colorMode), colorMode)
	}
	nightDisplay := fmtOnOff(nightActive)

	fmt.Printf("\n  Color Profile \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %s\n", "Color Mode", modeDisplay)
	fmt.Printf("  %-16s: %s\n", "Night Light", nightDisplay)
	if nightActive == "1" && nightTemp != "" {
		fmt.Printf("  %-16s: %s K\n", "Color Temp", nightTemp)
	}
	fmt.Println()
	return nil
}
