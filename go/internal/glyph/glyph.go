// Package glyph provides Nothing Glyph Interface control and diagnostics.
package glyph

import (
	"fmt"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const (
	glyphPkgLegacy = "ly.nothing.glyph.service"
	glyphPkgNew    = "com.nothing.hearthstone"
)

var settingLegacy = [2]string{"secure", "glyph_interface_enable"}

type glyphSetting struct {
	ns, key, label string
}

var settingsNew = []glyphSetting{
	{"global", "glyph_long_torch_enable", "Long torch"},
	{"global", "glyph_pocket_mode_state", "Pocket mode"},
	{"global", "glyph_screen_upward_state", "Screen-upward mode"},
}

// glyphZones maps model/codename key to zone list
var glyphZones = map[string][]string{
	"Phone (1)":       {"Camera", "Diagonal", "Battery dot", "Battery bar", "USB"},
	"spacewar":        {"Camera", "Diagonal", "Battery dot", "Battery bar", "USB"},
	"A063":            {"Camera", "Diagonal", "Battery dot", "Battery bar", "USB"},
	"Phone (2)":       {"Camera top", "Camera bottom", "Diagonal", "Battery left", "Battery right", "USB", "Notification"},
	"pong":            {"Camera top", "Camera bottom", "Diagonal", "Battery left", "Battery right", "USB", "Notification"},
	"Phone (2a)":      {"Camera", "Battery", "Bottom strip"},
	"pacman":          {"Camera", "Battery", "Bottom strip"},
	"Phone (3a) Lite": {"Camera", "Bottom strip"},
	"Phone (3a)":      {"Camera top", "Camera bottom", "Battery", "Bottom strip"},
	"galaxian":        {"Camera top", "Camera bottom", "Battery", "Bottom strip"},
	"A001":            {"Camera top", "Camera bottom", "Battery", "Bottom strip"},
	"A001T":           {"Camera", "Bottom strip"},
	"CMF Phone 1":     {"Ring", "Dot"},
}

// Zone name → (namespace, settings_key) for zones controllable via settings
var zoneSettingMap = map[string][2]string{
	"Long torch":  {"global", "glyph_long_torch_enable"},
	"Pocket mode": {"global", "glyph_pocket_mode_state"},
}


func detectPkg(serial string) string {
	for _, pkg := range []string{glyphPkgNew, glyphPkgLegacy} {
		out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("pm list packages %s", pkg)})
		if strings.Contains(out, pkg) {
			return pkg
		}
	}
	return ""
}

func isLegacy(pkg string) bool {
	return pkg == glyphPkgLegacy
}

func glyphServiceRunning(serial, pkg string) bool {
	out := adb.ShellStr(serial, fmt.Sprintf(
		"dumpsys activity services %s 2>/dev/null | grep -c ServiceRecord", pkg))
	return out != "" && out != "0"
}

func getZonesForModel(model string) (string, []string) {
	modelLower := strings.ToLower(model)
	for key, zones := range glyphZones {
		if strings.Contains(modelLower, strings.ToLower(key)) {
			return key, zones
		}
	}
	return "", nil
}

// ActionGlyph shows Glyph diagnostics or toggles the interface.
// enable="" means status only; "on"/"off" to toggle.
func ActionGlyph(serial, model, enable string) error {
	pkg := detectPkg(serial)
	if pkg == "" {
		return nterrors.AdbError(fmt.Sprintf(
			"No Glyph package found (%s or %s).\n"+
				"This device may not support the Glyph interface, or "+
				"the package was removed via debloat.", glyphPkgNew, glyphPkgLegacy))
	}

	if enable != "" {
		isOn := strings.ToLower(enable) == "on" || enable == "1" || strings.ToLower(enable) == "true"

		if isLegacy(pkg) {
			ns := settingLegacy[0]
			key := settingLegacy[1]
			val := "0"
			if isOn {
				val = "1"
			}
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s %s", ns, key, val)})
		} else {
			var svcCmd string
			if isOn {
				svcCmd = "su -c 'am startservice com.nothing.thirdparty/.GlyphService'"
			} else {
				svcCmd = "su -c 'am stopservice com.nothing.thirdparty/.GlyphService'"
			}
			_, _, code := adb.Run([]string{"adb", "-s", serial, "shell", svcCmd})
			if code != 0 {
				fmt.Println("[WARN] GlyphService toggle may have failed (needs root).")
				fmt.Println("       Fallback: use the Glyphs tile in Quick Settings.")
				return nil
			}
		}
		label := "disabled"
		if isOn {
			label = "enabled"
		}
		fmt.Printf("[OK] Glyph interface %s.\n", label)
		fmt.Println("     Changes take effect immediately (no reboot needed).")
		return nil
	}

	// Status display
	running := glyphServiceRunning(serial, pkg)
	fmt.Printf("\n  Glyph package  : %s\n", pkg)
	stateStr := "not running"
	if running {
		stateStr = "running"
	}
	fmt.Printf("  Service state  : %s\n", stateStr)

	if isLegacy(pkg) {
		ns := settingLegacy[0]
		key := settingLegacy[1]
		out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("settings get %s %s", ns, key)})
		val := strings.TrimSpace(out)
		stateLabel := map[string]string{"1": "ENABLED", "0": "DISABLED"}
		s, ok := stateLabel[val]
		if !ok {
			s = fmt.Sprintf("unknown (%s)", val)
		}
		fmt.Printf("  Interface      : %s\n", s)
	} else {
		fmt.Println("\n  Glyph feature settings:")
		for _, gs := range settingsNew {
			out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings get %s %s", gs.ns, gs.key)})
			val := strings.TrimSpace(out)
			s, ok := map[string]string{"1": "on", "0": "off"}[val]
			if !ok {
				s = fmt.Sprintf("unknown (%s)", val)
			}
			fmt.Printf("    %-26s %s\n", gs.label, s)
		}
		fmt.Println("\n  [INFO] Main on/off toggle: Glyphs Quick Settings tile")
	}

	// Zone map
	modelKey, zones := getZonesForModel(model)
	if modelKey != "" {
		fmt.Printf("\n  Glyph zones on %s (%d):\n", modelKey, len(zones))
		for _, z := range zones {
			fmt.Printf("    \u2022 %s\n", z)
		}
	} else {
		fmt.Printf("\n  Zone map: not available for model '%s'\n", model)
	}

	fmt.Println("\n  Toggle:")
	fmt.Println("    nothingctl glyph --enable on")
	fmt.Println("    nothingctl glyph --enable off")
	if !isLegacy(pkg) {
		fmt.Println("    (or use the Glyphs tile in Quick Settings)")
	}
	return nil
}

// GlyphPatterns lists available pattern names and their descriptions.
var GlyphPatterns = map[string]string{
	"pulse": "Slow pulse all zones",
	"blink": "Fast blink all zones",
	"wave":  "Sequential zone sweep",
	"off":   "Turn all zones off",
	"test":  "Light each zone once in sequence (diagnostic)",
}

func runTestPattern(serial, model string) {
	_, zones := getZonesForModel(model)
	if zones == nil {
		fmt.Printf("[WARN] No zone map found for model '%s' — skipping test pattern.\n", model)
		return
	}

	fmt.Printf("\n  Running test pattern on %s (%d zones)...\n", model, len(zones))
	for _, zone := range zones {
		if keyInfo, ok := zoneSettingMap[zone]; ok {
			ns, key := keyInfo[0], keyInfo[1]
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s 1", ns, key)})
			fmt.Printf("    [ON]  %s\n", zone)
			time.Sleep(500 * time.Millisecond)
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s 0", ns, key)})
			fmt.Printf("    [OFF] %s\n", zone)
		} else {
			fmt.Printf("    %s: (direct control not available via ADB settings)\n", zone)
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println("[OK] Test pattern complete.")
}

func runOffPattern(serial string) {
	for _, gs := range settingsNew {
		adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("settings put %s %s 0", gs.ns, gs.key)})
		fmt.Printf("  [OFF] %s\n", gs.label)
	}
	// Additional zone-mapped keys not in settingsNew
	for zone, keyInfo := range zoneSettingMap {
		alreadySet := false
		for _, gs := range settingsNew {
			if gs.key == keyInfo[1] {
				alreadySet = true
				break
			}
		}
		if !alreadySet {
			ns, key := keyInfo[0], keyInfo[1]
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s 0", ns, key)})
			fmt.Printf("  [OFF] %s\n", zone)
		}
	}
	fmt.Println("[OK] All known Glyph settings set to off.")
}

// ActionGlyphPattern displays available patterns or triggers a named pattern.
// pattern="" means list mode.
func ActionGlyphPattern(serial, model, pattern string) error {
	if pattern == "" {
		fmt.Printf("\n  Glyph Patterns \u2014 Nothing %s\n", model)
		fmt.Println("\n  Available patterns:")
		for _, name := range []string{"test", "off"} {
			fmt.Printf("    %-8s \u2014 %s\n", name, GlyphPatterns[name])
		}
		fmt.Println("\n  Custom patterns require the Nothing Glyph Composer app.")
		fmt.Println("  For advanced patterns, see: https://github.com/nothing-open-source/glyphify")
		fmt.Println("\n  Use: nothingctl glyph-pattern --pattern test")
		fmt.Println("       nothingctl glyph-pattern --pattern off")
		return nil
	}

	p := strings.ToLower(strings.TrimSpace(pattern))

	switch p {
	case "test":
		runTestPattern(serial, model)
	case "off":
		fmt.Printf("\n  Turning all Glyph zones off on %s...\n", model)
		runOffPattern(serial)
	default:
		if _, ok := GlyphPatterns[p]; ok {
			fmt.Printf("\n  [WARN] Pattern '%s' (%s) requires\n", p, GlyphPatterns[p])
			fmt.Println("         the Nothing Glyph Composer app or Glyph SDK.")
			fmt.Println("         See: https://github.com/nothing-open-source/glyphify")
			fmt.Println("\n  Running 'test' pattern as fallback...")
			runTestPattern(serial, model)
		} else {
			known := strings.Join([]string{"pulse", "blink", "wave", "off", "test"}, ", ")
			return nterrors.AdbError(fmt.Sprintf(
				"Unknown Glyph pattern '%s'. Available patterns: %s", pattern, known))
		}
	}
	return nil
}
