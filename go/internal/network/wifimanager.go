package network

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var macPrefixRe   = regexp.MustCompile(`^[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:`)
var rssiLeadingRe = regexp.MustCompile(`^(-?\d+)`)
var ageFieldRe    = regexp.MustCompile(`^[>\d][\d,.*]*$`)
var netIDLineRe   = regexp.MustCompile(`^\s*(\d+)\s+(.+?)\s+(any|\S+:\S+:\S+:\S+:\S+:\S+)\s*(.*)?$`)
var netIDFbRe     = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)
var netHeaderRe   = regexp.MustCompile(`(?i)^\s*Network\s+Id`)
var numericRe     = regexp.MustCompile(`^\d+$`)

func bandFromFreq(freq int) string {
	switch {
	case freq < 3000:
		return "2.4 GHz"
	case freq < 6000:
		return "5 GHz"
	default:
		return "6 GHz"
	}
}

func securityFromCaps(caps string) string {
	upper := strings.ToUpper(caps)
	switch {
	case strings.Contains(upper, "WPA3") || strings.Contains(upper, "SAE"):
		return "WPA3"
	case strings.Contains(upper, "WPA2"):
		return "WPA2"
	case strings.Contains(upper, "WPA"):
		return "WPA"
	case strings.Contains(upper, "WEP"):
		return "WEP"
	default:
		return "Open"
	}
}

type wifiNetwork struct {
	bssid, ssid, caps string
	freq, rssi        int
}

func parseScanResults(output string) []wifiNetwork {
	var networks []wifiNetwork
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !macPrefixRe.MatchString(strings.TrimSpace(line)) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		bssid := parts[0]
		var freq int
		fmt.Sscanf(parts[1], "%d", &freq)
		rssiStr := ""
		if m := rssiLeadingRe.FindStringSubmatch(parts[2]); m != nil {
			rssiStr = m[1]
		}
		var rssi int
		fmt.Sscanf(rssiStr, "%d", &rssi)

		skip := 3
		if skip < len(parts) && ageFieldRe.MatchString(parts[skip]) {
			skip++
		}
		rest := parts[skip:]
		capsIdx := len(rest)
		for i, p := range rest {
			if strings.HasPrefix(p, "[") {
				capsIdx = i
				break
			}
		}
		ssid := "<hidden>"
		if capsIdx > 0 {
			ssid = strings.Join(rest[:capsIdx], " ")
		}
		caps := strings.Join(rest[capsIdx:], " ")
		networks = append(networks, wifiNetwork{
			bssid: bssid,
			ssid:  ssid,
			caps:  caps,
			freq:  freq,
			rssi:  rssi,
		})
	}
	return networks
}

// ActionWifiScan scans and lists nearby WiFi networks sorted by signal strength.
func ActionWifiScan(serial, model string) error {
	raw := adb.ShellStr(serial, "cmd wifi list-scan-results")
	networks := parseScanResults(raw)

	if len(networks) == 0 {
		// Trigger a fresh scan and retry
		adb.ShellStr(serial, "cmd wifi start-scan")
		time.Sleep(3 * time.Second)
		raw = adb.ShellStr(serial, "cmd wifi list-scan-results")
		networks = parseScanResults(raw)
	}

	// Sort strongest first (bubble sort descending by rssi)
	for i := 0; i < len(networks); i++ {
		for j := i + 1; j < len(networks); j++ {
			if networks[j].rssi > networks[i].rssi {
				networks[i], networks[j] = networks[j], networks[i]
			}
		}
	}

	fmt.Printf("\n  WiFi Scan \u2014 %s\n", model)
	fmt.Println("  (trigger: cmd wifi start-scan)\n")

	if len(networks) == 0 {
		fmt.Println("  No scan results available.")
		return nil
	}

	colSSID := 24
	for _, n := range networks {
		if len(n.ssid) > colSSID {
			colSSID = len(n.ssid)
		}
	}

	headerFmt := fmt.Sprintf("  %%-4s %%-%ds  %%-9s %%-7s %%s", colSSID)
	header := fmt.Sprintf(headerFmt, "#", "SSID", "Band", "RSSI", "Security")
	fmt.Println(header)
	fmt.Println("  " + strings.Repeat("\u2500", len(header)-2))

	rowFmt := fmt.Sprintf("  %%-4d %%-%ds  %%-9s %%-7d %%s", colSSID)
	for idx, net := range networks {
		band := bandFromFreq(net.freq)
		security := securityFromCaps(net.caps)
		fmt.Printf(rowFmt+"\n", idx+1, net.ssid, band, net.rssi, security)
	}
	fmt.Println()
	return nil
}

type savedNetwork struct {
	id    int
	ssid  string
	flags string
}

func parseSavedNetworks(raw string) []savedNetwork {
	var networks []savedNetwork
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if netHeaderRe.MatchString(line) {
			continue
		}
		if m := netIDLineRe.FindStringSubmatch(line); m != nil {
			var id int
			fmt.Sscanf(m[1], "%d", &id)
			ssid := strings.TrimSpace(m[2])
			flags := strings.TrimSpace(m[4])
			networks = append(networks, savedNetwork{id: id, ssid: ssid, flags: flags})
		} else if m := netIDFbRe.FindStringSubmatch(line); m != nil {
			var id int
			fmt.Sscanf(m[1], "%d", &id)
			ssid := strings.TrimSpace(m[2])
			networks = append(networks, savedNetwork{id: id, ssid: ssid})
		}
	}
	return networks
}

// ActionWifiProfiles lists saved WiFi networks or forgets one by SSID/ID.
// forget="" means list mode; forget="<ssid or id>" to remove.
func ActionWifiProfiles(serial, model, forget string) error {
	raw := adb.ShellStr(serial, "cmd wifi list-networks")
	if raw == "" {
		raw = adb.ShellStr(serial, "cmd wifi list-saved-networks")
	}

	networks := parseSavedNetworks(raw)

	if forget != "" {
		var targetID int
		var targetSSID string

		if numericRe.MatchString(strings.TrimSpace(forget)) {
			fmt.Sscanf(strings.TrimSpace(forget), "%d", &targetID)
			for _, n := range networks {
				if n.id == targetID {
					targetSSID = n.ssid
					break
				}
			}
		} else {
			found := false
			for _, n := range networks {
				if n.ssid == forget {
					targetID = n.id
					targetSSID = n.ssid
					found = true
					break
				}
			}
			if !found {
				return nterrors.AdbError(fmt.Sprintf(
					"No saved network found with SSID %q.\nRun without --forget to list all saved networks.", forget))
			}
		}

		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("cmd wifi forget-network %d", targetID)})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf(
				"Failed to forget network id=%d: %s", targetID, strings.TrimSpace(stderr)))
		}

		label := fmt.Sprintf("id=%d", targetID)
		if targetSSID != "" {
			label = fmt.Sprintf("%s (id=%d)", targetSSID, targetID)
		}
		fmt.Printf("  [OK] Forgot network: %s\n", label)
		return nil
	}

	// List mode
	fmt.Printf("\n  Saved WiFi Networks \u2014 %s\n\n", model)

	if len(networks) == 0 {
		fmt.Println("  No saved networks found.")
		return nil
	}

	colSSID := 22
	for _, n := range networks {
		if len(n.ssid) > colSSID {
			colSSID = len(n.ssid)
		}
	}

	headerFmt := fmt.Sprintf("  %%-4s %%-4s %%-%ds  %%s", colSSID)
	header := fmt.Sprintf(headerFmt, "#", "ID", "SSID", "Status")
	fmt.Println(header)
	fmt.Println("  " + strings.Repeat("\u2500", len(header)-2))

	rowFmt := fmt.Sprintf("  %%-4d %%-4d %%-%ds  %%s", colSSID)
	for idx, net := range networks {
		status := "\u2014"
		if strings.Contains(strings.ToUpper(net.flags), "CURRENT") {
			status = "current"
		}
		fmt.Printf(rowFmt+"\n", idx, net.id, net.ssid, status)
	}
	fmt.Println()
	return nil
}
