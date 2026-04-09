// Package debloat provides NothingOS bloatware removal via pm uninstall --user 0 (reversible).
package debloat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/data"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// PackageEntry represents a single debloat entry from debloat.json.
type PackageEntry struct {
	ID       string   `json:"id"`
	Package  string   `json:"package"`
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Notes    string   `json:"notes"`
	Profiles []string `json:"profiles,omitempty"`
}

type debloatJSON struct {
	Packages []PackageEntry `json:"packages"`
}

// loadPackages parses the embedded debloat.json.
func loadPackages() ([]PackageEntry, error) {
	var d debloatJSON
	if err := json.Unmarshal(data.DebloatJSON, &d); err != nil {
		return nil, fmt.Errorf("failed to parse debloat.json: %w", err)
	}
	return d.Packages, nil
}

func isInstalled(serial, pkg string) bool {
	stdout, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "pm list packages " + pkg})
	return strings.Contains(stdout, "package:"+pkg)
}

// ActionDebloat lists debloat package status, or disables listed package IDs.
// removeIDs may be nil (list mode), []string{"all"}, or specific IDs.
func ActionDebloat(serial string, removeIDs []string) error {
	packages, err := loadPackages()
	if err != nil {
		return err
	}

	// List mode
	if len(removeIDs) == 0 {
		fmt.Printf("\n%-22s %-28s %-14s Notes\n", "ID", "Name", "Status")
		fmt.Println(strings.Repeat("─", 90))
		for _, p := range packages {
			status := "not installed"
			marker := "  "
			if isInstalled(serial, p.Package) {
				status = "INSTALLED"
				marker = "->"
			}
			fmt.Printf(" %s %-20s %-28s %-14s %s\n", marker, p.ID, p.Name, status, p.Notes)
		}
		fmt.Println()
		fmt.Println("Remove:  nothingctl debloat --remove <id,id,...|all>")
		fmt.Println("Restore: adb shell pm install-existing --user 0 <package>")
		return nil
	}

	// Remove mode
	var targets []PackageEntry
	if len(removeIDs) == 1 && removeIDs[0] == "all" {
		targets = packages
	} else {
		idSet := make(map[string]bool)
		for _, id := range removeIDs {
			idSet[strings.TrimSpace(id)] = true
		}
		foundIDs := make(map[string]bool)
		for _, p := range packages {
			if idSet[p.ID] {
				targets = append(targets, p)
				foundIDs[p.ID] = true
			}
		}
		var missing []string
		for id := range idSet {
			if !foundIDs[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			return nterrors.AdbError(fmt.Sprintf("Unknown package ID(s): %s\nRun debloat without --remove to see available IDs.", strings.Join(missing, ", ")))
		}
	}

	var installed []PackageEntry
	for _, p := range targets {
		if isInstalled(serial, p.Package) {
			installed = append(installed, p)
		}
	}

	if len(installed) == 0 {
		fmt.Println("\nAll selected packages are already removed.")
		return nil
	}

	fmt.Printf("\nWill disable %d package(s) for user 0 (fully reversible):\n", len(installed))
	for _, p := range installed {
		fmt.Printf("  %-28s %s\n", p.Name, p.Package)
	}

	if !adb.Confirm("\nProceed?") {
		return nil
	}

	var failed []PackageEntry
	for _, p := range installed {
		fmt.Printf("  Removing %s... ", p.Name)
		stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "pm uninstall --user 0 " + p.Package})
		if strings.Contains(stdout, "Success") || code == 0 {
			fmt.Println("OK")
		} else {
			fmt.Printf("FAILED (%s)\n", strings.TrimSpace(stdout))
			failed = append(failed, p)
		}
	}

	fmt.Println()
	if len(failed) > 0 {
		var names []string
		for _, p := range failed {
			names = append(names, p.Name)
		}
		fmt.Printf("WARNING: %d package(s) could not be removed: %s\n", len(failed), strings.Join(names, ", "))
	}
	ok := len(installed) - len(failed)
	fmt.Printf("[OK] %d/%d packages removed.\n", ok, len(installed))
	fmt.Println("\nTo restore any package:")
	for _, p := range installed {
		fmt.Printf("  adb shell pm install-existing --user 0 %s\n", p.Package)
	}
	return nil
}

// ActionDebloatProfile disables all packages tagged with the given profile.
// Valid profiles: "minimal", "recommended", "aggressive".
func ActionDebloatProfile(serial, profile string) error {
	packages, err := loadPackages()
	if err != nil {
		return err
	}
	var ids []string
	for _, p := range packages {
		for _, pr := range p.Profiles {
			if pr == profile {
				ids = append(ids, p.ID)
				break
			}
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("no packages tagged with profile %q — valid profiles: minimal, recommended, aggressive", profile)
	}
	fmt.Printf("  Applying debloat profile: %s (%d packages)\n\n", profile, len(ids))
	return ActionDebloat(serial, ids)
}

// ActionRestoreDebloat reinstalls removed packages for the given IDs.
func ActionRestoreDebloat(serial string, packageIDs []string) error {
	packages, err := loadPackages()
	if err != nil {
		return err
	}

	idSet := make(map[string]bool)
	for _, id := range packageIDs {
		idSet[strings.TrimSpace(id)] = true
	}

	var targets []PackageEntry
	for _, p := range packages {
		if idSet[p.ID] {
			targets = append(targets, p)
		}
	}

	if len(targets) == 0 {
		fmt.Println("No matching packages found.")
		return nil
	}

	for _, p := range targets {
		fmt.Printf("  Restoring %s (%s)... ", p.Name, p.Package)
		stdout, _, code := adb.Run([]string{"adb", "-s", serial, "shell", "pm install-existing --user 0 " + p.Package})
		if strings.Contains(stdout, "Success") || code == 0 {
			fmt.Println("OK")
		} else {
			fmt.Printf("FAILED (%s)\n", strings.TrimSpace(stdout))
		}
	}
	return nil
}
