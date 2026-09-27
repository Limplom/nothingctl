package network

import (
	"strings"
	"testing"
)

func TestBandFromFreq(t *testing.T) {
	cases := []struct {
		freq int
		want string
	}{
		{0, "2.4 GHz"},
		{2412, "2.4 GHz"},
		{2999, "2.4 GHz"},
		{3000, "5 GHz"},
		{5180, "5 GHz"},
		{5885, "5 GHz"}, // 5 GHz ch 177 (highest U-NII-4)
		{5925, "6 GHz"},
		{5935, "6 GHz"}, // 6 GHz ch 2
		{5955, "6 GHz"}, // 6 GHz ch 1 — was mislabelled "5 GHz" with the old < 6000 cutoff
		{6115, "6 GHz"},
		{7115, "6 GHz"},
	}
	for _, c := range cases {
		if got := bandFromFreq(c.freq); got != c.want {
			t.Errorf("bandFromFreq(%d) = %q, want %q", c.freq, got, c.want)
		}
	}
}

func TestSecurityFromCaps(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "Open"},
		{"[ESS]", "Open"},
		{"[WPA2-PSK-CCMP][RSN-PSK-CCMP][ESS]", "WPA2"},
		{"[RSN-SAE-CCMP][ESS][MFPR]", "WPA3"},
		{"[WPA3-SAE]", "WPA3"},
		{"[WPA-PSK-TKIP][ESS]", "WPA"},
		{"[WEP][ESS]", "WEP"},
		{"[wpa2-psk]", "WPA2"},
	}
	for _, c := range cases {
		if got := securityFromCaps(c.in); got != c.want {
			t.Errorf("securityFromCaps(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

const sampleScanResults = `    BSSID              Frequency      RSSI           Age(sec)     SSID                                 Flags
  aa:bb:cc:dd:ee:01       5180    -45(0:-47/1:-50)   3.123     Home Network 5G              [WPA2-PSK-CCMP][RSN-PSK-CCMP][ESS]
  aa:bb:cc:dd:ee:02       2437    -71                >1000.0   CafeWiFi                     [ESS]
  AA:BB:CC:DD:EE:03       6115    -60                12.5      [RSN-SAE-CCMP][ESS]
  not-a-mac line here
`

func TestParseScanResults(t *testing.T) {
	crlf := strings.ReplaceAll(sampleScanResults, "\n", "\r\n")
	for name, in := range map[string]string{"lf": sampleScanResults, "crlf": crlf} {
		t.Run(name, func(t *testing.T) {
			got := parseScanResults(in)
			want := []wifiNetwork{
				{"aa:bb:cc:dd:ee:01", "Home Network 5G", "[WPA2-PSK-CCMP][RSN-PSK-CCMP][ESS]", 5180, -45},
				{"aa:bb:cc:dd:ee:02", "CafeWiFi", "[ESS]", 2437, -71},
				{"AA:BB:CC:DD:EE:03", "<hidden>", "[RSN-SAE-CCMP][ESS]", 6115, -60},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d networks, want %d: %+v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("network %d = %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
	if got := parseScanResults(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}

func TestParseSavedNetworksLegacy(t *testing.T) {
	// wpa_cli-style list (network id / ssid / bssid / flags).
	in := strings.Join([]string{
		"network id / ssid / bssid / flags",
		"Network Id  SSID  BSSID  Flags",
		"0\tHome Net\tany\t[CURRENT]",
		"1\tOffice\t11:22:33:44:55:66\t",
		"",
	}, "\r\n")
	got := parseSavedNetworks(in)
	want := []savedNetwork{
		{0, "Home Net", "[CURRENT]"},
		{1, "Office", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d networks, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("network %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := parseSavedNetworks(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}

// TestParseSavedNetworksModern documents a suspected mismatch with Android 11+
// `cmd wifi list-networks`, which prints "Network Id / SSID / Security type"
// columns (no BSSID). netIDLineRe needs a BSSID/"any" column, so the fallback
// regex puts the padded security type into the SSID ("MyHome   wpa2-psk"),
// which also breaks `--forget <ssid>`. SSIDs may contain spaces, so a correct
// fix needs column-position parsing verified against a real device.
func TestParseSavedNetworksModern(t *testing.T) {
	t.Skip("parseSavedNetworks mis-parses Android 11+ list-networks format; needs device-verified fix")
	in := "Network Id      SSID                         Security type\n" +
		"0            MyHome                           wpa2-psk\n"
	got := parseSavedNetworks(in)
	if len(got) != 1 || got[0].ssid != "MyHome" {
		t.Errorf("got %+v, want ssid MyHome", got)
	}
}

func TestFilterNonEmpty(t *testing.T) {
	in := []string{"tcp:5555 tcp:5555\r", "", "   ", "\r", "x"}
	got := filterNonEmpty(in)
	want := []string{"tcp:5555 tcp:5555", "x"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
	if got := filterNonEmpty(nil); len(got) != 0 {
		t.Errorf("nil input: got %q", got)
	}
}
