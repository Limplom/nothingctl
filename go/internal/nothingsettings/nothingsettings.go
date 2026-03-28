// Package nothingsettings provides Nothing-specific Android settings management.
package nothingsettings

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

type knownKey struct {
	namespace, key, label, hint string
}

var knownKeys = []knownKey{
	// Glyph
	{"global", "glyph_long_torch_enable", "Glyph long torch", "all"},
	{"global", "nt_glyph_interface_debug_enable", "Glyph debug mode", "phone1"},
	{"global", "glyph_pocket_mode_state", "Glyph pocket mode", "new"},
	{"global", "glyph_screen_upward_state", "Glyph screen-upward mode", "new"},
	// Wireless charging
	{"global", "nt_wireless_forward_charge", "Wireless charging", "phone1"},
	{"global", "nt_wireless_reverse_charge", "Wireless reverse charging", "phone1"},
	{"global", "nt_reverse_charging_limiting_level", "Reverse charge limit (%)", "phone1"},
	// Essential Space
	{"global", "essential_notification_rules", "Essential Space notif. rules", "new"},
	{"secure", "essential_has_set_default_rule", "Essential Space default rule set", "new"},
	{"secure", "nt_essential_key_onboarding", "Essential key onboarding done", "new"},
	// UI / misc
	{"global", "nt_circle_to_search_support", "Circle-to-Search support", "all"},
	{"global", "nt_is_upgrade", "OTA upgrade flag", "all"},
	{"global", "ambient_enabled", "Always-on display", "all"},
	{"global", "ambient_tilt_to_wake", "Tilt-to-wake (AOD)", "all"},
	{"global", "ambient_touch_to_wake", "Touch-to-wake (AOD)", "all"},
	{"global", "led_effect_google_assistant_enalbe", "LED Google Assistant effect", "all"},
	{"system", "nothing_icon_pack", "Icon pack", "all"},
	{"system", "nothing_camera_foreground", "Camera foreground mode", "new"},
	// Game mode
	{"secure", "nt_game_mode_gaming", "Game mode active", "all"},
	{"secure", "nt_game_mode_notification_display_mode", "Game mode notif. display", "all"},
	{"secure", "nt_game_slider_enable", "Game slider enabled", "phone1"},
	// Misc secure
	{"secure", "nt_mistouch_prevention_enable", "Mistouch prevention", "phone1"},
	{"secure", "nt_face_recognition_unlock_with_mask", "Face unlock with mask", "phone1"},
	{"secure", "nt_flip_to_record_state", "Flip-to-record", "new"},
	{"secure", "nt_glimpse_lockscreen_cleared", "Glimpse lockscreen seen", "new"},
}


// ActionNothingSettings reads or writes Nothing-specific Android settings.
// key="" means list all. key set + value="" means read. key + value means write.
func ActionNothingSettings(serial, model, key, value string) error {
	if key != "" && value != "" {
		ns, rawKey, err := resolveNsKey(key, true)
		if err != nil {
			return err
		}
		if err := adb.PutSetting(serial, ns, rawKey, value); err != nil {
			return err
		}
		fmt.Printf("[OK] Set %s/%s = %s\n", ns, rawKey, value)
		return nil
	}

	if key != "" {
		ns, rawKey, err := resolveNsKey(key, false)
		if err != nil {
			return err
		}
		val := adb.Setting(serial, ns, rawKey)
		fmt.Printf("  %s/%s = %s\n", ns, rawKey, val)
		return nil
	}

	// List all
	fmt.Printf("\n  Nothing Settings \u2014 %s (%s)\n", model, serial)
	fmt.Printf("  %-8s  %-44s  %-38s  Value\n", "Namespace", "Key", "Label")
	fmt.Printf("  %-8s  %-44s  %-38s  -----\n",
		strings.Repeat("-", 8), strings.Repeat("-", 44), strings.Repeat("-", 38))

	for _, kk := range knownKeys {
		val := adb.Setting(serial, kk.namespace, kk.key)
		marker := "  "
		if val == "" {
			marker = " *"
		}
		fmt.Printf("%s %-8s  %-44s  %-38s  %s\n",
			marker, kk.namespace, kk.key, kk.label, val)
	}

	fmt.Println()
	fmt.Println("  (* = key not present on this device)")
	fmt.Println("  Use --key ns:key --value val to write.")
	return nil
}

func resolveNsKey(key string, forWrite bool) (ns, rawKey string, err error) {
	if strings.Contains(key, ":") {
		parts := strings.SplitN(key, ":", 2)
		return parts[0], parts[1], nil
	}
	// Look up in known keys
	for _, kk := range knownKeys {
		if kk.key == key {
			return kk.namespace, kk.key, nil
		}
	}
	if forWrite {
		return "", "", nterrors.AdbError(fmt.Sprintf(
			"Unknown key '%s'. Use 'namespace:key' syntax (e.g. global:%s) to write an arbitrary key.", key, key))
	}
	return "", "", nterrors.AdbError(fmt.Sprintf(
		"Unknown key '%s'. Use 'namespace:key' syntax (e.g. global:%s) to read an arbitrary key.", key, key))
}

const (
	essentialPkg      = "com.nothing.ntessentialspace"
	essentialIntelPkg = "com.nothing.essentialintelligence"
)

func hasPkg(serial, pkg string) bool {
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("pm list packages %s", pkg)})
	return strings.Contains(out, pkg)
}

// ActionEssentialSpace shows or toggles Essential Space.
// enable=nil -> status; enable=true -> enable; enable=false -> disable.
func ActionEssentialSpace(serial, model string, enable *bool) error {
	if !hasPkg(serial, essentialPkg) {
		fmt.Printf(
			"  [INFO] Essential Space is only available on Nothing Phone (2) and newer.\n"+
				"         Package '%s' not found on %s.\n", essentialPkg, model)
		return nil
	}

	if enable != nil {
		val := "0"
		label := "disabled"
		if *enable {
			val = "1"
			label = "enabled"
		}
		if err := adb.PutSetting(serial, "secure", "essential_space_enabled", val); err != nil {
			return err
		}
		fmt.Printf("[OK] Essential Space %s.\n", label)
		fmt.Println("     Changes take effect immediately.")
		return nil
	}

	// Status
	fmt.Printf("\n  Essential Space \u2014 %s (%s)\n", model, serial)

	enabledVal := adb.Setting(serial, "secure", "essential_space_enabled")
	rulesVal := adb.Setting(serial, "global", "essential_notification_rules")
	defaultRule := adb.Setting(serial, "secure", "essential_has_set_default_rule")
	onboarding := adb.Setting(serial, "secure", "nt_essential_key_onboarding")

	stateMap := map[string]string{"1": "ENABLED", "0": "DISABLED", "": "not set (default)"}
	state, ok := stateMap[enabledVal]
	if !ok {
		state = enabledVal
	}

	fmt.Printf("  %-22s: %s\n", "Enabled", state)
	fmt.Printf("  %-22s: %s\n", "Default rule set", yesNo(defaultRule == "1"))
	fmt.Printf("  %-22s: %s\n", "Onboarding completed", yesNo(onboarding == "1"))
	fmt.Printf("  %-22s: %s\n", "Notification rules", rulesVal)

	if hasPkg(serial, essentialIntelPkg) {
		fmt.Printf("  %-22s: installed\n", "Essential Intelligence")
	} else {
		fmt.Printf("  %-22s: not installed\n", "Essential Intelligence")
	}

	fmt.Println("\n  Toggle:")
	fmt.Println("    nothingctl essential-space --enable")
	fmt.Println("    nothingctl essential-space --no-enable")
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
