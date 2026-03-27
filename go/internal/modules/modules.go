package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/data"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const githubAPIBase = "https://api.github.com/repos"
const userAgent = "nothingctl/go"

type modulesJSON struct {
	Modules []ModuleInfo `json:"modules"`
}

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

func loadModules() ([]ModuleInfo, error) {
	var m modulesJSON
	if err := json.Unmarshal(data.ModulesJSON, &m); err != nil {
		return nil, fmt.Errorf("failed to parse modules.json: %w", err)
	}
	return m.Modules, nil
}

func githubLatest(repo string, usePrerelease bool) (string, []githubAsset, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	var url string
	if usePrerelease {
		url = fmt.Sprintf("%s/%s/releases?per_page=1", githubAPIBase, repo)
	} else {
		url = fmt.Sprintf("%s/%s/releases/latest", githubAPIBase, repo)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if usePrerelease {
		var releases []githubRelease
		if err := json.Unmarshal(body, &releases); err != nil || len(releases) == 0 {
			return "", nil, fmt.Errorf("failed to parse releases for %s", repo)
		}
		return releases[0].TagName, releases[0].Assets, nil
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return "", nil, fmt.Errorf("failed to parse release for %s", repo)
	}
	return rel.TagName, rel.Assets, nil
}

func findAsset(assets []githubAsset, pattern string) *githubAsset {
	rx := regexp.MustCompile("(?i)" + pattern)
	var matches []githubAsset
	for _, a := range assets {
		if rx.MatchString(a.Name) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	for _, a := range matches {
		if strings.Contains(strings.ToLower(a.Name), "arm64") {
			return &a
		}
	}
	return &matches[0]
}

func getInstalledModules(serial string) map[string]bool {
	stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "su -c 'ls /data/adb/modules/ 2>/dev/null'"})
	result := make(map[string]bool)
	if code != 0 || strings.TrimSpace(stdout) == "" {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		d := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if d != "" {
			result[d] = true
		}
	}
	return result
}

func getInstalledVersions(serial string) map[string]string {
	cmd := "su -c 'for d in /data/adb/modules/*/; do name=$(basename $d); ver=$(grep -m1 \"^version=\" $d/module.prop 2>/dev/null | cut -d= -f2); echo \"$name|$ver\"; done'"
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func normalizeModKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

func isInstalled(m ModuleInfo, installedDirs map[string]bool) bool {
	key := normalizeModKey(m.ID)
	for d := range installedDirs {
		if strings.Contains(normalizeModKey(d), key) {
			return true
		}
	}
	return false
}

func installedVersion(m ModuleInfo, installedDirs map[string]bool, versions map[string]string) string {
	key := normalizeModKey(m.ID)
	for d := range installedDirs {
		if strings.Contains(normalizeModKey(d), key) {
			return versions[d]
		}
	}
	return ""
}

func fuzzyFindDir(moduleID string, installedDirs map[string]bool) string {
	key := normalizeModKey(moduleID)
	var sorted []string
	for d := range installedDirs {
		sorted = append(sorted, d)
	}
	for _, d := range sorted {
		if strings.Contains(normalizeModKey(d), key) {
			return d
		}
	}
	return ""
}

func printModuleList(modules []ModuleInfo, installedDirs map[string]bool, installedVersions map[string]string, latestTags map[string]string, rootAvailable bool) {
	categories := []string{"framework", "privacy", "utility", "apps"}
	fmt.Println()
	for _, cat := range categories {
		var group []ModuleInfo
		for _, m := range modules {
			if m.Category == cat {
				group = append(group, m)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Printf("[%s]\n", cat)
		for _, m := range group {
			tag := latestTags[m.ID]
			latestStr := tag
			if latestStr == "" {
				latestStr = "?"
			}

			var status, verStr string
			if m.Source == "ksu_store" {
				status = "[manual install]"
				verStr = ""
			} else if !rootAvailable {
				status = "[unknown]"
				verStr = ""
			} else if isInstalled(m, installedDirs) {
				instVer := installedVersion(m, installedDirs, installedVersions)
				if instVer != "" && tag != "" {
					iv := strings.TrimPrefix(instVer, "v")
					tv := strings.TrimPrefix(tag, "v")
					if iv == tv {
						status = "[INSTALLED]"
						verStr = fmt.Sprintf("  %s — up to date", instVer)
					} else {
						status = "[UPDATE]"
						verStr = fmt.Sprintf("  %s -> %s", instVer, tag)
					}
				} else {
					status = "[INSTALLED]"
					if instVer == "" {
						instVer = "?"
					}
					verStr = fmt.Sprintf("  %s", instVer)
				}
			} else {
				status = "[not installed]"
				verStr = fmt.Sprintf("  latest: %s", latestStr)
			}

			zygiskStr := ""
			if m.RequiresZygisk {
				zygiskStr = "  [needs Zygisk]"
			}
			fmt.Printf("  %-24s %-16s%s%s\n", m.ID, status, verStr, zygiskStr)
			fmt.Printf("  %-24s %s\n", "", m.Description)
			if m.Notes != "" {
				fmt.Printf("  %-24s -> %s\n", "", m.Notes)
			}
			fmt.Println()
		}
	}

	fmt.Println("Usage:")
	fmt.Println("  List modules  :  nothingctl modules")
	fmt.Println("  Install one   :  nothingctl modules --install lsposed")
	fmt.Println("  Install set   :  nothingctl modules --install lsposed,shamiko,play-integrity-fix")
	fmt.Println("  Install all   :  nothingctl modules --install all")
}

func downloadModule(m ModuleInfo, baseDir string) (string, error) {
	if m.Source != "github" || m.Repo == "" {
		return "", nterrors.MagiskError(fmt.Sprintf("'%s' requires manual install.\n  → %s", m.ID, m.Notes))
	}
	tag, assets, err := githubLatest(m.Repo, m.UsePrerelease)
	if err != nil {
		return "", nterrors.MagiskError(fmt.Sprintf("failed to fetch release for %s: %v", m.ID, err))
	}
	asset := findAsset(assets, m.AssetPattern)
	if asset == nil {
		return "", nterrors.MagiskError(fmt.Sprintf("no matching asset for '%s' in %s @ %s", m.ID, m.Repo, tag))
	}

	destDir := filepath.Join(baseDir, "modules", m.ID, tag)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, asset.Name)

	if _, err := os.Stat(dest); err == nil {
		fmt.Printf("  Cached  : %s\n", asset.Name)
		return dest, nil
	}

	mb := asset.Size / 1024 / 1024
	fmt.Printf("  Download: %s (%d MB)...\n", asset.Name, mb)

	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return dest, nil
}

func installZip(localPath string, serial, model string) error {
	remote := "/data/local/tmp/" + filepath.Base(localPath)
	fmt.Printf("  Pushing %s...\n", filepath.Base(localPath))
	if err := adb.AdbPush(serial, localPath, remote); err != nil {
		return err
	}
	fmt.Println("  Installing via Magisk...")
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("su -c 'magisk --install-module %s && rm -f %s'", remote, remote)})
	if code != 0 {
		return nterrors.MagiskError(fmt.Sprintf("module install failed: %s", strings.TrimSpace(stderr)))
	}
	return nil
}

func installAPK(localPath, serial string, m ModuleInfo) error {
	if m.Notes != "" {
		fmt.Printf("  NOTE: %s\n", m.Notes)
	}
	fmt.Printf("  Installing APK %s...\n", filepath.Base(localPath))
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "install", "-r", localPath})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("APK install failed: %s", strings.TrimSpace(stderr)))
	}
	return nil
}

func installModule(m ModuleInfo, serial, baseDir string) error {
	if m.Source == "ksu_store" {
		fmt.Printf("\n[SKIP] %s — manual install required.\n", m.Name)
		fmt.Printf("       → %s\n", m.Notes)
		return nil
	}

	fmt.Printf("\nInstalling %s...\n", m.Name)
	localPath, err := downloadModule(m, baseDir)
	if err != nil {
		return err
	}

	if m.InstallType == "zip" {
		if err := installZip(localPath, serial, ""); err != nil {
			return err
		}
	} else {
		if err := installAPK(localPath, serial, m); err != nil {
			return err
		}
	}

	fmt.Printf("[OK] %s installed.\n", m.Name)
	if m.Notes != "" {
		fmt.Printf("     → %s\n", m.Notes)
	}
	return nil
}

// ActionModules lists recommended modules and/or installs them.
// installIDs: "" = list only, "all" = all github modules, "id,id,..." = specific ones.
func ActionModules(serial, baseDir string, installIDs []string) error {
	modules, err := loadModules()
	if err != nil {
		return err
	}

	rootOK := adb.CheckAdbRoot(serial)
	var installedDirs map[string]bool
	var installedVersions map[string]string
	if rootOK {
		installedDirs = getInstalledModules(serial)
		installedVersions = getInstalledVersions(serial)
	} else {
		installedDirs = make(map[string]bool)
		installedVersions = make(map[string]string)
	}

	if !rootOK && len(installIDs) > 0 {
		return nterrors.AdbError("Root required to install Magisk modules.\nEnable in Magisk: Settings → Superuser access → Apps and ADB.")
	}

	// Fetch latest release tags
	fmt.Print("Fetching release info")
	latestTags := make(map[string]string)
	for _, m := range modules {
		if m.Source == "github" && m.Repo != "" {
			tag, _, err := githubLatest(m.Repo, m.UsePrerelease)
			if err == nil {
				latestTags[m.ID] = tag
			}
		}
		fmt.Print(".")
	}
	fmt.Println()

	if len(installIDs) == 0 {
		printModuleList(modules, installedDirs, installedVersions, latestTags, rootOK)
		return nil
	}

	var targets []ModuleInfo
	if len(installIDs) == 1 && strings.ToLower(installIDs[0]) == "all" {
		for _, m := range modules {
			if m.Source == "github" {
				targets = append(targets, m)
			}
		}
	} else {
		idSet := make(map[string]bool)
		for _, id := range installIDs {
			idSet[strings.TrimSpace(id)] = true
		}
		foundIDs := make(map[string]bool)
		for _, m := range modules {
			if idSet[m.ID] {
				targets = append(targets, m)
				foundIDs[m.ID] = true
			}
		}
		for id := range idSet {
			if !foundIDs[id] {
				fmt.Printf("WARNING: Unknown ID: %s\n", id)
			}
		}
	}

	if len(targets) == 0 {
		fmt.Println("No modules to install.")
		return nil
	}

	var names []string
	for _, m := range targets {
		names = append(names, m.Name)
	}
	fmt.Printf("\nWill install: %s\n", strings.Join(names, ", "))
	if !adb.Confirm("Proceed?") {
		return nil
	}

	var failed []string
	for _, m := range targets {
		if err := installModule(m, serial, baseDir); err != nil {
			fmt.Printf("[FAIL] %s: %v\n", m.Name, err)
			failed = append(failed, m.ID)
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nFailed: %s\n", strings.Join(failed, ", "))
	}
	ok := len(targets) - len(failed)
	fmt.Printf("\n[OK] %d/%d modules installed.\n", ok, len(targets))
	if ok > 0 {
		fmt.Println("     Reboot device to activate modules.")
	}
	return nil
}

// ActionModulesStatus shows installed Magisk modules on device.
func ActionModulesStatus(serial string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError("Root not available via ADB shell.\nEnable in Magisk: Settings -> Superuser access -> Apps and ADB.")
	}

	installedDirs := getInstalledModules(serial)
	if len(installedDirs) == 0 {
		fmt.Println("No Magisk modules found in /data/adb/modules/.")
		return nil
	}

	checkCmd := "for d in /data/adb/modules/*/; do name=$(basename $d); if su -c 'test -f $d/disable' 2>/dev/null; then echo \"$name|disabled\"; else echo \"$name|enabled\"; fi; done"
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", checkCmd})

	states := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimRight(line, "\r")
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
			states[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	if len(states) == 0 {
		for d := range installedDirs {
			states[d] = "unknown"
		}
	}

	fmt.Println("\n  Installed Magisk modules:\n")
	var sorted []string
	for d := range states {
		sorted = append(sorted, d)
	}
	// simple sort
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for _, moduleDir := range sorted {
		state := strings.ToUpper(states[moduleDir])
		suffix := ""
		if state == "DISABLED" {
			suffix = "  <- has disable file"
		}
		fmt.Printf("  %-40s [%s]%s\n", moduleDir, state, suffix)
	}
	fmt.Println()
	return nil
}

// ActionModulesToggle enables or disables installed Magisk modules.
func ActionModulesToggle(serial string, moduleIDs []string, enable bool) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError("Root not available via ADB shell.\nEnable in Magisk: Settings -> Superuser access -> Apps and ADB.")
	}

	installedDirs := getInstalledModules(serial)
	if len(installedDirs) == 0 {
		return nterrors.AdbError("No Magisk modules found in /data/adb/modules/.")
	}

	for _, moduleID := range moduleIDs {
		moduleDir := fuzzyFindDir(moduleID, installedDirs)
		if moduleDir == "" {
			var dirs []string
			for d := range installedDirs {
				dirs = append(dirs, d)
			}
			return nterrors.AdbError(fmt.Sprintf("No installed module matching '%s' found.\nInstalled dirs: %s", moduleID, strings.Join(dirs, ", ")))
		}

		disableFile := fmt.Sprintf("/data/adb/modules/%s/disable", moduleDir)
		var cmd string
		if enable {
			cmd = fmt.Sprintf("su -c 'rm -f %s'", disableFile)
		} else {
			cmd = fmt.Sprintf("su -c 'touch %s'", disableFile)
		}

		_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", cmd})
		if code != 0 {
			action := "enable"
			if !enable {
				action = "disable"
			}
			return nterrors.AdbError(fmt.Sprintf("Failed to %s module '%s': %s", action, moduleDir, strings.TrimSpace(stderr)))
		}

		if enable {
			fmt.Printf("[OK] %s enabled — reboot to apply\n", moduleID)
		} else {
			fmt.Printf("[OK] %s disabled — reboot to apply\n", moduleID)
		}
	}
	return nil
}
