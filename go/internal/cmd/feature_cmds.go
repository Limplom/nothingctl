package cmd

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/appbackup"
	"github.com/Limplom/nothingctl/internal/appmanager"
	"github.com/Limplom/nothingctl/internal/audio"
	"github.com/Limplom/nothingctl/internal/battery"
	"github.com/Limplom/nothingctl/internal/capture"
	"github.com/Limplom/nothingctl/internal/debloat"
	"github.com/Limplom/nothingctl/internal/devoptions"
	"github.com/Limplom/nothingctl/internal/diagnostics"
	"github.com/Limplom/nothingctl/internal/display"
	"github.com/Limplom/nothingctl/internal/glyph"
	"github.com/Limplom/nothingctl/internal/info"
	"github.com/Limplom/nothingctl/internal/inputctl"
	"github.com/Limplom/nothingctl/internal/maintenance"
	"github.com/Limplom/nothingctl/internal/modules"
	"github.com/Limplom/nothingctl/internal/network"
	"github.com/Limplom/nothingctl/internal/notifclip"
	"github.com/Limplom/nothingctl/internal/nothingsettings"
	"github.com/Limplom/nothingctl/internal/performance"
	"github.com/Limplom/nothingctl/internal/permissions"
	"github.com/Limplom/nothingctl/internal/procmon"
	"github.com/Limplom/nothingctl/internal/prop"
	"github.com/Limplom/nothingctl/internal/reboot"
	"github.com/Limplom/nothingctl/internal/sideload"
	"github.com/Limplom/nothingctl/internal/storage"
	"github.com/Limplom/nothingctl/internal/sysmon"
	"github.com/Limplom/nothingctl/internal/thermal"
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
	flagProfile string
	flagKey     string
	flagValue   string
	flagStream  string
	flagVolume  int
	flagDuration int

	// network flags
	flagProvider  string
	flagLocalPort string
	flagRemPort   string
	flagClear     bool
	flagSSID      string
	flagHost      string
	flagCode      string
	flagPort      int

	// input flags
	flagTap      string
	flagSwipe    string
	flagText     string
	flagKeyevent string

	// nothing flags
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
	rootCmd.AddCommand(debloatCmd)

	modulesCmd.Flags().StringVar(&flagInstall, "install", "", "comma-separated module IDs to install, or 'all'")
	rootCmd.AddCommand(modulesCmd)

	rootCmd.AddCommand(modulesStatusCmd)

	modulesToggleCmd.Flags().StringVar(&flagModuleIDs, "modules", "", "comma-separated module IDs")
	modulesToggleCmd.Flags().BoolVar(&flagEnable, "enable", true, "true to enable, false to disable")
	rootCmd.AddCommand(modulesToggleCmd)

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
}

// ---------------------------------------------------------------------------
// App Management Commands
// ---------------------------------------------------------------------------

var packageListCmd = &cobra.Command{
	Use:   "package-list",
	Short: "List installed packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionPackageList(serial, flagFormat)
	},
}

var appInfoCmd = &cobra.Command{
	Use:   "app-info",
	Short: "Show detailed app information",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionAppInfo(serial, flagPackage)
	},
}

var killAppCmd = &cobra.Command{
	Use:   "kill-app",
	Short: "Force-stop an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionKillApp(serial, flagPackage)
	},
}

var launchAppCmd = &cobra.Command{
	Use:   "launch-app",
	Short: "Launch an app or deep link",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionLaunchApp(serial, flagPackage, flagDeepLink)
	},
}

var appBackupCmd = &cobra.Command{
	Use:   "app-backup",
	Short: "Backup APK and app data",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		pkgs := splitCSV(flagPackages)
		return appbackup.ActionAppBackup(serial, flagBaseDir, pkgs)
	},
}

var appRestoreCmd = &cobra.Command{
	Use:   "app-restore",
	Short: "Restore app backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		pkgs := splitCSV(flagPackages)
		return appbackup.ActionAppRestore(serial, flagBaseDir, pkgs)
	},
}

var sideloadCmd = &cobra.Command{
	Use:   "sideload",
	Short: "Install APK or split-APK",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sideload.ActionSideload(serial, flagAPK, flagDowngrade)
	},
}

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Audit dangerous app permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return permissions.ActionPermissions(serial, flagPackage)
	},
}

var debloatCmd = &cobra.Command{
	Use:   "debloat",
	Short: "Manage NothingOS bloatware",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		if flagInstall != "" {
			return debloat.ActionRestoreDebloat(serial, splitCSV(flagInstall))
		}
		return debloat.ActionDebloat(serial, splitCSV(flagRemove))
	},
}

var modulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List and install recommended Magisk modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModules(serial, flagBaseDir, splitCSV(flagInstall))
	},
}

var modulesStatusCmd = &cobra.Command{
	Use:   "modules-status",
	Short: "Show installed Magisk modules on device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModulesStatus(serial)
	},
}

var modulesToggleCmd = &cobra.Command{
	Use:   "modules-toggle",
	Short: "Enable or disable Magisk modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModulesToggle(serial, splitCSV(flagModuleIDs), flagEnable)
	},
}

// ---------------------------------------------------------------------------
// System Info & Monitoring Commands
// ---------------------------------------------------------------------------

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show full device dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return info.ActionInfo(serial)
	},
}

var batteryCmd = &cobra.Command{
	Use:   "battery",
	Short: "Show battery health report",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionBattery(serial)
	},
}

var batteryStatsCmd = &cobra.Command{
	Use:   "battery-stats",
	Short: "Show per-app wakelock drain since last charge",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionBatteryStats(serial)
	},
}

var chargingControlCmd = &cobra.Command{
	Use:   "charging-control",
	Short: "Set charging limit",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionChargingControl(serial, flagLimit)
	},
}

var thermalCmd = &cobra.Command{
	Use:   "thermal",
	Short: "Show thermal zone temperatures",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return thermal.ActionThermal(serial, flagWatch)
	},
}

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Show RAM usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sysmon.ActionMemory(serial, flagPackage, flagWatch)
	},
}

var cpuUsageCmd = &cobra.Command{
	Use:   "cpu-usage",
	Short: "Show CPU frequencies and top processes",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sysmon.ActionCPUUsage(serial, flagTopN, flagWatch)
	},
}

var processTreeCmd = &cobra.Command{
	Use:   "process-tree",
	Short: "Show running process tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionProcessTree(serial, flagPackage)
	},
}

var dozeStatusCmd = &cobra.Command{
	Use:   "doze-status",
	Short: "Show Doze mode status and whitelist",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionDozeStatus(serial, flagWhitelistAdd, flagWhitelistRemove)
	},
}

var locationCmd = &cobra.Command{
	Use:   "location",
	Short: "Show or set location mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionLocation(serial, flagLocationMode)
	},
}

var logcatCmd = &cobra.Command{
	Use:   "logcat",
	Short: "Capture logcat to file",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionLogcat(serial, flagBaseDir, flagPackage, flagTag, flagLevel, flagLines)
	},
}

var bugreportCmd = &cobra.Command{
	Use:   "bugreport",
	Short: "Capture full bugreport",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionBugreport(serial, flagBaseDir)
	},
}

var anrDumpCmd = &cobra.Command{
	Use:   "anr-dump",
	Short: "Collect ANR traces and tombstones",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionANRDump(serial, flagBaseDir)
	},
}

// ---------------------------------------------------------------------------
// Display & Audio Commands
// ---------------------------------------------------------------------------

var displayCmd = &cobra.Command{
	Use:   "display",
	Short: "Show or change display settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return display.ActionDisplay(serial, "", flagKey, flagValue)
	},
}

var colorProfileCmd = &cobra.Command{
	Use:   "color-profile",
	Short: "Set display color profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return display.ActionColorProfile(serial, "", flagProfile)
	},
}

var audioCmd = &cobra.Command{
	Use:   "audio",
	Short: "Show or adjust audio volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return audio.ActionAudio(serial, "", flagStream, flagVolume)
	},
}

var audioRouteCmd = &cobra.Command{
	Use:   "audio-route",
	Short: "Show active audio routing",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return audio.ActionAudioRoute(serial, "")
	},
}

var screenshotCmd = &cobra.Command{
	Use:   "screenshot",
	Short: "Take a screenshot and pull to host",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return capture.ActionScreenshot(serial, flagBaseDir)
	},
}

var screenrecordCmd = &cobra.Command{
	Use:   "screenrecord",
	Short: "Record screen to video file",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return capture.ActionScreenrecord(serial, flagBaseDir, flagDuration)
	},
}

// ---------------------------------------------------------------------------
// Network & Connectivity Commands
// ---------------------------------------------------------------------------

var networkInfoCmd = &cobra.Command{
	Use:   "network-info",
	Short: "Show WiFi, IP, and DNS info",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionNetworkInfo(serial, "")
	},
}

var dnsSetCmd = &cobra.Command{
	Use:   "dns-set",
	Short: "Set Private DNS provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionDNSSet(serial, "", flagProvider)
	},
}

var portForwardCmd = &cobra.Command{
	Use:   "port-forward",
	Short: "Set up or clear ADB port forwarding",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionPortForward(serial, "", flagLocalPort, flagRemPort, flagClear)
	},
}

var wifiScanCmd = &cobra.Command{
	Use:   "wifi-scan",
	Short: "Scan for nearby WiFi networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiScan(serial, "")
	},
}

var wifiProfilesCmd = &cobra.Command{
	Use:   "wifi-profiles",
	Short: "List saved WiFi profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiProfiles(serial, "", "")
	},
}

var forgetWifiCmd = &cobra.Command{
	Use:   "forget-wifi",
	Short: "Forget a saved WiFi network",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiProfiles(serial, "", flagSSID)
	},
}

var wifiADBCmd = &cobra.Command{
	Use:   "wifi-adb",
	Short: "Switch ADB to wireless TCP/IP mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiADB(serial)
	},
}

var adbPairCmd = &cobra.Command{
	Use:   "adb-pair",
	Short: "Pair device for wireless ADB (Android 11+)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return network.ActionADBPair(flagPort)
	},
}

// ---------------------------------------------------------------------------
// Input & Control Commands
// ---------------------------------------------------------------------------

var inputCmd = &cobra.Command{
	Use:   "input",
	Short: "Send touch, swipe, text or key input",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return inputctl.ActionInput(serial, "", flagTap, flagSwipe, flagText, flagKeyevent)
	},
}

var devOptionsCmd = &cobra.Command{
	Use:   "dev-options",
	Short: "Show or change Developer Options",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return devoptions.ActionDevOptions(serial, "", flagKey, flagValue)
	},
}

var screenAlwaysOnCmd = &cobra.Command{
	Use:   "screen-always-on",
	Short: "Keep screen on while charging",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return devoptions.ActionScreenAlwaysOn(serial, "", &flagEnable)
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "cache-clear",
	Short: "Clear app or system cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return maintenance.ActionCacheClear(serial, "", flagPackage)
	},
}

var localeCmd = &cobra.Command{
	Use:   "locale",
	Short: "Set locale, timezone or time format",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		var h24 *bool
		if cmd.Flags().Changed("24h") {
			h24 = &flagHour24
		}
		return maintenance.ActionLocale(serial, "", flagLang, flagTimezone, h24)
	},
}

var notificationsCmd = &cobra.Command{
	Use:   "notifications",
	Short: "List active notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return notifclip.ActionNotifications(serial, "", flagPackage)
	},
}

var clipboardCmd = &cobra.Command{
	Use:   "clipboard",
	Short: "Read or write clipboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return notifclip.ActionClipboard(serial, "", flagText)
	},
}

// ---------------------------------------------------------------------------
// Nothing-Specific Commands
// ---------------------------------------------------------------------------

var glyphCmd = &cobra.Command{
	Use:   "glyph",
	Short: "Show Glyph interface status",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyph(serial, "", flagValue)
	},
}

var glyphPatternCmd = &cobra.Command{
	Use:   "glyph-pattern",
	Short: "Run a Glyph light pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyphPattern(serial, "", flagProfile)
	},
}

var glyphNotifyCmd = &cobra.Command{
	Use:   "glyph-notify",
	Short: "Show Glyph notification settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyphNotify(serial, "")
	},
}

var nothingSettingsCmd = &cobra.Command{
	Use:   "nothing-settings",
	Short: "Show or change Nothing-specific settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return nothingsettings.ActionNothingSettings(serial, "", flagKey, flagValue)
	},
}

var essentialSpaceCmd = &cobra.Command{
	Use:   "essential-space",
	Short: "Enable or disable Essential Space",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return nothingsettings.ActionEssentialSpace(serial, "", &flagEnable)
	},
}

// ---------------------------------------------------------------------------
// Utility Commands
// ---------------------------------------------------------------------------

var rebootCmd = &cobra.Command{
	Use:   "reboot",
	Short: "Reboot to selected target",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return reboot.ActionReboot(serial, flagTarget)
	},
}

var propGetCmd = &cobra.Command{
	Use:   "prop-get",
	Short: "Read system property or list all",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return prop.ActionPropGet(serial, "", flagPropKey)
	},
}

var propSetCmd = &cobra.Command{
	Use:   "prop-set",
	Short: "Write system property (requires root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return prop.ActionPropSet(serial, flagPropKey, flagPropValue)
	},
}

var performanceCmd = &cobra.Command{
	Use:   "performance",
	Short: "Show or set CPU governor profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return performance.ActionPerformance(serial, flagProfile)
	},
}

var storageReportCmd = &cobra.Command{
	Use:   "storage-report",
	Short: "Show storage usage report",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return storage.ActionStorageReport(serial, flagTopN)
	},
}

var apkExtractCmd = &cobra.Command{
	Use:   "apk-extract",
	Short: "Extract APK(s) from device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return storage.ActionAPKExtract(serial, flagBaseDir, flagIncludeSystem)
	},
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
