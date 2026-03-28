// Package network provides network info, DNS, and port-forwarding management.
package network

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// DNS aliases → real hostnames
var dnsAliases = map[string]string{
	"cloudflare": "one.one.one.one",
	"1.1.1.1":    "one.one.one.one",
	"adguard":    "dns.adguard.com",
	"google":     "dns.google",
	"quad9":      "dns.quad9.net",
}

var ssidRe      = regexp.MustCompile(`SSID:\s*"([^"]*)"`)
var bssidRe     = regexp.MustCompile(`BSSID:\s*([0-9a-fA-F:]{17})`)
var rssiRe      = regexp.MustCompile(`RSSI:\s*(-?\d+)`)
var linkSpeedRe = regexp.MustCompile(`Link speed:\s*(\d+)`)
var freqRe      = regexp.MustCompile(`Frequency:\s*(\d+)`)
var ipWifiRe    = regexp.MustCompile(`IP:\s*/([\d.]+)`)
var inetRe      = regexp.MustCompile(`inet\s+([\d.]+)/`)
var dns12Re     = regexp.MustCompile(`net\.dns[12]\]:\s*\[([^\]]+)\]`)
var ndcDNSRe    = regexp.MustCompile(`DNS servers:\s*(.+)`)

// ActionNetworkInfo displays network information.
func ActionNetworkInfo(serial, model string) error {
	var wifiRaw, dnsRaw, operator, netType, connRaw string
	var pdnsMode, pdnsProvider string

	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); wifiRaw = adb.ShellStr(serial, "cmd wifi status") }()
	go func() {
		defer wg.Done()
		dnsRaw = adb.ShellStr(serial, "getprop | grep -E 'net\\.dns[12]|dhcp.*dns'")
	}()
	go func() { defer wg.Done(); operator = adb.ShellStr(serial, "getprop gsm.operator.alpha") }()
	go func() { defer wg.Done(); netType = adb.ShellStr(serial, "getprop gsm.network.type") }()
	go func() {
		defer wg.Done()
		connRaw = adb.ShellStr(serial, "dumpsys connectivity | grep -E 'NetworkAgentInfo.*CONNECTED|activeNetwork'")
	}()
	go func() {
		defer wg.Done()
		pdnsMode = adb.Setting(serial, "global", "private_dns_mode")
		pdnsProvider = adb.Setting(serial, "global", "private_dns_specifier")
	}()
	wg.Wait()

	var ssid, bssid, rssi, linkSpeed, freq, ipWifi string

	// Find WifiInfo: line
	for _, line := range strings.Split(wifiRaw, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.Contains(line, "WifiInfo:") {
			continue
		}
		if m := ssidRe.FindStringSubmatch(line); m != nil {
			ssid = m[1]
		}
		if m := bssidRe.FindStringSubmatch(line); m != nil {
			bssid = m[1]
		}
		if m := rssiRe.FindStringSubmatch(line); m != nil {
			rssi = m[1]
		}
		if m := linkSpeedRe.FindStringSubmatch(line); m != nil {
			linkSpeed = m[1]
		}
		if m := freqRe.FindStringSubmatch(line); m != nil {
			freq = m[1]
		}
		if m := ipWifiRe.FindStringSubmatch(line); m != nil {
			ipWifi = m[1]
		}
		break
	}

	// Fallback IP from ip addr show wlan0
	if ipWifi == "" {
		wlanRaw := adb.ShellStr(serial, "ip addr show wlan0")
		if m := inetRe.FindStringSubmatch(wlanRaw); m != nil {
			ipWifi = m[1]
		}
	}

	// Frequency band
	freqBand := ""
	if freq != "" {
		var f int
		fmt.Sscanf(freq, "%d", &f)
		switch {
		case f < 3000:
			freqBand = "2.4 GHz"
		case f < 6000:
			freqBand = "5 GHz"
		default:
			freqBand = "6 GHz"
		}
	}

	// DNS servers
	var dnsServers []string
	for _, line := range strings.Split(dnsRaw, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := dns12Re.FindStringSubmatch(line); m != nil {
			if v := strings.TrimSpace(m[1]); v != "" {
				dnsServers = append(dnsServers, v)
			}
		}
	}
	if len(dnsServers) == 0 {
		ndcRaw := adb.ShellStr(serial, "ndc resolver getnetworkinfo 100")
		for _, line := range strings.Split(ndcRaw, "\n") {
			line = strings.TrimRight(line, "\r")
			if m := ndcDNSRe.FindStringSubmatch(line); m != nil {
				for _, d := range strings.Fields(m[1]) {
					if d != "" {
						dnsServers = append(dnsServers, d)
					}
				}
				break
			}
		}
	}

	// Handle comma-separated multi-SIM values
	if strings.Contains(operator, ",") {
		for _, o := range strings.Split(operator, ",") {
			if strings.TrimSpace(o) != "" {
				operator = strings.TrimSpace(o)
				break
			}
		}
	}
	if strings.Contains(netType, ",") {
		for _, n := range strings.Split(netType, ",") {
			if strings.TrimSpace(n) != "" {
				netType = strings.TrimSpace(n)
				break
			}
		}
	}

	// Active connection type
	connType := "Unknown"
	upper := strings.ToUpper(connRaw)
	switch {
	case strings.Contains(upper, "WIFI"):
		connType = "WiFi"
	case strings.Contains(upper, "CELLULAR") || strings.Contains(upper, "MOBILE"):
		connType = "Mobile"
	case connRaw == "":
		connType = "None"
	}

	// Output
	fmt.Printf("\n  Network Info \u2014 %s\n\n", model)
	fmt.Printf("  %-18s: %s\n", "Connection", connType)
	fmt.Println()

	fmt.Println("  WiFi")
	if ssid == "" {
		fmt.Printf("  %-18s: %s\n", "  SSID", "(not connected)")
	} else {
		fmt.Printf("  %-18s: %s\n", "  SSID", ssid)
		bssidDisplay := bssid
		if bssidDisplay == "" {
			bssidDisplay = "n/a"
		}
		fmt.Printf("  %-18s: %s\n", "  BSSID", bssidDisplay)
		if rssi != "" {
			fmt.Printf("  %-18s: %s dBm\n", "  Signal", rssi)
		} else {
			fmt.Printf("  %-18s: n/a\n", "  Signal")
		}
		if linkSpeed != "" {
			fmt.Printf("  %-18s: %s Mbps\n", "  Link Speed", linkSpeed)
		} else {
			fmt.Printf("  %-18s: n/a\n", "  Link Speed")
		}
		freqDisplay := "n/a"
		if freq != "" && freqBand != "" {
			freqDisplay = fmt.Sprintf("%s MHz (%s)", freq, freqBand)
		} else if freq != "" {
			freqDisplay = freq
		}
		fmt.Printf("  %-18s: %s\n", "  Frequency", freqDisplay)
		ipDisplay := ipWifi
		if ipDisplay == "" {
			ipDisplay = "n/a"
		}
		fmt.Printf("  %-18s: %s\n", "  IP Address", ipDisplay)
	}
	fmt.Println()

	fmt.Println("  DNS")
	if len(dnsServers) > 0 {
		for i, srv := range dnsServers {
			fmt.Printf("  %-18s: %s\n", fmt.Sprintf("  Server %d", i+1), srv)
		}
	} else {
		fmt.Printf("  %-18s: (not available)\n", "  Servers")
	}

	pdnsDisplay := "off"
	if pdnsMode != "" {
		pdnsDisplay = pdnsMode
	}
	if pdnsMode == "hostname" && pdnsProvider != "" {
		pdnsDisplay = fmt.Sprintf("hostname (%s)", pdnsProvider)
	} else if pdnsMode == "opportunistic" {
		pdnsDisplay = "automatic"
	}
	fmt.Printf("  %-18s: %s\n", "  Private DNS", pdnsDisplay)
	fmt.Println()

	fmt.Println("  Mobile")
	opDisplay := operator
	if opDisplay == "" {
		opDisplay = "n/a"
	}
	ntDisplay := netType
	if ntDisplay == "" {
		ntDisplay = "n/a"
	}
	fmt.Printf("  %-18s: %s\n", "  Operator", opDisplay)
	fmt.Printf("  %-18s: %s\n", "  Network Type", ntDisplay)
	fmt.Println()
	return nil
}

// ActionDNSSet sets or displays Private DNS configuration.
func ActionDNSSet(serial, model, provider string) error {
	if provider == "" {
		// Read-only mode
		mode := adb.Setting(serial, "global", "private_dns_mode")
		specifier := adb.Setting(serial, "global", "private_dns_specifier")

		fmt.Printf("\n  Private DNS \u2014 %s\n\n", model)
		modeDisplay := "off"
		if mode != "" {
			modeDisplay = mode
		}
		if mode == "opportunistic" {
			modeDisplay = "automatic (opportunistic)"
		}
		fmt.Printf("  %-12s: %s\n", "Mode", modeDisplay)
		if mode == "hostname" {
			sp := specifier
			if sp == "" {
				sp = "(not set)"
			}
			fmt.Printf("  %-12s: %s\n", "Provider", sp)
		}
		fmt.Println()
		return nil
	}

	if strings.ToLower(provider) == "off" {
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"settings", "put", "global", "private_dns_mode", "off"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Failed to disable Private DNS: %s", strings.TrimSpace(stderr)))
		}
		fmt.Printf("  Private DNS disabled on %s.\n", model)
		return nil
	}

	hostname := provider
	if alias, ok := dnsAliases[strings.ToLower(provider)]; ok {
		hostname = alias
	}

	_, stderr1, code1 := adb.Run([]string{"adb", "-s", serial, "shell",
		"settings", "put", "global", "private_dns_specifier", hostname})
	_, stderr2, code2 := adb.Run([]string{"adb", "-s", serial, "shell",
		"settings", "put", "global", "private_dns_mode", "hostname"})

	if code1 != 0 || code2 != 0 {
		errMsg := strings.TrimSpace(stderr1 + stderr2)
		return nterrors.AdbError(fmt.Sprintf("Failed to set Private DNS: %s", errMsg))
	}

	aliasNote := ""
	if hostname != strings.ToLower(provider) {
		aliasNote = fmt.Sprintf(" (alias for %s)", hostname)
	}
	fmt.Printf("  Private DNS set to '%s'%s on %s.\n", hostname, aliasNote, model)
	return nil
}

// ActionPortForward manages ADB port forwards.
// local and remote are port numbers as strings (e.g. "8080"). Pass empty strings for list mode.
func ActionPortForward(serial, model, local, remote string, clear bool) error {
	if clear {
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "forward", "--remove-all"})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Failed to remove forwards: %s", strings.TrimSpace(stderr)))
		}
		fmt.Printf("  All port forwards removed on %s.\n", model)
		return nil
	}

	if local != "" && remote != "" {
		localSpec := "tcp:" + local
		remoteSpec := "tcp:" + remote
		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "forward", localSpec, remoteSpec})
		if code != 0 {
			return nterrors.AdbError(fmt.Sprintf("Failed to create forward %s -> %s: %s",
				localSpec, remoteSpec, strings.TrimSpace(stderr)))
		}
		fmt.Printf("  Forward added: %s -> %s on %s.\n", localSpec, remoteSpec, model)
		return nil
	}

	fwdOut, _, fwdCode := adb.Run([]string{"adb", "-s", serial, "forward", "--list"})
	revOut, _, revCode := adb.Run([]string{"adb", "-s", serial, "reverse", "--list"})

	fmt.Printf("\n  Port Forwards \u2014 %s\n\n", model)

	fmt.Println("  Forwards (host -> device):")
	if fwdCode == 0 {
		lines := filterNonEmpty(strings.Split(fwdOut, "\n"))
		if len(lines) > 0 {
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					fmt.Printf("    %s  ->  %s\n", parts[1], parts[2])
				} else {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			fmt.Println("    (none)")
		}
	} else {
		fmt.Println("    (none)")
	}

	fmt.Println()
	fmt.Println("  Reverse forwards (device -> host):")
	if revCode == 0 {
		lines := filterNonEmpty(strings.Split(revOut, "\n"))
		if len(lines) > 0 {
			for _, line := range lines {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					fmt.Printf("    %s  ->  %s\n", parts[1], parts[2])
				} else {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			fmt.Println("    (none)")
		}
	} else {
		fmt.Println("    (none)")
	}

	fmt.Println()
	return nil
}

func filterNonEmpty(lines []string) []string {
	var out []string
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
