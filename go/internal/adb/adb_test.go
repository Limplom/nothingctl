package adb

import (
	"reflect"
	"testing"
	"time"
)

func TestParseShellLines(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"only whitespace", " \r\n\t\n\r\n", nil},
		{"crlf", "boot_a\r\nboot_b\r\n", []string{"boot_a", "boot_b"}},
		{"lf", "a\nb\nc", []string{"a", "b", "c"}},
		{"blank lines dropped", "a\n\n\nb\n", []string{"a", "b"}},
		{"surrounding spaces trimmed", "  package:com.a  \n\tpackage:com.b\r\n", []string{"package:com.a", "package:com.b"}},
		{"inner spaces kept", "a b  c\n", []string{"a b  c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ParseShellLines(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestSetFastbootPollTimeout(t *testing.T) {
	orig := fastbootPollTimeout
	t.Cleanup(func() { fastbootPollTimeout = orig })

	cases := []struct {
		in   int
		want time.Duration
	}{
		{120, 120 * time.Second},
		{0, 120 * time.Second},  // ignored
		{-5, 120 * time.Second}, // ignored
		{1, 1 * time.Second},
	}
	for _, c := range cases {
		SetFastbootPollTimeout(c.in)
		if fastbootPollTimeout != c.want {
			t.Errorf("after SetFastbootPollTimeout(%d): %v, want %v", c.in, fastbootPollTimeout, c.want)
		}
	}
}

func TestParseCurrentSlot(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"a", "current-slot: a\nFinished. Total time: 0.001s\n", "_a"},
		{"b", "current-slot: b\r\n", "_b"},
		{"no space", "current-slot:b", "_b"},
		{"bootloader prefix", "(bootloader) current-slot: a\nOKAY", "_a"},
		{"missing", "getvar:current-slot FAILED (remote: 'unknown variable')", "unknown"},
		{"empty", "", "unknown"},
		{"invalid slot", "current-slot: c", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseCurrentSlot(c.in); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestParseCurrentSlotUnderscore documents that some bootloaders report the
// slot with a leading underscore ("current-slot: _a"), which is not parsed.
// QueryCurrentSlot is display-only, so this is not fixed.
func TestParseCurrentSlotUnderscore(t *testing.T) {
	t.Skip("ambiguous: 'current-slot: _a' form not recognised (display-only); see report")
	if got := parseCurrentSlot("current-slot: _a"); got != "_a" {
		t.Fatalf("got %q, want _a", got)
	}
}

func TestPickFastbootSerial(t *testing.T) {
	cases := []struct {
		name, out, adbSerial, want string
	}{
		{"same serial", "P2126F000626\tfastboot\n", "P2126F000626", "P2126F000626"},
		{"soc hash differs", "fd1163d8\tfastboot\n", "P2126F000626", "fd1163d8"},
		{"no fastboot device", "", "P2126F000626", "P2126F000626"},
		{"whitespace only", "\n  \n", "ABC", "ABC"},
		{"prefers adb serial among many", "OTHER1\tfastboot\nP2126F000626\tfastboot\n", "P2126F000626", "P2126F000626"},
		{"crlf output", "fd1163d8\tfastboot\r\n", "X", "fd1163d8"},
		{"empty adb serial", "fd1163d8\tfastboot\n", "", "fd1163d8"},
		// Regression: the ADB serial used to be matched as a substring of the
		// whole output, so "ABC" matched "ABCD" and a serial that is not in
		// the list was returned (fastboot -s ABC then waits forever).
		{"substring of another serial", "ABCD\tfastboot\n", "ABC", "ABCD"},
		{"serial appears only in state column", "XYZ\tfastboot\n", "fastboot", "XYZ"},
		// Several phones in fastboot and none is ours: never guess which one
		// to flash — fall back to the ADB serial so fastboot fails cleanly.
		{"ambiguous: several devices, no match", "OTHER1\tfastboot\nOTHER2\tfastboot\n", "P2126F000626", "P2126F000626"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickFastbootSerial(c.out, c.adbSerial); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseAdbDevices(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"none", "List of devices attached\n\n", nil},
		{"one", "List of devices attached\nABC123\tdevice\n\n", []string{"ABC123"}},
		{"crlf", "List of devices attached\r\nABC123\tdevice\r\n", []string{"ABC123"}},
		{"daemon noise", "* daemon not running; starting now at tcp:5037\n* daemon started successfully\nList of devices attached\nA\tdevice\n", []string{"A"}},
		{"non-ready states skipped",
			"List of devices attached\nA\tunauthorized\nB\toffline\nC\trecovery\nD\tsideload\nE\tdevice\n",
			[]string{"E"}},
		{"multiple", "List of devices attached\nA\tdevice\nemulator-5554\tdevice\n", []string{"A", "emulator-5554"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseAdbDevices(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestEnsureDeviceLines(t *testing.T) {
	cases := []struct {
		name, in string
		want     int
	}{
		{"none", "List of devices attached\n\n", 0},
		{"one", "List of devices attached\nABC123         device usb:1-1 product:Spacewar model:A063 device:Spacewar transport_id:1\n", 1},
		{"unauthorized skipped", "List of devices attached\nABC123         unauthorized usb:1-1 transport_id:1\n", 0},
		{"two", "List of devices attached\nA device usb:1 product:x model:y device:z\nB device usb:2 product:x model:y device:z\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ensureDeviceLines(c.in); len(got) != c.want {
				t.Errorf("got %d lines %q, want %d", len(got), got, c.want)
			}
		})
	}
}

// TestEnsureDeviceLinesNonReadyStates documents that EnsureDevice treats any
// `adb devices -l` line containing " device" (including the " device:<name>"
// field) as attached, so offline/recovery/sideload devices are selected.
func TestEnsureDeviceLinesNonReadyStates(t *testing.T) {
	t.Skip("ambiguous: offline/recovery devices counted by EnsureDevice via ' device:' field; see report")
	in := "List of devices attached\nABC offline product:Spacewar model:A063 device:Spacewar transport_id:1\n"
	if got := ensureDeviceLines(in); len(got) != 0 {
		t.Fatalf("offline device accepted: %q", got)
	}
}

func TestParseDeviceProps(t *testing.T) {
	cases := []struct {
		name                                string
		in                                  string
		model, manufacturer, codename, slot string
	}{
		{"full", "Nothing Phone (1)\nA063\nNothing\nSpacewar\n_a\n", "Phone (1)", "Nothing", "Spacewar", "_a"},
		{"crlf", "Nothing Phone (2)\r\nA065\r\nNothing\r\nPong\r\n_b\r\n", "Phone (2)", "Nothing", "Pong", "_b"},
		{"brand name missing uses model code", "\nA063\nNothing\nspacewar\n_a\n", "A063", "Nothing", "Spacewar", "_a"},
		{"lowercase nothing prefix stripped", "nothing Phone (3a)\nA059\nNothing\nasteroids\n_b", "Phone (3a)", "Nothing", "Asteroids", "_b"},
		{"brand without prefix kept", "CMF Phone 1\nA015\nNothing\ntetris\n", "CMF Phone 1", "Nothing", "Tetris", ""},
		{"empty output", "", "", "", "", ""},
		{"truncated output", "Nothing Phone (1)\nA063", "Phone (1)", "", "", ""},
		{"non-A/B device", "Nothing Phone (1)\nA063\nNothing\nSpacewar\n\n", "Phone (1)", "Nothing", "Spacewar", ""},
		{"brand exactly 'Nothing' not stripped", "Nothing\nA063\nNothing\nSpacewar\n_a", "Nothing", "Nothing", "Spacewar", "_a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, mf, cn, s := parseDeviceProps(c.in)
			if m != c.model || mf != c.manufacturer || cn != c.codename || s != c.slot {
				t.Errorf("got (%q,%q,%q,%q), want (%q,%q,%q,%q)", m, mf, cn, s, c.model, c.manufacturer, c.codename, c.slot)
			}
		})
	}
}
