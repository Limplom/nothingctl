package glyph

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

const (
	glyphNotifyPkg = "com.nothing.glyphnotification"
)

var glyphGlobalKeys = []struct{ key, label string }{
	{"glyph_long_torch_enable", "Long torch"},
	{"glyph_pocket_mode_state", "Pocket mode"},
	{"glyph_screen_upward_state", "Screen-upward mode"},
	{"nt_glyph_interface_debug_enable", "Glyph debug mode"},
}

var hearthstoneGlyphServices = []string{
	"GlyphService",
	"GlyphComposer",
	"GlyphManagerService",
}

func pkgInstalled(serial, pkg string) bool {
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("pm list packages %s", pkg)})
	return strings.Contains(out, pkg)
}

func getGlyphSettings(serial string) []struct{ label, key, val string } {
	var results []struct{ label, key, val string }
	for _, gk := range glyphGlobalKeys {
		val := adb.Setting(serial, "global", gk.key)
		if val != "" {
			results = append(results, struct{ label, key, val string }{gk.label, gk.key, val})
		}
	}
	return results
}

func getHearthstoneServices(serial string) []string {
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys activity services com.nothing.hearthstone 2>/dev/null"})
	var services []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(strings.TrimSpace(line), "\r")
		if !strings.HasPrefix(line, "* ServiceRecord") {
			continue
		}
		parts := strings.Fields(line)
		for _, part := range parts {
			if strings.Contains(part, "com.nothing.hearthstone/") {
				cls := strings.TrimRight(part, "}")
				short := cls
				if i := strings.LastIndex(cls, "."); i >= 0 {
					short = cls[i+1:]
				}
				services = append(services, short)
				break
			}
		}
	}
	return services
}

type glyphNotifyInfo struct {
	installed         bool
	channelImportance string
}

func getGlyphNotifyInfo(serial string) glyphNotifyInfo {
	info := glyphNotifyInfo{}
	if !pkgInstalled(serial, glyphNotifyPkg) {
		return info
	}
	info.installed = true

	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys notification 2>/dev/null | grep -A 12 'com.nothing.glyphnotification'"})

	impMap := map[string]string{"1": "MIN", "2": "LOW", "3": "DEFAULT", "4": "HIGH", "5": "MAX"}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(strings.TrimSpace(line), "\r")
		if strings.Contains(line, "mId='Glyph'") && strings.Contains(line, "mImportance=") {
			parts := strings.SplitN(line, "mImportance=", 2)
			if len(parts) == 2 {
				impStr := strings.SplitN(parts[1], ",", 2)[0]
				if label, ok := impMap[impStr]; ok {
					info.channelImportance = label
				} else {
					info.channelImportance = impStr
				}
			}
		}
	}
	return info
}

// ActionGlyphNotify shows Glyph notification configuration.
func ActionGlyphNotify(serial, model string) error {
	fmt.Printf("\n  Glyph Notifications \u2014 %s (%s)\n", model, serial)

	// 1. Glyph settings keys
	gs := getGlyphSettings(serial)
	if len(gs) > 0 {
		fmt.Println("\n  Glyph settings:")
		for _, s := range gs {
			stateMap := map[string]string{"1": "on", "0": "off"}
			state := s.val
			if v, ok := stateMap[s.val]; ok {
				state = v
			}
			fmt.Printf("    %-34s %s  (global/%s)\n", s.label, state, s.key)
		}
	} else {
		fmt.Println("\n  Glyph settings: none found on this device")
	}

	// 2. GlyphNotification package
	fmt.Printf("\n  Glyph Notification package (%s):\n", glyphNotifyPkg)
	nInfo := getGlyphNotifyInfo(serial)
	if nInfo.installed {
		imp := nInfo.channelImportance
		if imp == "" {
			imp = "unknown"
		}
		fmt.Printf("    %-23s: yes\n", "Installed")
		fmt.Printf("    %-23s: Glyph  (importance=%s)\n", "Notification channel", imp)
	} else {
		fmt.Printf("    %-23s: no  (package not present on this device)\n", "Installed")
	}

	// 3. Hearthstone services
	fmt.Printf("\n  Active Hearthstone services (%s):\n", glyphPkgNew)
	services := getHearthstoneServices(serial)
	if len(services) > 0 {
		for _, svc := range services {
			fmt.Printf("    \u2022 %s\n", svc)
		}
	} else {
		fmt.Println("    (none running)")
	}

	// 4. Notification listeners
	fmt.Println("\n  Notification listeners (enabled_notification_listeners):")
	listenersVal := adb.Setting(serial, "secure", "enabled_notification_listeners")
	if listenersVal != "" {
		for _, entry := range strings.Split(listenersVal, ":") {
			entry = strings.TrimSpace(entry)
			pkg := entry
			if i := strings.Index(entry, "/"); i >= 0 {
				pkg = entry[:i]
			}
			fmt.Printf("    \u2022 %s\n", pkg)
		}
	} else {
		fmt.Println("    (none)")
	}

	// 5. Glyph packages check
	fmt.Println("\n  Glyph packages:")
	for _, item := range []struct{ pkg, label string }{
		{glyphPkgNew, "Hearthstone (new, Phone 2a/3a/3a Lite)"},
		{glyphPkgLegacy, "Glyph service (legacy, Phone 1/2)"},
		{glyphNotifyPkg, "Glyph notification controller"},
	} {
		marker := "[-]"
		if pkgInstalled(serial, item.pkg) {
			marker = "[+]"
		}
		fmt.Printf("    %s %-46s (%s)\n", marker, item.label, item.pkg)
	}

	// 6. Deeper data notice
	fmt.Println()
	fmt.Println("  [INFO] Per-app Glyph lighting rules are stored inside Hearthstone's")
	fmt.Println("         private database (/data/data/com.nothing.hearthstone/databases/).")
	fmt.Println("         Reading that data requires root access or a backup extraction.")
	fmt.Println("         The ContentProvider content://com.nothing.hearthstone.provider/")
	fmt.Println("         is not publicly accessible without root.")
	return nil
}
