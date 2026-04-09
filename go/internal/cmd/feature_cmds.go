package cmd

import (
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Shared flag variables
// ---------------------------------------------------------------------------

var (
	// app flags
	flagPackage   string
	flagPackages  string
	flagFormat    string
	flagDeepLink  string
	flagDowngrade bool
	flagAPK       string
	flagRemove    string
	flagInstall   string
	flagEnable    bool

	// system info flags
	flagWatch  bool
	flagTopN   int
	flagLines  int
	flagTag    string
	flagLevel  string

	// display/audio flags
	flagProfile  string
	flagKey      string
	flagValue    string
	flagStream   string
	flagVolume   int
	flagDuration int

	// network flags
	flagProvider  string
	flagLocalPort string
	flagRemPort   string
	flagClear     bool
	flagSSID      string
	flagPort      int

	// input flags
	flagTap      string
	flagSwipe    string
	flagText     string
	flagKeyevent string

	// locale flags
	flagLang     string
	flagTimezone string
	flagHour24   bool
	flagLimit    int

	// reboot flags
	flagTarget string

	// prop flags
	flagPropKey   string
	flagPropValue string

	// storage flags
	flagIncludeSystem bool

	// modules flags
	flagModuleIDs string
	flagForce     bool

	// verify-backup flags
	flagLive bool

	// doze flags
	flagWhitelistAdd    string
	flagWhitelistRemove string

	// location flags
	flagLocationMode string
)

// ---------------------------------------------------------------------------
// init — register all feature subcommands
// ---------------------------------------------------------------------------

func init() {
	// ── App Management ──────────────────────────────────────────────────────

	packageListCmd.Flags().StringVar(&flagFormat, "format", "text", "output format: text, csv, json")
	rootCmd.AddCommand(packageListCmd)

	appInfoCmd.Flags().StringVar(&flagPackage, "package", "", "package name (required)")
	_ = appInfoCmd.MarkFlagRequired("package")
	rootCmd.AddCommand(appInfoCmd)

	killAppCmd.Flags().StringVar(&flagPackage, "package", "", "package name (required)")
	_ = killAppCmd.MarkFlagRequired("package")
	rootCmd.AddCommand(killAppCmd)

	launchAppCmd.Flags().StringVar(&flagPackage, "package", "", "package name (required)")
	launchAppCmd.Flags().StringVar(&flagDeepLink, "deep-link", "", "deep link URI to open")
	_ = launchAppCmd.MarkFlagRequired("package")
	rootCmd.AddCommand(launchAppCmd)

	appBackupCmd.Flags().StringVar(&flagPackages, "packages", "", "comma-separated package names (empty = all user apps)")
	rootCmd.AddCommand(appBackupCmd)

	appRestoreCmd.Flags().StringVar(&flagPackages, "packages", "", "comma-separated package names to restore")
	rootCmd.AddCommand(appRestoreCmd)

	sideloadCmd.Flags().StringVar(&flagAPK, "apk", "", "path to APK or split-APK directory (required)")
	sideloadCmd.Flags().BoolVar(&flagDowngrade, "downgrade", false, "allow version downgrade")
	_ = sideloadCmd.MarkFlagRequired("apk")
	rootCmd.AddCommand(sideloadCmd)

	permissionsCmd.Flags().StringVar(&flagPackage, "package", "", "filter to specific package (empty = all apps)")
	rootCmd.AddCommand(permissionsCmd)

	debloatCmd.Flags().StringVar(&flagRemove, "remove", "", "comma-separated bloatware IDs to disable")
	debloatCmd.Flags().StringVar(&flagInstall, "restore", "", "comma-separated bloatware IDs to restore")
	debloatCmd.Flags().StringVar(&flagProfile, "profile", "", "debloat profile: minimal | recommended | aggressive")
	rootCmd.AddCommand(debloatCmd)

	modulesCmd.Flags().StringVar(&flagInstall, "install", "", "comma-separated module IDs to install, or 'all'")
	rootCmd.AddCommand(modulesCmd)

	rootCmd.AddCommand(modulesStatusCmd)

	modulesToggleCmd.Flags().StringVar(&flagModuleIDs, "modules", "", "comma-separated module IDs")
	modulesToggleCmd.Flags().BoolVar(&flagEnable, "enable", true, "true to enable, false to disable")
	rootCmd.AddCommand(modulesToggleCmd)

	modulesUpdateAllCmd.Flags().BoolVar(&flagForce, "force", false, "skip confirmation prompt")
	rootCmd.AddCommand(modulesUpdateAllCmd)

	// ── System Info & Monitoring ─────────────────────────────────────────────

	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(batteryCmd)
	rootCmd.AddCommand(batteryStatsCmd)

	chargingControlCmd.Flags().IntVar(&flagLimit, "limit", 0, "charging limit percentage (0 = disable limit)")
	rootCmd.AddCommand(chargingControlCmd)

	thermalCmd.Flags().BoolVar(&flagWatch, "watch", false, "live monitoring mode")
	rootCmd.AddCommand(thermalCmd)

	memoryCmd.Flags().StringVar(&flagPackage, "package", "", "filter to specific package")
	memoryCmd.Flags().BoolVar(&flagWatch, "watch", false, "live monitoring mode")
	rootCmd.AddCommand(memoryCmd)

	cpuUsageCmd.Flags().IntVar(&flagTopN, "top", 10, "number of top processes to show")
	cpuUsageCmd.Flags().BoolVar(&flagWatch, "watch", false, "live monitoring mode")
	rootCmd.AddCommand(cpuUsageCmd)

	processTreeCmd.Flags().StringVar(&flagPackage, "package", "", "filter to specific package")
	rootCmd.AddCommand(processTreeCmd)

	dozeStatusCmd.Flags().StringVar(&flagWhitelistAdd, "whitelist-add", "", "package to add to Doze whitelist")
	dozeStatusCmd.Flags().StringVar(&flagWhitelistRemove, "whitelist-remove", "", "package to remove from Doze whitelist")
	rootCmd.AddCommand(dozeStatusCmd)

	locationCmd.Flags().StringVar(&flagLocationMode, "mode", "", "set location mode: off, device, battery, high")
	rootCmd.AddCommand(locationCmd)

	logcatCmd.Flags().StringVar(&flagPackage, "package", "", "filter by package")
	logcatCmd.Flags().StringVar(&flagTag, "tag", "", "filter by tag")
	logcatCmd.Flags().StringVar(&flagLevel, "level", "V", "log level: V, D, I, W, E")
	logcatCmd.Flags().IntVar(&flagLines, "lines", 500, "number of lines to capture")
	rootCmd.AddCommand(logcatCmd)

	rootCmd.AddCommand(bugreportCmd)
	rootCmd.AddCommand(anrDumpCmd)

	// ── Display & Audio ──────────────────────────────────────────────────────

	displayCmd.Flags().StringVar(&flagKey, "set", "", "setting to change (brightness, dpi, timeout, rotation, font-scale)")
	displayCmd.Flags().StringVar(&flagValue, "value", "", "new value for --set")
	rootCmd.AddCommand(displayCmd)

	colorProfileCmd.Flags().StringVar(&flagProfile, "profile", "", "color profile: natural, vivid, srgb (required)")
	_ = colorProfileCmd.MarkFlagRequired("profile")
	rootCmd.AddCommand(colorProfileCmd)

	audioCmd.Flags().StringVar(&flagStream, "set", "", "stream to adjust (voice, system, ring, media, alarm, notification)")
	audioCmd.Flags().IntVar(&flagVolume, "volume", -1, "volume level (0–15) for --set")
	rootCmd.AddCommand(audioCmd)

	rootCmd.AddCommand(audioRouteCmd)

	screenshotCmd.Flags().StringVar(&flagBaseDir, "output-dir", "", "directory to save screenshot (default: ~/tools/Nothing)")
	rootCmd.AddCommand(screenshotCmd)

	screenrecordCmd.Flags().IntVar(&flagDuration, "duration", 30, "recording duration in seconds (max 180)")
	rootCmd.AddCommand(screenrecordCmd)

	// ── Network & Connectivity ───────────────────────────────────────────────

	rootCmd.AddCommand(networkInfoCmd)

	dnsSetCmd.Flags().StringVar(&flagProvider, "provider", "", "DNS provider: cloudflare, adguard, google, quad9, or custom hostname (required)")
	_ = dnsSetCmd.MarkFlagRequired("provider")
	rootCmd.AddCommand(dnsSetCmd)

	portForwardCmd.Flags().StringVar(&flagLocalPort, "local", "", "local port (required)")
	portForwardCmd.Flags().StringVar(&flagRemPort, "remote", "", "remote port (required)")
	portForwardCmd.Flags().BoolVar(&flagClear, "clear", false, "remove all port forwards")
	rootCmd.AddCommand(portForwardCmd)

	rootCmd.AddCommand(wifiScanCmd)
	rootCmd.AddCommand(wifiProfilesCmd)

	forgetWifiCmd.Flags().StringVar(&flagSSID, "ssid", "", "network SSID to forget (required)")
	_ = forgetWifiCmd.MarkFlagRequired("ssid")
	rootCmd.AddCommand(forgetWifiCmd)

	rootCmd.AddCommand(wifiADBCmd)

	adbPairCmd.Flags().IntVar(&flagPort, "port", 0, "pairing port shown on device (required)")
	_ = adbPairCmd.MarkFlagRequired("port")
	rootCmd.AddCommand(adbPairCmd)

	// ── Input & Control ──────────────────────────────────────────────────────

	inputCmd.Flags().StringVar(&flagTap, "tap", "", "tap coordinates: X,Y")
	inputCmd.Flags().StringVar(&flagSwipe, "swipe", "", "swipe coordinates: X1,Y1,X2,Y2[,duration_ms]")
	inputCmd.Flags().StringVar(&flagText, "text", "", "text to type")
	inputCmd.Flags().StringVar(&flagKeyevent, "keyevent", "", "keyevent name or code")
	rootCmd.AddCommand(inputCmd)

	devOptionsCmd.Flags().StringVar(&flagKey, "set", "", "developer option key")
	devOptionsCmd.Flags().StringVar(&flagValue, "value", "", "value for --set")
	rootCmd.AddCommand(devOptionsCmd)

	screenAlwaysOnCmd.Flags().BoolVar(&flagEnable, "enable", true, "true to enable, false to disable")
	rootCmd.AddCommand(screenAlwaysOnCmd)

	cacheClearCmd.Flags().StringVar(&flagPackage, "package", "", "clear cache for specific package only (empty = system cache)")
	rootCmd.AddCommand(cacheClearCmd)

	localeCmd.Flags().StringVar(&flagLang, "lang", "", "BCP-47 locale tag (e.g. de-DE)")
	localeCmd.Flags().StringVar(&flagTimezone, "timezone", "", "timezone ID (e.g. Europe/Berlin)")
	localeCmd.Flags().BoolVar(&flagHour24, "24h", false, "enable 24-hour time format")
	rootCmd.AddCommand(localeCmd)

	notificationsCmd.Flags().StringVar(&flagPackage, "package", "", "filter to specific package")
	rootCmd.AddCommand(notificationsCmd)

	clipboardCmd.Flags().StringVar(&flagText, "text", "", "text to write to clipboard (empty = read clipboard)")
	rootCmd.AddCommand(clipboardCmd)

	// ── Nothing-Specific ─────────────────────────────────────────────────────

	glyphCmd.Flags().StringVar(&flagValue, "enable", "", "enable or disable glyph: true/false")
	rootCmd.AddCommand(glyphCmd)

	glyphPatternCmd.Flags().StringVar(&flagProfile, "pattern", "", "pattern: pulse, blink, wave (required)")
	_ = glyphPatternCmd.MarkFlagRequired("pattern")
	rootCmd.AddCommand(glyphPatternCmd)

	rootCmd.AddCommand(glyphNotifyCmd)

	nothingSettingsCmd.Flags().StringVar(&flagKey, "set", "", "setting key to change")
	nothingSettingsCmd.Flags().StringVar(&flagValue, "value", "", "new value for --set")
	rootCmd.AddCommand(nothingSettingsCmd)

	essentialSpaceCmd.Flags().BoolVar(&flagEnable, "enable", true, "true to enable, false to disable")
	rootCmd.AddCommand(essentialSpaceCmd)

	// ── Utilities ────────────────────────────────────────────────────────────

	rebootCmd.Flags().StringVar(&flagTarget, "target", "", "reboot target: system, bootloader, recovery, safe, download, sideload")
	rootCmd.AddCommand(rebootCmd)

	propGetCmd.Flags().StringVar(&flagPropKey, "key", "", "property key to read (empty = all properties)")
	rootCmd.AddCommand(propGetCmd)

	propSetCmd.Flags().StringVar(&flagPropKey, "key", "", "property key (required)")
	propSetCmd.Flags().StringVar(&flagPropValue, "value", "", "property value (required)")
	_ = propSetCmd.MarkFlagRequired("key")
	_ = propSetCmd.MarkFlagRequired("value")
	rootCmd.AddCommand(propSetCmd)

	performanceCmd.Flags().StringVar(&flagProfile, "profile", "", "profile: performance, balanced, powersave")
	rootCmd.AddCommand(performanceCmd)

	storageReportCmd.Flags().IntVar(&flagTopN, "top", 20, "number of largest directories to show")
	rootCmd.AddCommand(storageReportCmd)

	apkExtractCmd.Flags().StringVar(&flagPackage, "package", "", "package name to extract (empty = all user apps)")
	apkExtractCmd.Flags().BoolVar(&flagIncludeSystem, "include-system", false, "include system apps")
	rootCmd.AddCommand(apkExtractCmd)

	selfUpdateCmd.Flags().BoolVar(&flagDryRun, "dry-run", false,
		"print what would be downloaded without replacing the binary")
	rootCmd.AddCommand(selfUpdateCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitCSV splits a comma-separated string into a slice, trimming spaces.
// Returns nil (not empty slice) when input is empty.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// boolPtr returns a pointer to b. Used for optional bool flags.
func boolPtr(b bool) *bool { return &b }

// intStr converts an integer flag value to string; returns "" if zero.
func intStr(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
