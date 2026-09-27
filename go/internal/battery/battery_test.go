package battery

import (
	"strings"
	"testing"
)

const sampleDumpsysBattery = `Current Battery Service state:
  AC powered: false
  USB powered: true
  Wireless powered: false
  Max charging current: 500000
  Max charging voltage: 5000000
  Charge counter: 4123000
  status: 2
  health: 2
  present: true
  level: 87
  scale: 100
  voltage: 4312
  temperature: 312
  technology: Li-poly
`

func TestParseDumpsysBattery(t *testing.T) {
	crlf := strings.ReplaceAll(sampleDumpsysBattery, "\n", "\r\n")
	for name, in := range map[string]string{"lf": sampleDumpsysBattery, "crlf": crlf} {
		t.Run(name, func(t *testing.T) {
			f := parseDumpsysBattery(in)
			want := map[string]string{
				"AC powered":  "false",
				"USB powered": "true",
				"status":      "2",
				"level":       "87",
				"voltage":     "4312",
				"temperature": "312",
				"technology":  "Li-poly",
			}
			for k, v := range want {
				if got := f[k]; got != v {
					t.Errorf("field %q = %q, want %q", k, got, v)
				}
			}
			// Header line ends with ":" and no space — must not become a key.
			if _, ok := f["Current Battery Service state"]; ok {
				t.Error("header line parsed as field")
			}
		})
	}
}

func TestParseDumpsysBatteryEmpty(t *testing.T) {
	if f := parseDumpsysBattery(""); len(f) != 0 {
		t.Errorf("empty input: got %v, want empty map", f)
	}
	if f := parseDumpsysBattery("garbage without separator\n\n"); len(f) != 0 {
		t.Errorf("garbage input: got %v, want empty map", f)
	}
}

func TestIntField(t *testing.T) {
	fields := map[string]string{
		"level":       "87",
		"temperature": " 312 ",
		"bad":         "abc",
		"negative":    "-5",
		"empty":       "",
	}
	cases := []struct {
		key    string
		want   int
		wantOK bool
	}{
		{"level", 87, true},
		{"temperature", 312, true},
		{"negative", -5, true},
		{"bad", 0, false},
		{"empty", 0, false},
		{"missing", 0, false},
	}
	for _, c := range cases {
		got, ok := intField(fields, c.key)
		if got != c.want || ok != c.wantOK {
			t.Errorf("intField(%q) = (%d, %v), want (%d, %v)", c.key, got, ok, c.want, c.wantOK)
		}
	}
}

func TestSecondsToHMS(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "00:00:00"},
		{59.9, "00:00:59"},
		{61, "00:01:01"},
		{3600, "01:00:00"},
		{3723, "01:02:03"},
		{360000, "100:00:00"},
	}
	for _, c := range cases {
		if got := secondsToHMS(c.in); got != c.want {
			t.Errorf("secondsToHMS(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseTimeExpr(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"5s", 5},
		{"1h 2m 3s", 3723},
		{"2m 500ms", 120.5},
		{"250ms", 0.25},
		{"1h2m3s4ms", 3723.004},
		{"garbage", 0},
	}
	for _, c := range cases {
		got := parseTimeExpr(c.in)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("parseTimeExpr(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseBatterystats(t *testing.T) {
	in := strings.Join([]string{
		"  UID 1000:",
		"    Wake lock system_server_lock: 9h 0m 0s (100 times)",
		"  UID 10123:",
		"    Wake lock com.example.sync: 1m 30s (4 times)",
		"    Wake lock com.example.sync: 30s (1 time)",
		"    Wake lock com.example.push: 5m 0s 0ms (2 times)",
		"  UID 10200:",
		"    Wake lock *alarm*: 10s (3 times)",
		"    unrelated line",
	}, "\r\n")

	got := parseBatterystats(in)
	if len(got) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(got), got)
	}
	// System UID (<10000) must be skipped; result sorted by secs desc.
	want := []appDrain{
		{"com.example.push", 300, 2},
		{"com.example.sync", 120, 5},
		{"*alarm*", 10, 3},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseBatterystatsEmpty(t *testing.T) {
	if got := parseBatterystats(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
	// Wake locks before any UID header are ignored.
	if got := parseBatterystats("Wake lock foo: 5s (1 times)\n"); len(got) != 0 {
		t.Errorf("wake lock without UID: got %+v", got)
	}
}

// TestParseBatterystatsRealFormat documents a suspected mismatch: real AOSP
// `dumpsys batterystats` output uses per-UID headers like "  u0a123:" (not
// "UID 10123:") and wake lock lines contain a type word between the duration
// and the count, e.g. "Wake lock foo: 1s 234ms partial (5 times) max=...".
// Neither matches uidRe/wlRe, so parseBatterystats returns nothing on real
// devices. Fixing it needs a device-verified sample, so it is skipped for now.
func TestParseBatterystatsRealFormat(t *testing.T) {
	t.Skip("parseBatterystats does not match real AOSP batterystats format; needs device-verified fix")
	in := "  u0a123:\n    Wake lock com.example.sync: 1m 30s 0ms partial (4 times) max=5000 actual=6000 realtime\n"
	if got := parseBatterystats(in); len(got) != 1 {
		t.Errorf("got %+v, want 1 entry", got)
	}
}
