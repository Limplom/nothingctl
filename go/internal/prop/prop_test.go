package prop

import (
	"reflect"
	"testing"
)

func TestGroupKey(t *testing.T) {
	cases := []struct {
		prop string
		want string
	}{
		{"ro.product.model", "ro.product"},
		{"ro.build.version.release", "ro.build"},
		{"ro.boot.slot_suffix", "ro.boot"},
		{"ro.hardware", "ro.hardware"},
		{"persist.sys.usb.config", "persist"},
		{"sys.boot_completed", "sys"},
		{"gsm.sim.state", "gsm"},
		{"net.dns1", "net"},
		{"wifi.interface", "wifi"},
		{"dalvik.vm.heapsize", "other"},
		{"ro.secure", "other"},
		{"", "other"},
	}
	for _, tc := range cases {
		if got := groupKey(tc.prop); got != tc.want {
			t.Errorf("groupKey(%q) = %q, want %q", tc.prop, got, tc.want)
		}
	}
}

func TestParseGetprop(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want [][2]string
	}{
		{"empty", "", nil},
		{
			"typical",
			"[ro.product.model]: [A063]\n[ro.boot.slot_suffix]: [_a]\n",
			[][2]string{{"ro.product.model", "A063"}, {"ro.boot.slot_suffix", "_a"}},
		},
		{
			"CRLF (Windows adb)",
			"[ro.product.model]: [A063]\r\n[sys.boot_completed]: [1]\r\n",
			[][2]string{{"ro.product.model", "A063"}, {"sys.boot_completed", "1"}},
		},
		{"empty value", "[persist.foo]: []\n", [][2]string{{"persist.foo", ""}}},
		{"value with spaces", "[ro.build.display.id]: [Nothing OS 3.0]\n", [][2]string{{"ro.build.display.id", "Nothing OS 3.0"}}},
		{"value with brackets", "[x.y]: [a]b]\n", [][2]string{{"x.y", "a]b"}}},
		{"unbracketed value", "[x.y]: raw\n", [][2]string{{"x.y", "raw"}}},
		{
			"garbage lines skipped",
			"WARNING: linker\n\n[no-colon]\n[]: [emptykey]\n[ok]: [1]\n",
			[][2]string{{"ok", "1"}},
		},
	}
	for _, tc := range cases {
		got := parseGetprop(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseGetprop = %v, want %v", tc.name, got, tc.want)
		}
	}
}
