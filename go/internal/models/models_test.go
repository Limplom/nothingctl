package models

import "testing"

func intp(v int) *int { return &v }

func TestIsOutdated(t *testing.T) {
	cases := []struct {
		name string
		ms   MagiskStatus
		want bool
	}{
		{"app not installed", MagiskStatus{AppInstalled: false, InstalledVersion: intp(26000), LatestVersion: intp(30700)}, false},
		{"latest unknown (offline)", MagiskStatus{AppInstalled: true, InstalledVersion: intp(26000)}, false},
		{"installed unknown", MagiskStatus{AppInstalled: true, LatestVersion: intp(30700)}, false},
		{"older", MagiskStatus{AppInstalled: true, InstalledVersion: intp(30600), LatestVersion: intp(30700)}, true},
		{"equal", MagiskStatus{AppInstalled: true, InstalledVersion: intp(30700), LatestVersion: intp(30700)}, false},
		{"newer (canary)", MagiskStatus{AppInstalled: true, InstalledVersion: intp(30701), LatestVersion: intp(30700)}, false},
	}
	for _, tc := range cases {
		if got := tc.ms.IsOutdated(); got != tc.want {
			t.Errorf("%s: IsOutdated() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStateLabel(t *testing.T) {
	cases := []struct {
		name string
		ms   MagiskStatus
		want string
	}{
		{"not installed", MagiskStatus{}, "NOT INSTALLED"},
		{"app only", MagiskStatus{AppInstalled: true, InstalledVersion: intp(30700)}, "APP ONLY (boot not patched)"},
		{"outdated", MagiskStatus{AppInstalled: true, RootActive: true, InstalledVersion: intp(30600), LatestVersion: intp(30700)}, "ACTIVE but OUTDATED (v30600 < v30700)"},
		{"current", MagiskStatus{AppInstalled: true, RootActive: true, InstalledVersion: intp(30700), LatestVersion: intp(30700)}, "ACTIVE  v30700"},
		{"active, offline", MagiskStatus{AppInstalled: true, RootActive: true, InstalledVersion: intp(30700)}, "ACTIVE  v30700"},
		{"active, version unknown", MagiskStatus{AppInstalled: true, RootActive: true}, "ACTIVE  v0"},
	}
	for _, tc := range cases {
		if got := tc.ms.StateLabel(); got != tc.want {
			t.Errorf("%s: StateLabel() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
