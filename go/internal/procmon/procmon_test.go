package procmon

import (
	"strings"
	"testing"
)

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

const samplePS = `  PID  PPID USER           NAME                        S
    1     0 root           init                        S
  812     1 system         system_server               S
 4321   812 u0_a123        com.example.app             R
 4400   812 u0_i9          com.example.app:isolated    S
 5000     1 shell          sh                          Z
garbage line
  abc     1 root           broken                      S
`

func TestParsePS(t *testing.T) {
	for name, in := range map[string]string{"lf": samplePS, "crlf": crlf(samplePS)} {
		t.Run(name, func(t *testing.T) {
			got := parsePS(in)
			want := []process{
				{1, 0, "root", "init", "S"},
				{812, 1, "system", "system_server", "S"},
				{4321, 812, "u0_a123", "com.example.app", "R"},
				{4400, 812, "u0_i9", "com.example.app:isolated", "S"},
				{5000, 1, "shell", "sh", "Z"},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d procs, want %d: %+v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("proc %d = %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
	if got := parsePS(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}

func TestUserClassification(t *testing.T) {
	cases := []struct {
		user                  string
		app, isolated, system bool
	}{
		{"u0_a123", true, false, false},
		{"u0_i9", false, true, false},
		{"root", false, false, true},
		{"system", false, false, true},
		{"bluetooth", false, false, true},
		{"u10_a5", false, false, false},
		{"u0_a", false, false, false},
		{"", false, false, false},
	}
	for _, c := range cases {
		if got := isUserApp(c.user); got != c.app {
			t.Errorf("isUserApp(%q) = %v, want %v", c.user, got, c.app)
		}
		if got := isIsolated(c.user); got != c.isolated {
			t.Errorf("isIsolated(%q) = %v, want %v", c.user, got, c.isolated)
		}
		if got := isSystem(c.user); got != c.system {
			t.Errorf("isSystem(%q) = %v, want %v", c.user, got, c.system)
		}
	}
}

const sampleDeviceIdle = `  mLightState=ACTIVE mLightAlarmTime=0
  mState=IDLE mInactiveTimeout=+30m0s0ms
  mScreenOn=true
`

func TestParseDozeState(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleDeviceIdle, "crlf": crlf(sampleDeviceIdle)} {
		if got := parseDozeState(in); got != "IDLE" {
			t.Errorf("%s: parseDozeState = %q, want IDLE", name, got)
		}
		if got := parseLightState(in); got != "ACTIVE" {
			t.Errorf("%s: parseLightState = %q, want ACTIVE", name, got)
		}
		if !parseScreenOn(in) {
			t.Errorf("%s: parseScreenOn = false, want true", name)
		}
	}
	if got := parseDozeState(""); got != "UNKNOWN" {
		t.Errorf("empty: parseDozeState = %q, want UNKNOWN", got)
	}
	if got := parseLightState("garbage"); got != "UNKNOWN" {
		t.Errorf("garbage: parseLightState = %q, want UNKNOWN", got)
	}
}

func TestParseScreenOn(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"mScreenOn=true", true},
		{"mScreenOn = FALSE", false},
		{"  Interactive: true\r\n", true},
		{"mScreenOn=false\nInteractive: true", false}, // mScreenOn wins
		{"", false},
	}
	for _, c := range cases {
		if got := parseScreenOn(c.in); got != c.want {
			t.Errorf("parseScreenOn(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParsePluggedIn(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"  AC powered: false\r\n  USB powered: false\r\n  Wireless powered: false\r\n", false},
		{"  AC powered: false\n  USB powered: true\n", true},
		{"  Wireless powered: true\n", true},
		{"", false},
	}
	for _, c := range cases {
		if got := parsePluggedIn(c.in); got != c.want {
			t.Errorf("parsePluggedIn(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseWhitelist(t *testing.T) {
	in := crlf(`system-excidle,com.android.providers.downloads,10045
system,com.google.android.gms,10120
user,com.example.app,10200
user,com.example.app,10200
com.plain.pkg
UID=10300: com.uid.pkg
nodotword
`)
	got := parseWhitelist(in)
	want := []string{
		"com.android.providers.downloads",
		"com.google.android.gms",
		"com.example.app",
		"com.plain.pkg",
		"com.uid.pkg",
	}
	// "nodotword" is dropped: whitelist entries must look like package names.
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
	if got := parseWhitelist(""); len(got) != 0 {
		t.Errorf("empty input: got %q", got)
	}
}

const sampleLocationDump = `Location Manager State:
  Last Known Locations:
    gps: Location[gps 52.520008,13.404954 hAcc=5.0 et=+1h2m3s alt=34.0]
    network: Location[network -33.868820,-151.209290 acc=20 et=+5m]
    passive: null
    fused: Location[fused 48.8566,2.3522 et=+10s]
  Geofences:
    gps: Location[gps 1.0,1.0]
`

func TestParseLastKnownLocations(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleLocationDump, "crlf": crlf(sampleLocationDump)} {
		t.Run(name, func(t *testing.T) {
			got := parseLastKnownLocations(in)
			if len(got) != 4 {
				t.Fatalf("got %d entries, want 4: %+v", len(got), got)
			}
			type flat struct {
				prov     string
				hasLoc   bool
				lat, lon float64
				acc, age string
			}
			want := []flat{
				{"gps", true, 52.520008, 13.404954, "5.0m", "+1h2m3s"}, // hAcc= matches accRe
				{"network", true, -33.868820, -151.209290, "20m", "+5m"},
				{"passive", false, 0, 0, "", ""},
				{"fused", true, 48.8566, 2.3522, "", "+10s"},
			}
			for i, w := range want {
				e := got[i]
				if e.provider != w.prov {
					t.Errorf("entry %d provider = %q, want %q", i, e.provider, w.prov)
				}
				if (e.lat != nil) != w.hasLoc {
					t.Errorf("entry %d hasLoc = %v, want %v", i, e.lat != nil, w.hasLoc)
					continue
				}
				if w.hasLoc && (*e.lat != w.lat || *e.lon != w.lon) {
					t.Errorf("entry %d coords = %v,%v, want %v,%v", i, *e.lat, *e.lon, w.lat, w.lon)
				}
				if e.accuracy != w.acc {
					t.Errorf("entry %d accuracy = %q, want %q", i, e.accuracy, w.acc)
				}
				if e.age != w.age {
					t.Errorf("entry %d age = %q, want %q", i, e.age, w.age)
				}
			}
		})
	}
	if got := parseLastKnownLocations("gps: Location[gps 1.0,1.0]\n"); len(got) != 0 {
		t.Errorf("no section header: got %+v", got)
	}
}

func TestParseProviders(t *testing.T) {
	in := crlf(`  gps provider [enabled]:
  network provider: disabled
  passive provider [enabled]
  fused provider [enabled]
`)
	got := parseProviders(in)
	want := map[string]bool{"gps": true, "network": false, "passive": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if gv, ok := got[k]; !ok || gv != v {
			t.Errorf("provider %q = %v (present %v), want %v", k, gv, ok, v)
		}
	}
}

func TestParseFineLocationApps(t *testing.T) {
	in := crlf(`Package com.google.android.apps.maps uid=10150
com.example.tracker
  not a package
singleword
`)
	got := parseFineLocationApps(in)
	want := []string{"com.google.android.apps.maps", "com.example.tracker"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFormatCoord(t *testing.T) {
	cases := []struct {
		lat, lon float64
		want     string
	}{
		{52.520008, 13.404954, "52.5200° N, 13.4050° E"},
		{-33.86882, -151.20929, "33.8688° S, 151.2093° W"},
		{0, 0, "0.0000° N, 0.0000° E"},
	}
	for _, c := range cases {
		if got := formatCoord(c.lat, c.lon); got != c.want {
			t.Errorf("formatCoord(%v,%v) = %q, want %q", c.lat, c.lon, got, c.want)
		}
	}
}
