package profile

import "testing"

// TestCatalogueLoads ensures the embedded JSON parses without error and
// every profile has the minimum required fields.
func TestCatalogueLoads(t *testing.T) {
	devs, err := All()
	if err != nil {
		t.Fatalf("load catalogue: %v", err)
	}
	if len(devs) == 0 {
		t.Fatal("catalogue is empty")
	}
	for _, d := range devs {
		if d.Codename == "" {
			t.Errorf("device with empty codename: %+v", d)
		}
		if d.Model == "" {
			t.Errorf("device %q has empty model", d.Codename)
		}
		if d.Backend == "" {
			t.Errorf("device %q has empty backend", d.Codename)
		}
		if len(d.Zones) == 0 {
			t.Errorf("device %q has no zones", d.Codename)
		}
		for _, z := range d.Zones {
			if z.Name == "" {
				t.Errorf("device %q has a zone with empty name", d.Codename)
			}
		}
		if d.Backend == BackendSysfsNothLeds || d.Backend == BackendSysfsAW210xx {
			if d.Sysfs == nil {
				t.Errorf("device %q has sysfs backend but no sysfs config", d.Codename)
			}
		}
	}
}

// TestLookupMatchesLongest verifies the longest-match rule — "A001T" must not
// be shadowed by "A001".
func TestLookupMatchesLongest(t *testing.T) {
	cases := []struct {
		input    string
		wantName string // expected profile model
	}{
		{"Nothing Phone (3a) Lite A001T", "Nothing Phone (3a) Lite"},
		{"Nothing Phone (3a)", "Nothing Phone (3a)"},
		{"galaxian", "Nothing Phone (3a) Lite"},
		{"spacewar", "Nothing Phone (1)"},
		{"A063", "Nothing Phone (1)"},
		{"pong", "Nothing Phone (2)"},
		{"pacman", "Nothing Phone (2a)"},
	}
	for _, tc := range cases {
		d, ok := Lookup(tc.input)
		if !ok {
			t.Errorf("Lookup(%q): no match", tc.input)
			continue
		}
		if d.Model != tc.wantName {
			t.Errorf("Lookup(%q): got %q, want %q", tc.input, d.Model, tc.wantName)
		}
	}
}

// TestGalaxianProfileSpecifics locks in the verified galaxian details so a
// future JSON edit can't silently break the 3a Lite flow.
func TestGalaxianProfileSpecifics(t *testing.T) {
	d, ok := Lookup("galaxian")
	if !ok {
		t.Fatal("galaxian profile missing")
	}
	if d.Backend != BackendSysfsNothLeds {
		t.Errorf("galaxian backend: got %q, want %q", d.Backend, BackendSysfsNothLeds)
	}
	if d.Sysfs == nil {
		t.Fatal("galaxian sysfs config missing")
	}
	if !d.Sysfs.BrightnessIsBinary {
		t.Error("galaxian brightness should be binary (verified on-device)")
	}
	if d.Sysfs.StateSemantics != StateBlinkPeriodMs {
		t.Errorf("galaxian state semantics: got %q, want %q",
			d.Sysfs.StateSemantics, StateBlinkPeriodMs)
	}
	if d.Sysfs.MaxBrightness != 255 {
		t.Errorf("galaxian max_brightness: got %d, want 255", d.Sysfs.MaxBrightness)
	}
	if !d.Supports(CapBlink) {
		t.Error("galaxian should declare blink capability")
	}
	if d.Supports(CapBreath) {
		t.Error("galaxian should NOT declare breath capability (no hardware PWM)")
	}
	if len(d.Zones) != 1 {
		t.Errorf("galaxian zones: got %d, want 1", len(d.Zones))
	}
}
