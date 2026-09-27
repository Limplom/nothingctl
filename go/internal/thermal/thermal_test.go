package thermal

import (
	"strings"
	"testing"
)

func TestFormatTemp(t *testing.T) {
	cases := []struct {
		milliC   int
		wantTemp string
		wantBar  int
		wantWarn bool
	}{
		{0, "  0.0 °C", 0, false},
		{-5000, " -5.0 °C", 0, false},
		{20000, " 20.0 °C", 0, false},
		{35500, " 35.5 °C", 7, false},
		{59999, " 60.0 °C", 19, false}, // rounds for display, but < 60 → no warning
		{60000, " 60.0 °C", 20, true},
		{95000, " 95.0 °C", 30, true}, // bar capped at 30
	}
	for _, c := range cases {
		got := formatTemp(c.milliC)
		if !strings.HasPrefix(got, c.wantTemp) {
			t.Errorf("formatTemp(%d) = %q, want prefix %q", c.milliC, got, c.wantTemp)
		}
		if n := strings.Count(got, "█"); n != c.wantBar {
			t.Errorf("formatTemp(%d) bar length = %d, want %d", c.milliC, n, c.wantBar)
		}
		if warn := strings.HasSuffix(got, " !"); warn != c.wantWarn {
			t.Errorf("formatTemp(%d) warn = %v, want %v", c.milliC, warn, c.wantWarn)
		}
	}
}

func TestParseThermalZones(t *testing.T) {
	in := strings.Join([]string{
		"/sys/class/thermal/thermal_zone0/|soc_max|45123",
		"/sys/class/thermal/thermal_zone1/|battery|31000",
		"/sys/class/thermal/thermal_zone2/|disabled_sensor|-274000", // invalid sentinel
		"/sys/class/thermal/thermal_zone3/|broken|",                 // no value
		"/sys/class/thermal/thermal_zone4/|cold|-12000",
		"su: permission denied",
		"",
	}, "\r\n")

	got := parseThermalZones(in)
	want := []thermalZone{
		{"/sys/class/thermal/thermal_zone0/", "soc_max", 45123},
		{"/sys/class/thermal/thermal_zone1/", "battery", 31000},
		{"/sys/class/thermal/thermal_zone4/", "cold", -12000},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d zones, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("zone %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseThermalZonesEmpty(t *testing.T) {
	if got := parseThermalZones(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}
