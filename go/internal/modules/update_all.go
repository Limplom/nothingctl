package modules

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

// ActionModulesUpdateAll checks all installed Magisk modules for available
// updates and installs them after user confirmation (skipped if force is true).
func ActionModulesUpdateAll(serial, baseDir string, force bool) error {
	all, err := loadModules()
	if err != nil {
		return err
	}

	fmt.Print("Checking installed modules...")
	installed := getInstalledModules(serial)
	installedVers := getInstalledVersions(serial)
	fmt.Println(" done")

	fmt.Print("Fetching latest release tags")
	latestTags := map[string]string{}
	for _, m := range all {
		if m.Source == "github" && m.Repo != "" {
			tag, _, err := githubLatest(m.Repo, m.UsePrerelease)
			if err == nil {
				latestTags[m.ID] = tag
			}
		}
		fmt.Print(".")
	}
	fmt.Println()

	// find modules that are installed and have a newer version available
	type updateTarget struct {
		m          ModuleInfo
		installedV string
		latestV    string
	}
	var targets []updateTarget
	for _, m := range all {
		if !isInstalled(m, installed) {
			continue
		}
		latest, ok := latestTags[m.ID]
		if !ok || latest == "" {
			continue
		}
		iv := installedVersion(m, installed, installedVers)
		ivTrimmed := strings.TrimPrefix(iv, "v")
		tv := strings.TrimPrefix(latest, "v")
		if ivTrimmed == "" || tv == "" || ivTrimmed == tv {
			continue
		}
		targets = append(targets, updateTarget{m, iv, latest})
	}

	if len(targets) == 0 {
		fmt.Println("All installed modules are up to date.")
		return nil
	}

	fmt.Printf("\n%-24s  %-14s  ->  %-14s\n", "MODULE", "INSTALLED", "LATEST")
	fmt.Printf("%-24s  %-14s  ->  %-14s\n", "------", "---------", "------")
	for _, t := range targets {
		fmt.Printf("%-24s  %-14s  ->  %-14s\n", t.m.ID, t.installedV, t.latestV)
	}
	fmt.Println()

	if !force {
		if !adb.Confirm(fmt.Sprintf("Update %d module(s)?", len(targets))) {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	var failed []string
	for _, t := range targets {
		fmt.Printf("\nUpdating %s...\n", t.m.ID)
		if err := installModule(t.m, serial, baseDir); err != nil {
			fmt.Printf("  [FAIL] %v\n", err)
			failed = append(failed, t.m.ID)
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nFailed: %s\n", strings.Join(failed, ", "))
	} else {
		fmt.Printf("\nAll modules updated successfully.\n")
	}
	return nil
}
