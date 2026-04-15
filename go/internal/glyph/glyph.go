// Package glyph provides Nothing Glyph Interface control and diagnostics.
//
// High-level responsibilities:
//
//   - detect the Glyph app package (legacy ly.nothing.glyph.service or the
//     newer com.nothing.hearthstone) and its toggle surface;
//   - render the per-device zone map + capability summary (data from the
//     glyph_devices.json catalogue in internal/data);
//   - run patterns against the correct hardware backend via internal/glyph/
//     adapter (sysfs_noth_leds, sysfs_aw210xx, binder_helper, …).
//
// The actual LED hardware calls live in the adapter subpackage; this file
// wires the CLI surface to it.
package glyph

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"github.com/Limplom/nothingctl/internal/glyph/adapter"
	"github.com/Limplom/nothingctl/internal/glyph/profile"
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

// Zone-name → (namespace, settings_key) for zones controllable via settings.
// Used by runTestPattern as a fallback when the adapter can't reach a zone.
var zoneSettingMap = map[string][2]string{
	"Long torch":  {"global", "glyph_long_torch_enable"},
	"Pocket mode": {"global", "glyph_pocket_mode_state"},
}

// ---------------------------------------------------------------------------
// Package / service detection
// ---------------------------------------------------------------------------

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

func isLegacy(pkg string) bool { return pkg == glyphPkgLegacy }

func glyphServiceRunning(serial, pkg string) bool {
	out := adb.ShellStr(serial, fmt.Sprintf(
		"dumpsys activity services %s 2>/dev/null | grep -c ServiceRecord", pkg))
	return out != "" && out != "0"
}

// ---------------------------------------------------------------------------
// Device profile lookup (thin wrapper around internal/glyph/profile)
// ---------------------------------------------------------------------------

// getZonesForModel returns the profile's display name and zone list for the
// given model/codename string. Empty name / nil zones means no profile found.
// Retained for callers that only need the zone list (e.g. status display).
func getZonesForModel(model string) (string, []string) {
	dev, ok := profile.Lookup(model)
	if !ok {
		return "", nil
	}
	return dev.Model, dev.ZoneNames()
}

// ---------------------------------------------------------------------------
// Public actions
// ---------------------------------------------------------------------------

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

	// ---- Status display -----------------------------------------------------
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

	// Zone map + backend summary from profile catalogue.
	if dev, ok := profile.Lookup(model); ok {
		fmt.Printf("\n  Glyph profile  : %s\n", dev.Model)
		fmt.Printf("  Backend        : %s\n", dev.Backend)
		if len(dev.Capabilities) > 0 {
			fmt.Printf("  Capabilities   : %s\n", strings.Join(dev.Capabilities, ", "))
		}
		fmt.Printf("\n  Glyph zones (%d):\n", len(dev.Zones))
		for _, z := range dev.Zones {
			fmt.Printf("    \u2022 %s\n", z.Name)
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

// ---------------------------------------------------------------------------
// Patterns
// ---------------------------------------------------------------------------

// GlyphPatterns lists available pattern names and their descriptions.
var GlyphPatterns = map[string]string{
	"pulse": "Slow pulse all zones",
	"blink": "Fast blink all zones",
	"wave":  "Sequential zone sweep",
	"off":   "Turn all zones off",
	"test":  "Light each zone once in sequence (diagnostic)",
}

// runTestPattern walks every zone of the device, turning each on for a beat
// then off, using whichever backend the profile declares. Zones that require
// an Android settings toggle (Long torch / Pocket mode) fall back to a
// settings put when the adapter returns ErrUnsupported or doesn't know the
// zone.
func runTestPattern(serial, model string) {
	dev, ok := profile.Lookup(model)
	if !ok {
		fmt.Printf("[WARN] No profile found for model '%s' — skipping test pattern.\n", model)
		return
	}

	fmt.Printf("\n  Running test pattern on %s (%d zones, backend=%s)...\n",
		dev.Model, len(dev.Zones), dev.Backend)

	ad, adErr := adapter.For(serial, dev)
	if adErr != nil {
		fmt.Printf("[WARN] %s — falling back to settings-only zones where possible.\n", adErr)
	}

	for _, z := range dev.Zones {
		if ad != nil && tryZoneBeat(ad, z.Name) {
			continue
		}
		if keyInfo, ok := zoneSettingMap[z.Name]; ok {
			ns, key := keyInfo[0], keyInfo[1]
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s 1", ns, key)})
			fmt.Printf("    [ON]  %s (via settings)\n", z.Name)
			time.Sleep(800 * time.Millisecond)
			adb.Run([]string{"adb", "-s", serial, "shell",
				fmt.Sprintf("settings put %s %s 0", ns, key)})
			fmt.Printf("    [OFF] %s\n", z.Name)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		fmt.Printf("    %s: (no direct control available)\n", z.Name)
	}
	fmt.Println("[OK] Test pattern complete.")
}

// tryZoneBeat runs On(80%) → wait → Off on a zone, printing user-facing
// status. Returns false if the adapter can't drive this zone (caller then
// tries a fallback); returns true otherwise (regardless of whether an
// individual write failed, the zone has been "attempted").
func tryZoneBeat(ad adapter.Adapter, zone string) bool {
	if err := ad.On(zone, 80); err != nil {
		if errors.Is(err, adapter.ErrUnsupported) {
			return false
		}
		fmt.Printf("    [ERR] %s: %s\n", zone, err)
		return true
	}
	fmt.Printf("    [ON]  %s\n", zone)
	time.Sleep(800 * time.Millisecond)
	if err := ad.Off(zone); err != nil {
		fmt.Printf("    [ERR] off %s: %s\n", zone, err)
	} else {
		fmt.Printf("    [OFF] %s\n", zone)
	}
	time.Sleep(300 * time.Millisecond)
	return true
}

func runOffPattern(serial, model string) {
	if dev, ok := profile.Lookup(model); ok {
		if ad, err := adapter.For(serial, dev); err == nil {
			if err := ad.OffAll(); err != nil {
				fmt.Printf("  [WARN] adapter OffAll: %s\n", err)
			} else {
				fmt.Printf("  [OFF] all %d zone(s) via %s\n", len(dev.Zones), dev.Backend)
			}
		} else {
			fmt.Printf("  [WARN] no adapter for %s: %s\n", dev.Model, err)
		}
	}
	// Turn off settings-based zones regardless of profile — these are Android-
	// level features that every Glyph device exposes.
	for _, gs := range settingsNew {
		adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("settings put %s %s 0", gs.ns, gs.key)})
		fmt.Printf("  [OFF] %s\n", gs.label)
	}
	fmt.Println("[OK] All known Glyph zones set to off.")
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
		runOffPattern(serial, model)
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
