package permissions

import (
	"strings"
	"testing"
)

const samplePkgPerms = `    requested permissions:
      android.permission.CAMERA
      android.permission.INTERNET
      android.permission.ACCESS_FINE_LOCATION
    install permissions:
      android.permission.INTERNET: granted=true
    runtime permissions:
      android.permission.CAMERA: granted=true, flags=[ USER_SET|USER_SENSITIVE_WHEN_GRANTED ]
      android.permission.ACCESS_FINE_LOCATION: granted=false, flags=[ USER_SET ]
      android.permission.RECORD_AUDIO: granted=true, flags=[ USER_SET ]
      com.example.permission.CUSTOM: granted=true
`

func TestShortPerm(t *testing.T) {
	cases := []struct{ in, want string }{
		{"android.permission.CAMERA", "CAMERA"},
		{"com.example.permission.X", "com.example.permission.X"},
		{"", ""},
	}
	for _, c := range cases {
		if got := shortPerm(c.in); got != c.want {
			t.Errorf("shortPerm(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseGrantedDangerous(t *testing.T) {
	crlf := strings.ReplaceAll(samplePkgPerms, "\n", "\r\n")
	for name, in := range map[string]string{"lf": samplePkgPerms, "crlf": crlf} {
		got := parseGrantedDangerous(in)
		want := []string{"android.permission.CAMERA", "android.permission.RECORD_AUDIO"}
		if len(got) != len(want) {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: entry %d = %q, want %q", name, i, got[i], want[i])
			}
		}
	}
	if got := parseGrantedDangerous(""); len(got) != 0 {
		t.Errorf("empty input: got %q", got)
	}
}

func TestParseAllDangerous(t *testing.T) {
	crlf := strings.ReplaceAll(samplePkgPerms, "\n", "\r\n")
	for name, in := range map[string]string{"lf": samplePkgPerms, "crlf": crlf} {
		granted, notGranted := parseAllDangerous(in)
		// Ordered by the dangerousPermissions list, not by input order.
		wantGranted := []string{"android.permission.CAMERA", "android.permission.RECORD_AUDIO"}
		if len(granted) != len(wantGranted) {
			t.Fatalf("%s: granted = %q, want %q", name, granted, wantGranted)
		}
		for i := range wantGranted {
			if granted[i] != wantGranted[i] {
				t.Errorf("%s: granted[%d] = %q, want %q", name, i, granted[i], wantGranted[i])
			}
		}
		if len(granted)+len(notGranted) != len(dangerousPermissions) {
			t.Errorf("%s: granted+notGranted = %d, want %d", name, len(granted)+len(notGranted), len(dangerousPermissions))
		}
		for _, p := range notGranted {
			if p == "android.permission.CAMERA" || p == "android.permission.RECORD_AUDIO" {
				t.Errorf("%s: %s in both lists", name, p)
			}
		}
	}
	granted, notGranted := parseAllDangerous("")
	if len(granted) != 0 || len(notGranted) != len(dangerousPermissions) {
		t.Errorf("empty input: granted=%d notGranted=%d", len(granted), len(notGranted))
	}
}
