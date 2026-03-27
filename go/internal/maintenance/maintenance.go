// Package maintenance provides cache clearing and locale management.
package maintenance

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)


func settingGet(serial, namespace, key string) string {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "settings", "get", namespace, key})
	val := strings.TrimSpace(strings.TrimRight(stdout, "\r\n"))
	if val == "null" || val == "null\r" || val == "null\n" {
		return ""
	}
	return val
}

func parseDfAvailable(dfOutput string) int {
	for _, line := range strings.Split(dfOutput, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "Filesystem") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 4 {
			var v int
			if _, err := fmt.Sscanf(parts[1], "%d", &v); err == nil {
				return v
			}
		}
		if len(parts) >= 4 {
			var v int
			if _, err := fmt.Sscanf(parts[3], "%d", &v); err == nil {
				return v
			}
		}
	}
	return -1
}

func fmtKB(kb int) string {
	switch {
	case kb >= 1024*1024:
		return fmt.Sprintf("%.2f GB", float64(kb)/1024/1024)
	case kb >= 1024:
		return fmt.Sprintf("%.1f MB", float64(kb)/1024)
	default:
		return fmt.Sprintf("%d KB", kb)
	}
}

func getFreeData(serial string) int {
	stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "df /data"})
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return -1
	}
	return parseDfAvailable(stdout)
}

// ActionCacheClear clears app caches.
// packageName="" means system-wide trim only; non-empty also clears that package.
func ActionCacheClear(serial, model, packageName string) error {
	fmt.Printf("\n  Cache Clear \u2014 %s\n\n", model)

	freeBefore := getFreeData(serial)

	_, _, trimCode := adb.Run([]string{"adb", "-s", serial, "shell", "pm", "trim-caches", "10G"})
	if trimCode == 0 {
		fmt.Println("  [OK] System cache trimmed (target: 10 GB freed).")
	} else {
		fmt.Println("  [WARN] System trim failed or not supported.")
	}

	if packageName != "" {
		out, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"cmd", "package", "clear-cache", packageName})
		if code == 0 {
			fmt.Printf("  [OK] Cache cleared: %s\n", packageName)
		} else {
			errMsg := strings.TrimSpace(stderr)
			if errMsg == "" {
				errMsg = strings.TrimSpace(out)
			}
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nterrors.AdbError(fmt.Sprintf("Failed to clear cache for '%s': %s", packageName, errMsg))
		}
	}

	freeAfter := getFreeData(serial)

	if freeBefore >= 0 || freeAfter >= 0 {
		fmt.Println()
		if freeBefore >= 0 {
			fmt.Printf("  %-22s: %s\n", "Free (/data) before", fmtKB(freeBefore))
		}
		if freeAfter >= 0 {
			fmt.Printf("  %-22s: %s\n", "Free (/data) after", fmtKB(freeAfter))
		}
		if freeBefore >= 0 && freeAfter >= 0 {
			delta := freeAfter - freeBefore
			sign := ""
			if delta >= 0 {
				sign = "+"
			}
			fmt.Printf("  %-22s: %s%s\n", "Change", sign, fmtKB(delta))
		}
	}
	fmt.Println()
	return nil
}

var utcOffsetRe = regexp.MustCompile(`^([+-])(\d{2})(\d{2})$`)

func getUTCOffset(serial string) string {
	raw := adb.ShellStr(serial, "date +%z")
	m := utcOffsetRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return ""
	}
	var hh, mm int
	fmt.Sscanf(m[2], "%d", &hh)
	fmt.Sscanf(m[3], "%d", &mm)
	if mm == 0 {
		return fmt.Sprintf("UTC%s%d", m[1], hh)
	}
	return fmt.Sprintf("UTC%s%d:%02d", m[1], hh, mm)
}

// ActionLocale reads or sets locale, timezone, and time format.
// Empty lang/timezone means don't change. hour24=nil means don't change.
func ActionLocale(serial, model, lang, timezone string, hour24 *bool) error {
	anyChange := lang != "" || timezone != "" || hour24 != nil

	if lang != "" {
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"settings", "put", "system", "system_locales", lang})
		if code != 0 {
			errMsg := strings.TrimSpace(stderr)
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nterrors.AdbError(fmt.Sprintf("Failed to set language to '%s': %s", lang, errMsg))
		}
		adb.Run([]string{"adb", "-s", serial, "shell",
			"am broadcast -a android.intent.action.LOCALE_CHANGED"})
		fmt.Printf("  [OK] Language set to: %s\n", lang)
	}

	if timezone != "" {
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"cmd", "alarm", "set-timezone", timezone})
		if code != 0 {
			errMsg := strings.TrimSpace(stderr)
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nterrors.AdbError(fmt.Sprintf("Failed to set timezone to '%s': %s", timezone, errMsg))
		}
		fmt.Printf("  [OK] Timezone set to: %s\n", timezone)
	}

	if hour24 != nil {
		value := "12"
		if *hour24 {
			value = "24"
		}
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"settings", "put", "system", "time_12_24", value})
		if code != 0 {
			errMsg := strings.TrimSpace(stderr)
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nterrors.AdbError(fmt.Sprintf("Failed to set time format: %s", errMsg))
		}
		fmt.Printf("  [OK] Time format set to: %sh\n", value)
	}

	if anyChange {
		fmt.Println()
	}

	// Read current state
	localeVal := adb.ShellStr(serial, "getprop persist.sys.locale")
	if localeVal == "" {
		localeVal = settingGet(serial, "system", "system_locales")
	}
	if localeVal == "" {
		localeVal = adb.ShellStr(serial, "getprop ro.product.locale")
	}
	langDisplay := localeVal
	if langDisplay == "" {
		langDisplay = "(not available)"
	}

	tzVal := adb.ShellStr(serial, "getprop persist.sys.timezone")
	utcLabel := getUTCOffset(serial)
	tzDisplay := tzVal
	if tzVal != "" && utcLabel != "" {
		tzDisplay = fmt.Sprintf("%s (%s)", tzVal, utcLabel)
	} else if tzVal == "" {
		tzDisplay = "(not available)"
	}

	fmtVal := settingGet(serial, "system", "time_12_24")
	fmtDisplay := "(system default)"
	switch fmtVal {
	case "24":
		fmtDisplay = "24h"
	case "12":
		fmtDisplay = "12h"
	case "":
		fmtDisplay = "(system default)"
	default:
		fmtDisplay = fmtVal
	}

	fmt.Printf("  Locale \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %s\n", "Language", langDisplay)
	fmt.Printf("  %-16s: %s\n", "Timezone", tzDisplay)
	fmt.Printf("  %-16s: %s\n", "Time Format", fmtDisplay)
	fmt.Println()
	return nil
}
