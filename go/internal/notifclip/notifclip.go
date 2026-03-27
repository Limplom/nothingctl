// Package notifclip provides notification listing and clipboard management.
package notifclip

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const maxNotifications = 50

func shellStr(serial, cmd string) string {
	stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimRight(stdout, "\r\n"))
}

func getSDK(serial string) int {
	raw := shellStr(serial, "getprop ro.build.version.sdk")
	var sdk int
	fmt.Sscanf(raw, "%d", &sdk)
	return sdk
}

type notification struct {
	pkg, title, text string
}

var notifRecordRe    = regexp.MustCompile(`(?m)\bNotificationRecord\b`)
var pkgRe            = regexp.MustCompile(`\bpkg=([^\s,)]+)`)
var androidTitleRe   = regexp.MustCompile(`^android\.title=`)
var androidTextRe    = regexp.MustCompile(`^android\.text=`)
var spanWrapRe       = regexp.MustCompile(`^(?:String|SpannableString)\s+\((.+)\)$`)

func parseNotifications(output string) []notification {
	var notifications []notification
	blocks := splitOnBoundary(output, "NotificationRecord")

	for _, block := range blocks {
		if !strings.Contains(block, "NotificationRecord") {
			continue
		}

		var pkg, title, text string

		for _, line := range strings.Split(block, "\n") {
			stripped := strings.TrimRight(strings.TrimSpace(line), "\r")

			if pkg == "" {
				if m := pkgRe.FindStringSubmatch(stripped); m != nil {
					pkg = m[1]
				}
			}

			if title == "" && androidTitleRe.MatchString(stripped) {
				raw := stripped[len("android.title="):]
				if m := spanWrapRe.FindStringSubmatch(strings.TrimSpace(raw)); m != nil {
					title = m[1]
				} else {
					title = strings.TrimSpace(raw)
				}
			}

			if text == "" && androidTextRe.MatchString(stripped) {
				raw := stripped[len("android.text="):]
				if m := spanWrapRe.FindStringSubmatch(strings.TrimSpace(raw)); m != nil {
					text = m[1]
				} else {
					text = strings.TrimSpace(raw)
				}
			}
		}

		if pkg == "" {
			continue
		}
		if title == "null" || title == "null\r" {
			title = ""
		}
		if text == "null" || text == "null\r" {
			text = ""
		}
		if title == "" {
			title = "(no title)"
		}
		if text == "" {
			text = "(no text)"
		}
		notifications = append(notifications, notification{pkg: pkg, title: title, text: text})
	}
	return notifications
}

// splitOnBoundary splits text into sections before each occurrence of boundary word.
func splitOnBoundary(text, boundary string) []string {
	var blocks []string
	start := 0
	for i := 0; i < len(text); i++ {
		idx := strings.Index(text[i:], boundary)
		if idx == -1 {
			blocks = append(blocks, text[start:])
			break
		}
		absIdx := i + idx
		if absIdx > start {
			blocks = append(blocks, text[start:absIdx])
		}
		start = absIdx
		i = absIdx
	}
	if len(blocks) == 0 && start < len(text) {
		blocks = append(blocks, text[start:])
	}
	return blocks
}

// ActionNotifications lists active notifications on the device.
// packageName="" means show all.
func ActionNotifications(serial, model, packageName string) error {
	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
		"dumpsys notification --noredact"})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("dumpsys notification failed: %s", strings.TrimSpace(stderr)))
	}

	all := parseNotifications(stdout)

	shown := all
	if packageName != "" {
		var filtered []notification
		for _, n := range all {
			if n.pkg == packageName {
				filtered = append(filtered, n)
			}
		}
		shown = filtered
	}

	capped := shown
	if len(capped) > maxNotifications {
		capped = capped[:maxNotifications]
	}

	fmt.Printf("\n  Active Notifications \u2014 %s\n\n", model)

	if len(capped) == 0 {
		if packageName != "" {
			fmt.Printf("  No notifications found for package: %s\n\n", packageName)
		} else {
			fmt.Println("  No active notifications.\n")
		}
		return nil
	}

	pkgW := len("Package")
	titleW := len("Title")
	for _, n := range capped {
		if len(n.pkg) > pkgW {
			pkgW = len(n.pkg)
		}
		if len(n.title) > titleW {
			titleW = len(n.title)
		}
	}
	if pkgW > 40 {
		pkgW = 40
	}
	if titleW > 30 {
		titleW = 30
	}

	headerFmt := fmt.Sprintf("  %%-4s %%-%ds  %%-%ds  %%s", pkgW, titleW)
	header := fmt.Sprintf(headerFmt, "#", "Package", "Title", "Text")
	fmt.Println(header)
	fmt.Println("  " + strings.Repeat("-", len(header)-2))

	rowFmt := fmt.Sprintf("  %%-4d %%-%ds  %%-%ds  %%s", pkgW, titleW)
	for idx, notif := range capped {
		pkgCol := notif.pkg
		if len(pkgCol) > pkgW {
			pkgCol = pkgCol[:pkgW]
		}
		titleCol := notif.title
		if len(titleCol) > titleW {
			titleCol = titleCol[:titleW]
		}
		fmt.Printf(rowFmt+"\n", idx+1, pkgCol, titleCol, notif.text)
	}
	fmt.Println()

	total := len(all)
	shownCount := len(capped)

	var footer string
	switch {
	case packageName != "":
		footer = fmt.Sprintf("  Total: %d notification(s) (%d shown, filtered by package: %s)",
			total, shownCount, packageName)
	case shownCount < total:
		footer = fmt.Sprintf("  Total: %d notification(s) (showing first %d)", total, shownCount)
	default:
		footer = fmt.Sprintf("  Total: %d notification(s)", total)
	}
	fmt.Println(footer)
	fmt.Println()
	return nil
}

// ActionClipboard reads or sets the device clipboard.
// text="" means read mode; non-empty means set mode.
func ActionClipboard(serial, model, text string) error {
	sdk := getSDK(serial)

	fmt.Printf("\n  Clipboard \u2014 %s\n\n", model)

	if text != "" {
		out, _, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"cmd", "clipboard", "set-primary", text})
		outLower := strings.ToLower(out)
		if code == 0 && !strings.Contains(outLower, "error") && !strings.Contains(outLower, "unknown") {
			fmt.Printf("  Clipboard set to: %s\n\n", text)
			return nil
		}
		// Fallback
		out2, _, code2 := adb.Run([]string{"adb", "-s", serial, "shell",
			"cmd", "clipboard", "set", text})
		out2Lower := strings.ToLower(out2)
		if code2 == 0 && !strings.Contains(out2Lower, "error") && !strings.Contains(out2Lower, "unknown") {
			fmt.Printf("  Clipboard set to: %s\n\n", text)
			return nil
		}
		fmt.Println("  [WARN] Could not set clipboard via adb shell cmd clipboard.")
		if sdk > 0 && sdk < 29 {
			fmt.Printf("         Device is running SDK %d — cmd clipboard may not be available.\n", sdk)
		} else {
			fmt.Printf("         The shell user may lack permission on this build (SDK %d).\n", sdk)
		}
		fmt.Println()
		return nil
	}

	// Read mode
	// Attempt 1: cmd clipboard get
	r1, _, c1 := adb.Run([]string{"adb", "-s", serial, "shell", "cmd", "clipboard", "get"})
	r1Lower := strings.ToLower(r1)
	if c1 == 0 && strings.TrimSpace(r1) != "" &&
		!strings.Contains(r1Lower, "error") && !strings.Contains(r1Lower, "unknown") {
		fmt.Printf("  Content: %s\n\n", strings.TrimSpace(r1))
		return nil
	}

	// Attempt 2: service call clipboard 2 i32 0
	r2, _, c2 := adb.Run([]string{"adb", "-s", serial, "shell",
		"service", "call", "clipboard", "2", "i32", "0"})
	if c2 == 0 && strings.TrimSpace(r2) != "" {
		decoded := decodeParcelUTF16(r2)
		if decoded != "" {
			fmt.Printf("  Content: %s\n\n", decoded)
			return nil
		}
	}

	sdkLabel := "SDK unknown"
	if sdk > 0 {
		sdkLabel = fmt.Sprintf("SDK %d", sdk)
	}
	fmt.Printf("  [INFO] Clipboard read requires foreground app access on Android 10+ (%s).\n", sdkLabel)
	fmt.Println("         Background reads via adb shell are blocked by the OS.")
	fmt.Println("         Use --text \"content\" to set clipboard instead.\n")
	return nil
}

var parcelRe = regexp.MustCompile(`Result:\s*Parcel\(([0-9a-fA-F\s]+)\)`)

func decodeParcelUTF16(output string) string {
	m := parcelRe.FindStringSubmatch(output)
	if m == nil {
		return ""
	}
	tokens := strings.Fields(m[1])
	if len(tokens) < 2 {
		return ""
	}
	dataTokens := tokens[2:]
	raw := make([]byte, 0, len(dataTokens)*4)
	for _, token := range dataTokens {
		var word uint32
		if _, err := fmt.Sscanf(token, "%x", &word); err != nil {
			continue
		}
		raw = append(raw,
			byte(word),
			byte(word>>8),
			byte(word>>16),
			byte(word>>24))
	}
	// Decode UTF-16LE
	if len(raw)%2 != 0 {
		raw = raw[:len(raw)-1]
	}
	runes := make([]rune, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		r := rune(uint16(raw[i]) | uint16(raw[i+1])<<8)
		if r == 0 {
			break
		}
		runes = append(runes, r)
	}
	return strings.TrimSpace(string(runes))
}
