package magisk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"github.com/Limplom/nothingctl/internal/models"
)

const (
	magiskAPI = "https://api.github.com/repos/topjohnwu/Magisk/releases/latest"
	magiskUA  = "nothing-firmware-manager/2.0"
)

// CheckMagisk probes the device and GitHub to build a complete MagiskStatus.
func CheckMagisk(serial string) (*models.MagiskStatus, error) {
	ms := &models.MagiskStatus{}

	// 1. APK presence and version code via pm.
	stdout, _, _ := adb.Run([]string{
		"adb", "-s", serial, "shell", "pm list packages --show-versioncode",
	})
	var magiskLine string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "com.topjohnwu.magisk") {
			magiskLine = strings.TrimSpace(line)
			break
		}
	}
	ms.AppInstalled = magiskLine != ""
	if magiskLine != "" {
		re := regexp.MustCompile(`versionCode:(\d+)`)
		if m := re.FindStringSubmatch(magiskLine); m != nil {
			vc, _ := strconv.Atoi(m[1])
			ms.InstalledVersion = &vc
		}
	}

	// 2. Root active: daemon version via su (authoritative over APK versionCode).
	stdout, _, code := adb.Run([]string{
		"adb", "-s", serial, "shell", "su -c 'magisk -V 2>/dev/null'",
	})
	trimmed := strings.TrimSpace(stdout)
	if code == 0 && isDigit(trimmed) {
		vc, _ := strconv.Atoi(trimmed)
		ms.InstalledVersion = &vc
		ms.RootActive = true
	}

	// 3. Latest from GitHub (graceful failure if offline).
	latestVC, latestStr, latestURL, fetchErr := fetchLatestMagiskRelease()
	if fetchErr == nil {
		ms.LatestVersion = &latestVC
		ms.LatestVersionStr = &latestStr
		ms.LatestApkURL = &latestURL
	}

	return ms, nil
}

// fetchLatestMagiskRelease queries the Magisk GitHub releases API and returns
// the version code, human-readable version string, and APK download URL.
func fetchLatestMagiskRelease() (versionCode int, versionStr string, downloadURL string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, reqErr := http.NewRequest("GET", magiskAPI, nil)
	if reqErr != nil {
		return 0, "", "", reqErr
	}
	req.Header.Set("User-Agent", magiskUA)

	resp, doErr := client.Do(req)
	if doErr != nil {
		return 0, "", "", doErr
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return 0, "", "", readErr
	}

	var data map[string]any
	if jsonErr := json.Unmarshal(body, &data); jsonErr != nil {
		return 0, "", "", jsonErr
	}

	tag, _ := data["tag_name"].(string)
	versionStr = strings.TrimPrefix(tag, "v")
	versionCode = magiskTagToCode(tag)

	assets, _ := data["assets"].([]any)
	for _, a := range assets {
		asset, _ := a.(map[string]any)
		name, _ := asset["name"].(string)
		if strings.HasPrefix(name, "Magisk-v") && strings.HasSuffix(name, ".apk") {
			downloadURL, _ = asset["browser_download_url"].(string)
			break
		}
	}

	return versionCode, versionStr, downloadURL, nil
}

// FetchLatestMagiskRelease is the exported version for use by install.go.
func FetchLatestMagiskRelease() (version int, downloadURL string, err error) {
	vc, _, url, fetchErr := fetchLatestMagiskRelease()
	return vc, url, fetchErr
}

// magiskTagToCode converts a GitHub tag like "v30.7" to version code 30700.
func magiskTagToCode(tag string) int {
	re := regexp.MustCompile(`v?(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return 0
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major*1000 + minor*100
}

// PrintMagiskStatus prints a Magisk status summary and feature-availability
// table to stdout.
func PrintMagiskStatus(ms *models.MagiskStatus) {
	fmt.Printf("\n  Magisk : %s\n", ms.StateLabel())
	if ms.LatestVersionStr != nil {
		updateNote := "  [up to date]"
		if ms.IsOutdated() {
			updateNote = "  [UPDATE AVAILABLE]"
		}
		fmt.Printf("  Latest : v%s%s\n", *ms.LatestVersionStr, updateNote)
	}

	root := ms.RootActive

	type feature struct {
		name  string
		avail bool
		note  string
	}
	features := []feature{
		{"Firmware check + download", true, "always available"},
		{"--flash-firmware / --restore", true, "fastboot — no root needed"},
		{"--push-for-patch / --flash-patched", true, "fastboot — no root needed"},
		{"--backup (partition dump)", root, "requires root + ADB su"},
		{"Auto-backup before flash", root, "requires root + ADB su"},
		{"Performance tweaks (su)", root, "requires root + ADB su"},
		{"System cert install", root, "requires root + ADB su"},
		{"App private data access", root, "requires root + ADB su"},
	}

	if !root {
		fmt.Println("\n  Feature availability without active root:")
		for _, f := range features {
			mark := "[OK]  "
			if !f.avail {
				mark = "[N/A] "
			}
			fmt.Printf("    %s %s\n", mark, f.name)
			if !f.avail {
				fmt.Printf("           -> %s\n", f.note)
			}
		}
	}

	if !ms.AppInstalled {
		fmt.Println("\n  Run install-magisk to install Magisk and enable root features.")
	} else if ms.IsOutdated() {
		fmt.Println("\n  Run install-magisk to update Magisk.")
	}
}

// PrintMagiskStatusForSerial is a convenience wrapper that looks up the status
// and prints it, returning any error encountered.
func PrintMagiskStatusForSerial(serial string) error {
	ms, err := CheckMagisk(serial)
	if err != nil {
		return nterrors.MagiskError("checking Magisk status: " + err.Error())
	}
	PrintMagiskStatus(ms)
	return nil
}
