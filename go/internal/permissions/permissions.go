// Package permissions provides dangerous permission auditing for installed apps.
package permissions

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var dangerousPermissions = []string{
	"android.permission.READ_CONTACTS",
	"android.permission.WRITE_CONTACTS",
	"android.permission.READ_CALL_LOG",
	"android.permission.WRITE_CALL_LOG",
	"android.permission.READ_PHONE_STATE",
	"android.permission.CALL_PHONE",
	"android.permission.CAMERA",
	"android.permission.RECORD_AUDIO",
	"android.permission.ACCESS_FINE_LOCATION",
	"android.permission.ACCESS_COARSE_LOCATION",
	"android.permission.ACCESS_BACKGROUND_LOCATION",
	"android.permission.READ_EXTERNAL_STORAGE",
	"android.permission.WRITE_EXTERNAL_STORAGE",
	"android.permission.READ_MEDIA_IMAGES",
	"android.permission.READ_MEDIA_VIDEO",
	"android.permission.READ_MEDIA_AUDIO",
	"android.permission.BODY_SENSORS",
	"android.permission.ACTIVITY_RECOGNITION",
	"android.permission.SEND_SMS",
	"android.permission.RECEIVE_SMS",
	"android.permission.READ_SMS",
	"android.permission.BLUETOOTH_SCAN",
	"android.permission.BLUETOOTH_CONNECT",
	"android.permission.NEARBY_WIFI_DEVICES",
	"android.permission.USE_BIOMETRIC",
	"android.permission.USE_FINGERPRINT",
	"android.permission.PROCESS_OUTGOING_CALLS",
	"android.permission.READ_CALENDAR",
	"android.permission.WRITE_CALENDAR",
}

var dangerousSet map[string]bool

func init() {
	dangerousSet = make(map[string]bool)
	for _, p := range dangerousPermissions {
		dangerousSet[p] = true
	}
}

func shortPerm(p string) string {
	return strings.TrimPrefix(p, "android.permission.")
}


func parseGrantedDangerous(output string) []string {
	var granted []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.Contains(line, "android.permission.") {
			continue
		}
		if !strings.Contains(line, "granted=true") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		perm := strings.TrimSpace(line[:colonIdx])
		if dangerousSet[perm] {
			granted = append(granted, perm)
		}
	}
	return granted
}

func parseAllDangerous(output string) (granted, notGranted []string) {
	grantedSet := make(map[string]bool)
	seenDangerous := make(map[string]bool)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if !strings.Contains(line, "android.permission.") {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		perm := strings.TrimSpace(line[:colonIdx])
		if !dangerousSet[perm] {
			continue
		}
		seenDangerous[perm] = true
		if strings.Contains(line, "granted=true") {
			grantedSet[perm] = true
		}
	}

	for _, p := range dangerousPermissions {
		if grantedSet[p] {
			granted = append(granted, p)
		}
	}
	for _, p := range dangerousPermissions {
		if !grantedSet[p] {
			notGranted = append(notGranted, p)
		}
	}
	return
}

func auditSingle(serial, pkg string) error {
	output := adb.DumpsysPackage(serial, pkg)
	if output == "" {
		return nterrors.AdbError(fmt.Sprintf("dumpsys package %s returned no output", pkg))
	}

	// Detect package not found
	if !strings.Contains(output, "Package ["+pkg+"]") && !strings.Contains(output, "package:"+pkg) {
		stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages " + pkg})
		if !strings.Contains(stdout, "package:"+pkg) {
			return nterrors.AdbError(fmt.Sprintf("Package not found on device: %s", pkg))
		}
	}

	granted, notGranted := parseAllDangerous(output)

	fmt.Printf("\n  Permissions for %s:\n\n", pkg)

	if len(granted) > 0 {
		fmt.Println("  GRANTED (dangerous):")
		for _, p := range granted {
			fmt.Printf("    %s\n", shortPerm(p))
		}
	} else {
		fmt.Println("  GRANTED (dangerous): none")
	}

	fmt.Println()
	fmt.Println("  NOT GRANTED (dangerous):")
	for _, p := range notGranted {
		fmt.Printf("    %s\n", shortPerm(p))
	}
	fmt.Println()
	return nil
}

func auditAll(serial string) error {
	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages -3"})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("Failed to list packages: %s", strings.TrimSpace(stderr)))
	}

	var packages []string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "package:") {
			packages = append(packages, strings.TrimPrefix(line, "package:"))
		}
	}

	total := len(packages)
	type result struct {
		pkg   string
		perms []string
	}
	var results []result

	for i, pkg := range packages {
		fmt.Printf("\r  Scanning %d/%d...", i+1, total)
		output := adb.DumpsysPackage(serial, pkg)
		if output == "" {
			continue
		}
		granted := parseGrantedDangerous(output)
		if len(granted) > 0 {
			results = append(results, result{pkg: pkg, perms: granted})
		}
	}

	// Clear progress
	fmt.Print("\r" + strings.Repeat(" ", 40) + "\r")

	if len(results) == 0 {
		fmt.Println("  [OK] No user-installed apps have dangerous permissions granted.")
		return nil
	}

	fmt.Printf("  Permission Audit — %d apps with dangerous permissions\n\n", len(results))
	for _, r := range results {
		var short []string
		for _, p := range r.perms {
			short = append(short, shortPerm(p))
		}
		fmt.Printf("  %s\n", r.pkg)
		fmt.Printf("    %s\n\n", strings.Join(short, ", "))
	}

	fmt.Println("  Run with --package <pkg> for per-app detail.")
	return nil
}

// ActionPermissions audits dangerous permissions for a package or all user apps.
func ActionPermissions(serial, packageName string) error {
	if packageName != "" {
		return auditSingle(serial, packageName)
	}
	return auditAll(serial)
}
