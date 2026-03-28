// Package devoptions manages Android developer options settings.
package devoptions

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

type setting struct {
	namespace, key, value string
}

type option struct {
	label    string
	settings []setting
}

var animKeys = []struct{ ns, key string }{
	{"global", "window_animation_scale"},
	{"global", "transition_animation_scale"},
	{"global", "animator_duration_scale"},
}

func animSettings(value string) []setting {
	var s []setting
	for _, k := range animKeys {
		s = append(s, setting{k.ns, k.key, value})
	}
	return s
}

var options = map[string]option{
	"animations_off":   {"Animations off (0x)", animSettings("0")},
	"animations_on":    {"Animations on (1x)", animSettings("1")},
	"stay_awake":       {"Display stays on while charging", []setting{{"global", "stay_on_while_plugged_in", "3"}}},
	"stay_awake_off":   {"Normal screen timeout", []setting{{"global", "stay_on_while_plugged_in", "0"}}},
	"show_touches":     {"Show touches", []setting{{"system", "show_touches", "1"}}},
	"hide_touches":     {"Hide touches", []setting{{"system", "show_touches", "0"}}},
	"pointer_location": {"Pointer Location", []setting{{"system", "pointer_location", "1"}}},
	"usb_debugging":    {"USB Debugging", []setting{{"global", "adb_enabled", "1"}}},
	"bg_process_limit": {"Background processes: max 4", []setting{{"global", "background_process_limit", "4"}}},
}

var menuOrder = []string{
	"animations_off",
	"animations_on",
	"stay_awake",
	"stay_awake_off",
	"show_touches",
	"hide_touches",
	"pointer_location",
	"usb_debugging",
	"bg_process_limit",
}


func currentValueForOption(serial, key string) string {
	opt := options[key]
	if len(opt.settings) == 0 {
		return "(not set)"
	}
	val := adb.Setting(serial, opt.settings[0].namespace, opt.settings[0].key)
	if val == "" {
		return "(not set)"
	}
	return val
}

func applyOption(serial, key string) error {
	opt := options[key]
	for _, s := range opt.settings {
		if err := adb.PutSetting(serial, s.namespace, s.key, s.value); err != nil {
			return err
		}
	}
	return nil
}

// ActionDevOptions manages developer options.
// key="" and value="" -> interactive menu
// key set, value="" -> read current value
// key + value -> apply directly
func ActionDevOptions(serial, model, key, value string) error {
	if key != "" && value != "" {
		if opt, ok := options[key]; ok {
			if err := applyOption(serial, key); err != nil {
				return err
			}
			fmt.Printf("  [OK] Applied: %s  (%s)\n", opt.label, key)
			return nil
		}

		// Parse "namespace/key" or "namespace key"
		ns, settingKey, err := parseNsKey(key)
		if err != nil {
			return err
		}
		if err := adb.PutSetting(serial, ns, settingKey, value); err != nil {
			return err
		}
		fmt.Printf("  [OK] settings put %s %s = %s  [%s]\n", ns, settingKey, value, model)
		return nil
	}

	if key != "" && value == "" {
		if opt, ok := options[key]; ok {
			current := currentValueForOption(serial, key)
			fmt.Printf("\n  %s (%s): %s\n\n", opt.label, key, current)
			return nil
		}
		ns, settingKey, err := parseNsKey(key)
		if err != nil {
			return err
		}
		current := adb.Setting(serial, ns, settingKey)
		if current == "" {
			current = "(not set)"
		}
		fmt.Printf("\n  settings get %s %s = %s\n\n", ns, settingKey, current)
		return nil
	}

	// Interactive menu
	return showMenu(serial, model)
}

func parseNsKey(key string) (ns, settingKey string, err error) {
	if strings.Contains(key, "/") {
		parts := strings.SplitN(key, "/", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}
	parts := strings.Fields(key)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", "", nterrors.AdbError(fmt.Sprintf(
		"Cannot parse key '%s'. Use 'namespace/setting_key' or a named shorthand.", key))
}

func showMenu(serial, model string) error {
	fmt.Printf("\n  Developer Options \u2014 %s\n\n", model)
	fmt.Printf("  %-3s  %-20s  %-38s  %s\n", "#", "Key", "Description", "Current")
	fmt.Printf("  %-3s  %-20s  %-38s  %s\n",
		strings.Repeat("-", 3), strings.Repeat("-", 20),
		strings.Repeat("-", 38), strings.Repeat("-", 12))

	for idx, optKey := range menuOrder {
		opt := options[optKey]
		current := currentValueForOption(serial, optKey)
		fmt.Printf("  %-3d  %-20s  %-38s  %s\n", idx, optKey, opt.label, current)
	}
	fmt.Println()
	fmt.Println("  Enter number to apply, or press Enter to cancel.")

	choice, err := adb.Prompt("  Select: ")
	if err != nil || choice == "" {
		return nil
	}

	var idx int
	if _, err := fmt.Sscanf(choice, "%d", &idx); err != nil || idx < 0 || idx >= len(menuOrder) {
		fmt.Printf("  [WARN] Invalid selection: %q. Aborted.\n", choice)
		return nil
	}

	optKey := menuOrder[idx]
	opt := options[optKey]
	if err := applyOption(serial, optKey); err != nil {
		return err
	}
	fmt.Printf("  [OK] Applied: %s  (%s)\n", opt.label, optKey)
	return nil
}

// ActionScreenAlwaysOn controls the 'stay on while plugged in' setting.
// enable=nil -> show status; enable=true -> on; enable=false -> off
func ActionScreenAlwaysOn(serial, model string, enable *bool) error {
	stayVal := adb.Setting(serial, "global", "stay_on_while_plugged_in")
	timeoutMs := adb.Setting(serial, "system", "screen_off_timeout")

	if enable == nil {
		fmt.Printf("\n  Screen Always On \u2014 %s\n\n", model)

		var v int
		vValid := false
		if stayVal != "" {
			if _, err := fmt.Sscanf(stayVal, "%d", &v); err == nil {
				vValid = true
			}
		}

		plugged := map[int]string{1: "AC", 2: "USB", 4: "Wireless", 8: "Dock"}
		stayLabel := ""
		if vValid {
			if v == 0 {
				stayLabel = "off (normal timeout)"
			} else {
				var sources []string
				for bit, label := range plugged {
					if v&bit != 0 {
						sources = append(sources, label)
					}
				}
				stayLabel = fmt.Sprintf("on (%s)", strings.Join(sources, ", "))
			}
		} else {
			stayLabel = fmt.Sprintf("unknown (%q)", stayVal)
		}
		fmt.Printf("  %-24s: %s\n", "Stay-on while plugged", stayLabel)

		timeoutLabel := "(not set)"
		if timeoutMs != "" {
			var ms int
			if _, err := fmt.Sscanf(timeoutMs, "%d", &ms); err == nil {
				switch {
				case ms < 0:
					timeoutLabel = "never"
				case ms < 1000:
					timeoutLabel = fmt.Sprintf("%d ms", ms)
				case ms < 60000:
					timeoutLabel = fmt.Sprintf("%d s", ms/1000)
				default:
					timeoutLabel = fmt.Sprintf("%d min", ms/60000)
				}
			} else {
				timeoutLabel = timeoutMs
			}
		}
		fmt.Printf("  %-24s: %s\n", "Screen-off timeout", timeoutLabel)
		fmt.Println()
		return nil
	}

	if *enable {
		if err := adb.PutSetting(serial, "global", "stay_on_while_plugged_in", "3"); err != nil {
			return err
		}
		fmt.Printf("  [OK] Screen stays on while plugged in (AC + USB + Wireless)  [%s]\n", model)
	} else {
		if err := adb.PutSetting(serial, "global", "stay_on_while_plugged_in", "0"); err != nil {
			return err
		}
		fmt.Printf("  [OK] Screen always-on disabled (normal timeout restored)  [%s]\n", model)
	}
	return nil
}
