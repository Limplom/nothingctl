// Package appmanager provides app management actions for Nothing phones.
package appmanager

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func packageExists(serial, pkg string) bool {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages " + pkg})
	return strings.Contains(stdout, "package:"+pkg)
}

func dumpsysPackage(serial, pkg string) (string, error) {
	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "dumpsys package " + pkg})
	if code != 0 && strings.TrimSpace(stdout) == "" {
		return "", nterrors.AdbError(fmt.Sprintf("dumpsys package %s failed: %s", pkg, strings.TrimSpace(stderr)))
	}
	return stdout, nil
}

func fmtBytes(size int64) string {
	units := []string{"B", "KB", "MB", "GB"}
	f := float64(size)
	for _, u := range units {
		if f < 1024 {
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.1f TB", f)
}

func installerLabel(installer string) string {
	if installer == "" || installer == "null" {
		return "Unknown / sideloaded"
	}
	known := map[string]string{
		"com.android.vending":  "Google Play Store",
		"com.amazon.venezia":   "Amazon Appstore",
		"org.fdroid.fdroid":    "F-Droid",
		"com.huawei.appmarket": "Huawei AppGallery",
	}
	if label, ok := known[installer]; ok {
		return label
	}
	return installer
}

func extractPackagesSection(output, pkg string) string {
	lines := strings.Split(output, "\n")
	startPat := regexp.MustCompile(`\s{2}Package \[` + regexp.QuoteMeta(pkg) + `\]`)
	nextPat := regexp.MustCompile(`\s{2}Package \[`)

	start := -1
	for i, line := range lines {
		if startPat.MatchString(line) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}

	var result []string
	for _, line := range lines[start+1:] {
		if nextPat.MatchString(line) {
			break
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func reFirst(pattern, text string) string {
	rx := regexp.MustCompile(pattern)
	m := rx.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func parseAppInfo(serial, pkg string) (map[string]string, error) {
	raw, err := dumpsysPackage(serial, pkg)
	if err != nil {
		return nil, err
	}
	block := extractPackagesSection(raw, pkg)

	versionName := reFirst(`versionName=(\S+)`, block)
	vcRx := regexp.MustCompile(`versionCode=(\d+)`)
	versionCode := ""
	if m := vcRx.FindStringSubmatch(block); len(m) >= 2 {
		versionCode = m[1]
	}

	minSDK := reFirst(`minSdk=(\d+)`, block)
	targetSDK := reFirst(`targetSdk=(\d+)`, block)

	userSplitRx := regexp.MustCompile(`\n\s+User \d+:`)
	topBlock := userSplitRx.Split(block, 2)[0]
	lastUpdate := reFirst(`lastUpdateTime=([\d\- :]+)`, topBlock)
	if lastUpdate == "" {
		lastUpdate = reFirst(`timeStamp=([\d\- :]+)`, topBlock)
	}

	firstInstall := ""
	user0Rx := regexp.MustCompile(`(?s)User 0:.*?(?:\n\s+User \d+:|\z)`)
	if m := user0Rx.FindString(block); m != "" {
		firstInstall = reFirst(`firstInstallTime=([\d\- :]+)`, m)
	}

	codePath := reFirst(`codePath=(\S+)`, block)
	apkPath := ""
	if codePath != "" {
		apkPath = strings.TrimRight(codePath, "/") + "/base.apk"
	}
	if apkPath == "" {
		stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages -f " + pkg})
		for _, line := range strings.Split(stdout, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package:") && strings.Contains(line, "="+pkg) {
				line = strings.TrimPrefix(line, "package:")
				parts := strings.Split(line, "=")
				if len(parts) >= 1 {
					apkPath = strings.TrimSpace(parts[0])
				}
				break
			}
		}
	}

	apkSize := ""
	if apkPath != "" {
		stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "stat " + apkPath + " 2>/dev/null"})
		sizeRx := regexp.MustCompile(`Size:\s+(\d+)`)
		if m := sizeRx.FindStringSubmatch(stdout); len(m) >= 2 {
			var n int64
			fmt.Sscanf(m[1], "%d", &n)
			apkSize = fmtBytes(n)
		}
	}

	dataDir := "/data/data/" + pkg
	dataSize := ""
	stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "du -sh " + dataDir + " 2>/dev/null"})
	if code == 0 && strings.TrimSpace(stdout) != "" {
		parts := strings.Fields(stdout)
		if len(parts) > 0 {
			dataSize = parts[0]
		}
	}

	enabledStr := "enabled"
	enabledRx := regexp.MustCompile(`(?s)User 0:.*?enabled=(\d+)`)
	if m := enabledRx.FindStringSubmatch(block); len(m) >= 2 {
		val := 0
		fmt.Sscanf(m[1], "%d", &val)
		if val != 0 && val != 1 {
			enabledStr = "disabled"
		}
	}

	installer := reFirst(`installerPackageName=(\S+)`, block)

	return map[string]string{
		"package":       pkg,
		"version_name":  versionName,
		"version_code":  versionCode,
		"min_sdk":       minSDK,
		"target_sdk":    targetSDK,
		"first_install": firstInstall,
		"last_update":   lastUpdate,
		"apk_path":      apkPath,
		"apk_size":      apkSize,
		"data_size":     dataSize,
		"enabled":       enabledStr,
		"installer":     installer,
	}, nil
}

// ---------------------------------------------------------------------------
// ActionAppInfo
// ---------------------------------------------------------------------------

// ActionAppInfo displays detailed information about an installed app.
func ActionAppInfo(serial, packageName string) error {
	if !packageExists(serial, packageName) {
		return nterrors.AdbError(fmt.Sprintf("Package not found on device: %s", packageName))
	}

	// We need the model for display; read it
	model, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
	model = strings.TrimSpace(model)

	info, err := parseAppInfo(serial, packageName)
	if err != nil {
		return err
	}

	field := func(label, value, fallback string) {
		if value == "" {
			value = fallback
		}
		fmt.Printf("  %-18s: %s\n", label, value)
	}

	fmt.Printf("\n  App Info — %s\n\n", model)
	field("Package", info["package"], "")
	field("Version name", info["version_name"], "not available")
	field("Version code", info["version_code"], "not available")
	field("Min SDK", info["min_sdk"], "not available")
	field("Target SDK", info["target_sdk"], "not available")
	field("First installed", info["first_install"], "not available")
	field("Last updated", info["last_update"], "not available")
	field("APK path", info["apk_path"], "not available")
	field("APK size", info["apk_size"], "not available")
	field("Data size", info["data_size"], "(no root / not available)")
	field("Status", info["enabled"], "not available")
	field("Installer", installerLabel(info["installer"]), "")
	fmt.Println()
	return nil
}

// ---------------------------------------------------------------------------
// ActionKillApp
// ---------------------------------------------------------------------------

// ActionKillApp force-stops an app.
func ActionKillApp(serial, packageName string) error {
	if !packageExists(serial, packageName) {
		return nterrors.AdbError(fmt.Sprintf("Package not found on device: %s", packageName))
	}

	model, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
	model = strings.TrimSpace(model)

	fmt.Printf("  Force-stopping %s on %s...\n", packageName, model)
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "am force-stop " + packageName})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("am force-stop failed: %s", strings.TrimSpace(stderr)))
	}
	fmt.Println("  Done.")
	return nil
}

// ---------------------------------------------------------------------------
// ActionLaunchApp
// ---------------------------------------------------------------------------

// ActionLaunchApp launches an app by package name or deep link.
// If deepLink is non-empty, it launches an ACTION_VIEW intent.
func ActionLaunchApp(serial, packageName, deepLink string) error {
	model, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
	model = strings.TrimSpace(model)

	if packageName != "" && deepLink != "" {
		return nterrors.AdbError("Specify either --package or --deep-link, not both.")
	}

	if deepLink != "" {
		fmt.Printf("  Starting VIEW intent: %s\n", deepLink)
		stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"am start -a android.intent.action.VIEW -d " + deepLink})
		output := strings.TrimSpace(stdout + stderr)
		if code != 0 || strings.Contains(output, "Error") {
			return nterrors.AdbError(fmt.Sprintf("Failed to start intent '%s': %s", deepLink, output))
		}
		fmt.Println("  Intent sent.")
		return nil
	}

	if packageName != "" {
		if !packageExists(serial, packageName) {
			return nterrors.AdbError(fmt.Sprintf("Package not found on device: %s", packageName))
		}
		fmt.Printf("  Launching %s on %s...\n", packageName, model)
		stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
			"monkey -p " + packageName + " -c android.intent.category.LAUNCHER 1"})
		output := strings.TrimSpace(stdout + stderr)
		if code != 0 || strings.Contains(output, "Error") || strings.Contains(output, "aborted") {
			return nterrors.AdbError(fmt.Sprintf("Failed to launch %s: %s", packageName, output))
		}
		fmt.Println("  Launched.")
		return nil
	}

	// Interactive selection
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages -3"})
	var packages []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "package:") {
			packages = append(packages, strings.TrimPrefix(line, "package:"))
		}
	}

	if len(packages) == 0 {
		fmt.Println("  No user-installed apps found.")
		return nil
	}

	fmt.Printf("\n  User apps on %s:\n\n", model)
	for i, pkg := range packages {
		fmt.Printf("  %3d. %s\n", i+1, pkg)
	}
	fmt.Println()

	raw, err := adb.Prompt("  Enter number to launch (or press Enter to cancel): ")
	if err != nil || raw == "" {
		fmt.Println("  Cancelled.")
		return nil
	}

	var idx int
	if _, err := fmt.Sscanf(raw, "%d", &idx); err != nil || idx < 1 || idx > len(packages) {
		fmt.Println("  Invalid selection.")
		return nil
	}

	chosenPkg := packages[idx-1]
	fmt.Printf("  Launching %s on %s...\n", chosenPkg, model)
	stdout2, stderr2, code2 := adb.Run([]string{"adb", "-s", serial, "shell",
		"monkey -p " + chosenPkg + " -c android.intent.category.LAUNCHER 1"})
	output := strings.TrimSpace(stdout2 + stderr2)
	if code2 != 0 || strings.Contains(output, "Error") || strings.Contains(output, "aborted") {
		return nterrors.AdbError(fmt.Sprintf("Failed to launch %s: %s", chosenPkg, output))
	}
	fmt.Println("  Launched.")
	return nil
}

// ---------------------------------------------------------------------------
// ActionPackageList
// ---------------------------------------------------------------------------

type pkgRow struct {
	Package     string `json:"package"`
	VersionCode string `json:"version_code"`
	APKPath     string `json:"apk_path"`
}

func getAllPackages(serial string, includeSystem bool) ([]pkgRow, error) {
	flags := "-3"
	if includeSystem {
		flags = ""
	}
	cmd := "pm list packages -f --show-versioncode"
	if flags != "" {
		cmd += " " + flags
	}
	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
	if code != 0 {
		return nil, nterrors.AdbError(fmt.Sprintf("pm list packages failed: %s", strings.TrimSpace(stderr)))
	}

	vcRx := regexp.MustCompile(`\s+versionCode:(\d+)$`)
	var rows []pkgRow
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.HasPrefix(line, "package:") {
			continue
		}
		line = strings.TrimPrefix(line, "package:")
		versionCode := ""
		if m := vcRx.FindStringSubmatch(line); len(m) >= 2 {
			versionCode = m[1]
			line = line[:vcRx.FindStringIndex(line)[0]]
		}
		apkPath := ""
		pkg := ""
		if idx := strings.LastIndex(line, "="); idx >= 0 {
			apkPath = strings.TrimSpace(line[:idx])
			pkg = strings.TrimSpace(line[idx+1:])
		} else {
			pkg = strings.TrimSpace(line)
		}
		rows = append(rows, pkgRow{Package: pkg, VersionCode: versionCode, APKPath: apkPath})
	}

	// simple sort by package name
	for i := 0; i < len(rows)-1; i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[i].Package > rows[j].Package {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	return rows, nil
}

// ActionPackageList lists installed packages in text, csv, or json format.
func ActionPackageList(serial, format string) error {
	model, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "getprop ro.product.model"})
	model = strings.TrimSpace(model)

	fmt.Printf("\r  Fetching packages from %s...", model)
	rows, err := getAllPackages(serial, false)
	if err != nil {
		return err
	}
	fmt.Print("\r" + strings.Repeat(" ", 60) + "\r")

	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rows)
	case "csv":
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"package", "version_code", "apk_path"})
		for _, r := range rows {
			_ = w.Write([]string{r.Package, r.VersionCode, r.APKPath})
		}
		w.Flush()
	default:
		if len(rows) == 0 {
			fmt.Println("  (no packages)")
			return nil
		}
		wPkg := len("Package")
		wVC := len("VersionCode")
		for _, r := range rows {
			if len(r.Package) > wPkg {
				wPkg = len(r.Package)
			}
			if len(r.VersionCode) > wVC {
				wVC = len(r.VersionCode)
			}
		}
		fmt.Printf("  %-*s  %-*s  APK Path\n", wPkg, "Package", wVC, "VersionCode")
		fmt.Printf("  %s\n", strings.Repeat("-", wPkg+wVC+4+40))
		for _, r := range rows {
			fmt.Printf("  %-*s  %-*s  %s\n", wPkg, r.Package, wVC, r.VersionCode, r.APKPath)
		}
	}

	fmt.Printf("\n  Total: %d packages\n", len(rows))
	return nil
}
