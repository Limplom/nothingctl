// Package procmon provides process monitoring, Doze status, and location management.
package procmon

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var stateLabels = map[string]string{
	"S": "Sleeping",
	"R": "Running",
	"Z": "Zombie",
	"T": "Stopped",
	"D": "Disk sleep",
}

type process struct {
	pid   int
	ppid  int
	user  string
	name  string
	state string
}

func parsePS(output string) []process {
	var procs []process
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		var pid, ppid int
		if _, err := fmt.Sscanf(parts[0], "%d", &pid); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(parts[1], "%d", &ppid); err != nil {
			continue
		}
		user := parts[2]
		state := parts[len(parts)-1]
		name := strings.Join(parts[3:len(parts)-1], " ")
		procs = append(procs, process{pid, ppid, user, name, state})
	}
	return procs
}

var u0aRe = regexp.MustCompile(`^u0_a(\d+)$`)
var u0iRe = regexp.MustCompile(`^u0_i(\d+)$`)
var sysUserRe = regexp.MustCompile(`^(shell|radio|log|nobody|nfc|bluetooth|wifi|camera|media|audioserver|cameraserver|credstore|keystore|statsd|storaged|inet|net_bt|net_bt_admin|net_raw|net_admin)$`)

func isUserApp(user string) bool { return u0aRe.MatchString(user) }
func isIsolated(user string) bool { return u0iRe.MatchString(user) }
func isSystem(user string) bool {
	return user == "root" || user == "system" || sysUserRe.MatchString(user)
}

// ActionProcessTree shows process list, optionally filtered by package name.
func ActionProcessTree(serial, packageName string) error {
	raw := adb.ShellStr(serial, "ps -A -o PID,PPID,USER,NAME,S")
	if raw == "" {
		return nterrors.AdbError("Failed to retrieve process list from device.")
	}
	model := adb.ShellStr(serial, "getprop ro.product.model")
	procs := parsePS(raw)

	fmt.Printf("\n  Process Tree \u2014 %s\n", model)

	if packageName != "" {
		var filtered []process
		for _, p := range procs {
			if strings.Contains(p.name, packageName) {
				filtered = append(filtered, p)
			}
		}
		fmt.Printf("  Filter: %s\n\n", packageName)
		if len(filtered) == 0 {
			fmt.Printf("  No processes found matching '%s'.\n\n", packageName)
			return nil
		}
		fmt.Printf("  %-6s %-6s %-10s %-24s %s\n", "PID", "PPID", "UID", "Name", "State")
		fmt.Printf("  %-6s %-6s %-10s %-24s %s\n",
			strings.Repeat("\u2500", 6), strings.Repeat("\u2500", 6),
			strings.Repeat("\u2500", 10), strings.Repeat("\u2500", 24), strings.Repeat("\u2500", 16))
		// Sort by PID
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].pid < filtered[j].pid })
		for _, p := range filtered {
			stateLabel, ok := stateLabels[p.state]
			if !ok {
				stateLabel = p.state
			}
			fmt.Printf("  %-6d %-6d %-10s %-24s %s (%s)\n",
				p.pid, p.ppid, p.user, p.name, p.state, stateLabel)
		}
		fmt.Println()
		return nil
	}

	// Summary view
	var userApps, isolated, systemProcs []process
	for _, p := range procs {
		if isUserApp(p.user) {
			userApps = append(userApps, p)
		} else if isIsolated(p.user) {
			isolated = append(isolated, p)
		} else if isSystem(p.user) {
			systemProcs = append(systemProcs, p)
		}
	}
	otherCount := len(procs) - len(userApps) - len(isolated) - len(systemProcs)
	if otherCount < 0 {
		otherCount = 0
	}

	fmt.Println()
	fmt.Printf("  Total: %d processes\n\n", len(procs))
	fmt.Printf("  System processes: %d\n", len(systemProcs))
	fmt.Printf("  User apps:        %d\n", len(userApps))
	fmt.Printf("  Isolated:         %d\n", len(isolated))
	fmt.Printf("  Other:            %d\n\n", otherCount)
	fmt.Println("  User Apps (running):")
	fmt.Printf("  %-6s %-32s %s\n", "PID", "Name", "UID")
	fmt.Printf("  %-6s %-32s %s\n", strings.Repeat("\u2500", 6), strings.Repeat("\u2500", 32), strings.Repeat("\u2500", 12))

	// Sort user apps by PID
	sort.Slice(userApps, func(i, j int) bool { return userApps[i].pid < userApps[j].pid })
	top := userApps
	if len(top) > 30 {
		top = top[:30]
	}
	for _, p := range top {
		fmt.Printf("  %-6d %-32s %s\n", p.pid, p.name, p.user)
	}
	if len(userApps) > 30 {
		fmt.Printf("  ... (%d more)\n", len(userApps)-30)
	}
	fmt.Println()
	return nil
}

// ── Doze ──────────────────────────────────────────────────────────────────────

var mStateRe = regexp.MustCompile(`mState\s*=\s*(\S+)`)
var mLightStateRe = regexp.MustCompile(`mLightState\s*=\s*(\S+)`)
var screenOnRe = regexp.MustCompile(`(?i)mScreenOn\s*=\s*(true|false)`)
var interactiveRe = regexp.MustCompile(`(?i)Interactive:\s*(true|false)`)
var acRe = regexp.MustCompile(`(?i)AC powered:\s*(true|false)`)
var usbRe = regexp.MustCompile(`(?i)USB powered:\s*(true|false)`)
var wirelessRe = regexp.MustCompile(`(?i)Wireless powered:\s*(true|false)`)
var whitelistPkgRe = regexp.MustCompile(`(?i)(?:UID=\d+:\s*)?([a-z][a-z0-9_.]+)`)

var dozeStateLabels = map[string]string{
	"ACTIVE":           "active (not in Doze)",
	"IDLE_PENDING":     "idle pending",
	"SENSING":          "sensing",
	"LOCATING":         "locating",
	"IDLE":             "IDLE (full Doze)",
	"IDLE_MAINTENANCE": "idle maintenance",
	"OVERRIDE":         "override",
	"UNKNOWN":          "unknown",
}

func parseDozeState(s string) string {
	if m := mStateRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return "UNKNOWN"
}

func parseLightState(s string) string {
	if m := mLightStateRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return "UNKNOWN"
}

func parseScreenOn(s string) bool {
	if m := screenOnRe.FindStringSubmatch(s); m != nil {
		return strings.ToLower(m[1]) == "true"
	}
	if m := interactiveRe.FindStringSubmatch(s); m != nil {
		return strings.ToLower(m[1]) == "true"
	}
	return false
}

func parsePluggedIn(s string) bool {
	for _, re := range []*regexp.Regexp{acRe, usbRe, wirelessRe} {
		if m := re.FindStringSubmatch(s); m != nil && strings.ToLower(m[1]) == "true" {
			return true
		}
	}
	return false
}

func parseWhitelist(output string) []string {
	seen := make(map[string]bool)
	var pkgs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if strings.Contains(line, ",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pkg := strings.TrimSpace(parts[1])
				if strings.Contains(pkg, ".") && !seen[pkg] {
					seen[pkg] = true
					pkgs = append(pkgs, pkg)
				}
			}
		} else {
			if m := whitelistPkgRe.FindStringSubmatch(line); m != nil {
				pkg := m[1]
				if strings.Contains(pkg, ".") && !seen[pkg] {
					seen[pkg] = true
					pkgs = append(pkgs, pkg)
				}
			}
		}
	}
	return pkgs
}

// ActionDozeStatus shows Doze mode status and manages the battery-optimization whitelist.
func ActionDozeStatus(serial, whitelistAdd, whitelistRemove string) error {
	model := adb.ShellStr(serial, "getprop ro.product.model")

	if whitelistAdd != "" {
		result := adb.ShellStr(serial, "dumpsys deviceidle whitelist +"+whitelistAdd)
		fmt.Printf("\n  Whitelist add: %s\n", whitelistAdd)
		if result != "" {
			fmt.Printf("  Response: %s\n", result)
		} else {
			fmt.Println("  Done (no output from device).")
		}
		fmt.Println()
	}
	if whitelistRemove != "" {
		result := adb.ShellStr(serial, "dumpsys deviceidle whitelist -"+whitelistRemove)
		fmt.Printf("\n  Whitelist remove: %s\n", whitelistRemove)
		if result != "" {
			fmt.Printf("  Response: %s\n", result)
		} else {
			fmt.Println("  Done (no output from device).")
		}
		fmt.Println()
	}

	var deviceidleDump, whitelistRaw, batteryDump string
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); deviceidleDump = adb.ShellStr(serial, "dumpsys deviceidle") }()
	go func() { defer wg.Done(); whitelistRaw = adb.ShellStr(serial, "dumpsys deviceidle whitelist") }()
	go func() { defer wg.Done(); batteryDump = adb.ShellStr(serial, "dumpsys battery") }()
	wg.Wait()

	state := parseDozeState(deviceidleDump)
	lightState := parseLightState(deviceidleDump)
	screenOn := parseScreenOn(deviceidleDump)
	pluggedIn := parsePluggedIn(batteryDump)
	whitelist := parseWhitelist(whitelistRaw)

	stateLabel, ok := dozeStateLabels[state]
	if !ok {
		stateLabel = strings.ToLower(state)
	}

	fmt.Printf("\n  Doze Status \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %s (%s)\n", "State", state, stateLabel)
	fmt.Printf("  %-16s: %s\n", "Light State", lightState)
	screenStr := "no"
	if screenOn {
		screenStr = "yes"
	}
	fmt.Printf("  %-16s: %s\n", "Screen On", screenStr)
	if pluggedIn {
		fmt.Printf("  %-16s: yes (Doze requires battery)\n", "Plugged In")
	} else {
		fmt.Printf("  %-16s: no\n", "Plugged In")
	}

	fmt.Println()
	fmt.Println("  Whitelist (battery optimization exempt):")
	if len(whitelist) > 0 {
		// Sort
		sort.Strings(whitelist)
		for _, pkg := range whitelist {
			fmt.Printf("  %s\n", pkg)
		}
		fmt.Printf("  ... (%d total)\n", len(whitelist))
	} else {
		fmt.Println("  (empty)")
	}
	fmt.Println()
	return nil
}

// ── Location ──────────────────────────────────────────────────────────────────

var locationModes = map[string]string{
	"0": "Off",
	"1": "Sensors only (GPS)",
	"2": "Battery saving (Network only)",
	"3": "High Accuracy (GPS + Network)",
	"4": "Device only (GPS)",
}

var modeMap = map[string]string{
	"off": "0", "gps": "1", "device": "1", "sensors": "1",
	"battery": "2", "on": "3", "high": "3", "accuracy": "3",
}

var latLonRe = regexp.MustCompile(`([-\d.]+),([-\d.]+)`)
var accRe = regexp.MustCompile(`(?i)acc(?:uracy)?[=\s]+([\d.]+)`)
var etRe = regexp.MustCompile(`(?i)et=(\S+)`)
var providerLineRe = regexp.MustCompile(`(?i)^\s*(gps|network|passive|fused)\s*:\s*(.*)`)
var providerStatusRe = regexp.MustCompile(`(?i)(gps|network|passive)\s+provider[^:]*?(?:\[|:)?\s*(enabled|disabled)`)
var fineLocRe = regexp.MustCompile(`(?i)(?:Package\s+)?([a-z][a-z0-9_.]+)\s+uid=`)
var fineLocSimpleRe = regexp.MustCompile(`(?i)^[a-z][a-z0-9_.]+\.[a-z0-9_.]+$`)

type locationEntry struct {
	provider string
	lat, lon *float64
	accuracy string
	age      string
}

func parseLastKnownLocations(dump string) []locationEntry {
	var results []locationEntry
	inSection := false
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.Contains(line, "Last Known Locations") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		// Check for new major section header
		if stripped != "" && !strings.HasPrefix(strings.ToLower(stripped), "passive") &&
			!strings.HasPrefix(strings.ToLower(stripped), "gps") &&
			!strings.HasPrefix(strings.ToLower(stripped), "network") &&
			!strings.HasPrefix(strings.ToLower(stripped), "fused") &&
			len(stripped) > 0 && stripped[0] >= 'A' && stripped[0] <= 'Z' {
			break
		}
		if m := providerLineRe.FindStringSubmatch(line); m != nil {
			prov := strings.ToLower(m[1])
			rest := m[2]
			if strings.Contains(strings.ToLower(rest), "null") {
				results = append(results, locationEntry{provider: prov})
				continue
			}
			entry := locationEntry{provider: prov}
			if lm := latLonRe.FindStringSubmatch(rest); lm != nil {
				var la, lo float64
				fmt.Sscanf(lm[1], "%f", &la)
				fmt.Sscanf(lm[2], "%f", &lo)
				entry.lat = &la
				entry.lon = &lo
			}
			if am := accRe.FindStringSubmatch(rest); am != nil {
				entry.accuracy = am[1] + "m"
			}
			if em := etRe.FindStringSubmatch(rest); em != nil {
				entry.age = em[1]
			}
			results = append(results, entry)
		}
	}
	return results
}

func parseProviders(dump string) map[string]bool {
	providers := make(map[string]bool)
	for _, line := range strings.Split(dump, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := providerStatusRe.FindStringSubmatch(line); m != nil {
			providers[strings.ToLower(m[1])] = strings.ToLower(m[2]) == "enabled"
		}
	}
	return providers
}

func parseFineLocationApps(output string) []string {
	var pkgs []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		stripped := strings.TrimSpace(line)
		if m := fineLocRe.FindStringSubmatch(stripped); m != nil && strings.Contains(m[1], ".") {
			pkgs = append(pkgs, m[1])
		} else if fineLocSimpleRe.MatchString(stripped) {
			pkgs = append(pkgs, stripped)
		}
	}
	return pkgs
}

func formatCoord(lat, lon float64) string {
	ns := "N"
	if lat < 0 {
		ns = "S"
	}
	ew := "E"
	if lon < 0 {
		ew = "W"
	}
	if lat < 0 {
		lat = -lat
	}
	if lon < 0 {
		lon = -lon
	}
	return fmt.Sprintf("%.4f\u00b0 %s, %.4f\u00b0 %s", lat, ns, lon, ew)
}

// ActionLocation shows GPS/location status and optionally sets the location mode.
func ActionLocation(serial, mode string) error {
	model := adb.ShellStr(serial, "getprop ro.product.model")

	if mode != "" {
		numeric, ok := modeMap[strings.ToLower(mode)]
		if !ok {
			fmt.Printf("\n  Unknown mode '%s'. Valid options: off, gps, device, sensors, battery, on, high, accuracy\n\n", mode)
			return nil
		}
		_, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "settings", "put", "secure", "location_mode", numeric})
		fmt.Println()
		if code == 0 {
			label, _ := locationModes[numeric]
			fmt.Printf("  Location mode set to: %s\n", label)
		} else {
			fmt.Println("  Failed to set location mode.")
		}
		fmt.Println()
		return nil
	}

	rawMode := adb.Setting(serial, "secure", "location_mode")
	locationDump := adb.ShellStr(serial, "dumpsys location")
	appopsOut := adb.ShellStr(serial, "cmd appops query-op android:fine_location allow")

	modeLabel, ok := locationModes[rawMode]
	if !ok {
		modeLabel = fmt.Sprintf("Unknown (mode=%s)", rawMode)
	}
	providers := parseProviders(locationDump)
	locations := parseLastKnownLocations(locationDump)
	fineApps := parseFineLocationApps(appopsOut)

	fmt.Printf("\n  Location \u2014 %s\n\n", model)
	fmt.Printf("  %-16s: %s\n", "Mode", modeLabel)

	gpsStatus := "disabled"
	if providers["gps"] {
		gpsStatus = "enabled"
	}
	networkStatus := "disabled"
	if providers["network"] {
		networkStatus = "enabled"
	}
	fmt.Printf("  %-16s: %s\n", "GPS Provider", gpsStatus)
	fmt.Printf("  %-16s: %s\n", "Network Provider", networkStatus)

	fmt.Println()
	fmt.Println("  Last Known Location:")
	if len(locations) > 0 {
		for _, loc := range locations {
			pname := strings.ToUpper(loc.provider[:1]) + loc.provider[1:]
			if loc.lat == nil {
				fmt.Printf("    %-9s: (none)\n", pname)
			} else {
				coordStr := formatCoord(*loc.lat, *loc.lon)
				parts := []string{coordStr}
				if loc.accuracy != "" {
					parts = append(parts, "accuracy: "+loc.accuracy)
				}
				if loc.age != "" {
					parts = append(parts, "age: "+loc.age)
				}
				fmt.Printf("    %-9s: %s\n", pname, strings.Join(parts, ",  "))
			}
		}
	} else {
		fmt.Println("    (not available)")
	}

	fmt.Println()
	appSample := fineApps
	if len(appSample) > 8 {
		appSample = appSample[:8]
	}
	fmt.Printf("  Apps with Location Permission (fine): %d\n", len(fineApps))
	if len(appSample) > 0 {
		suffix := ""
		if len(fineApps) > 8 {
			suffix = " ..."
		}
		fmt.Printf("    %s%s\n", strings.Join(appSample, ", "), suffix)
	}
	fmt.Println()
	return nil
}
