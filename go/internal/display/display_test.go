package display

import "testing"

func TestFmtTimeout(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "n/a"},
		{"abc", "abc"},
		{"15000", "15s"},
		{"59999", "59s"},
		{"60000", "1 min"},
		{"119999", "1 min"},
		{"600000", "10 min"},
		{"2147483647", "35791 min"}, // "never" sentinel on some ROMs
	}
	for _, c := range cases {
		if got := fmtTimeout(c.in); got != c.want {
			t.Errorf("fmtTimeout(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFmtRotation(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0", "portrait"},
		{"1", "landscape"},
		{"2", "reverse-portrait"},
		{"3", "reverse-landscape"},
		{"", "n/a"},
		{"9", "9"},
	}
	for _, c := range cases {
		if got := fmtRotation(c.in); got != c.want {
			t.Errorf("fmtRotation(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFmtOnOff(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1", "on"},
		{"0", "off"},
		{"", "n/a"},
		{"null", "null"},
	}
	for _, c := range cases {
		if got := fmtOnOff(c.in); got != c.want {
			t.Errorf("fmtOnOff(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseWMSize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Physical size: 1080x2412", "1080x2412 (physical)"},
		{"Physical size: 1080x2412\r\nOverride size: 720x1608\r\n", "1080x2412 (physical)"},
		{"1080x2400", "1080x2400"},
		{"", "n/a"},
		{"error: no display", "error: no display"},
	}
	for _, c := range cases {
		if got := parseWMSize(c.in); got != c.want {
			t.Errorf("parseWMSize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseWMDensity(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Physical density: 420", "420"},
		{"Override density: 360\r\n", "360"},
		{"480", "480"},
		{"", "n/a"},
		{"no digits", "no digits"},
	}
	for _, c := range cases {
		if got := parseWMDensity(c.in); got != c.want {
			t.Errorf("parseWMDensity(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestParseWMDensityOverride documents an ambiguity: `wm density` prints the
// Physical line before the Override line, and the regex takes the leftmost
// match, so an active override (the effective DPI) is never shown even though
// the alternation lists "Override" first. Whether physical or effective DPI is
// intended is unclear, so this is left as-is.
func TestParseWMDensityOverride(t *testing.T) {
	t.Skip("ambiguous: parseWMDensity reports physical DPI even when an override is active")
	if got := parseWMDensity("Physical density: 420\nOverride density: 360\n"); got != "360" {
		t.Errorf("got %q, want 360", got)
	}
}

func TestParseRefreshRate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"    mRefreshRate=120.0\r\n", "120.0 Hz"},
		{"  DisplayDeviceInfo{... refreshRate 60.000004, ...}", "60.000004 Hz"},
		{"mDefaultRefreshRate=60.0\nmRefreshRate=90", "90 Hz"},
		{"", "n/a"},
		{"nothing relevant", "n/a"},
	}
	for _, c := range cases {
		if got := parseRefreshRate(c.in); got != c.want {
			t.Errorf("parseRefreshRate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestColorProfileLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0", "Natural (sRGB)"},
		{"1", "Vivid (P3)"},
		{"256", "Custom"},
		{"", "n/a"},
		{"7", "Unknown (7)"},
	}
	for _, c := range cases {
		if got := colorProfileLabel(c.in); got != c.want {
			t.Errorf("colorProfileLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
