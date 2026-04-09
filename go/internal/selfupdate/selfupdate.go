// Package selfupdate checks the nothingctl GitHub repository for a newer
// release and, when one is found, downloads and replaces the running binary.
package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Limplom/nothingctl/internal/firmware"
)

const (
	selfOwner = "Limplom"
	selfRepo  = "nothingctl"
)

// assetName returns the expected release asset filename for the current platform.
func assetName() string {
	name := fmt.Sprintf("nothingctl-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// ActionSelfUpdate checks the nothingctl GitHub repo for a newer release.
// If one is found it downloads the correct binary and replaces the running
// executable. Pass dryRun=true to only print what would happen.
func ActionSelfUpdate(currentVersion string, dryRun bool) error {
	ctx := context.Background()

	fmt.Println("Checking for nothingctl updates...")

	releases, err := firmware.FetchReleasesCtx(ctx, selfOwner, selfRepo)
	if err != nil {
		return fmt.Errorf("could not fetch releases: %w", err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found")
	}

	latest := releases[0]
	latestTag, _ := latest["tag_name"].(string)
	if latestTag == "" {
		return fmt.Errorf("could not read latest release tag")
	}

	// Normalize versions for comparison
	cv := strings.TrimPrefix(currentVersion, "v")
	lv := strings.TrimPrefix(latestTag, "v")

	fmt.Printf("  Current : %s\n", currentVersion)
	fmt.Printf("  Latest  : %s\n", latestTag)

	if cv == lv || currentVersion == "dev" {
		fmt.Println("\nAlready on latest version.")
		return nil
	}

	// Find the asset for this platform
	want := assetName()
	_, assetURL, found := firmware.FindAsset(latest, want)
	if !found {
		return fmt.Errorf("no release asset found for platform: %s", want)
	}

	if dryRun {
		fmt.Printf("\n[dry-run] Would download: %s\n", assetURL)
		fmt.Printf("[dry-run] Would replace : %s\n", os.Args[0])
		return nil
	}

	// Determine running executable path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	// On Windows we cannot replace a running .exe — tell the user to do it manually
	if runtime.GOOS == "windows" {
		tmpPath := exePath + ".new"
		fmt.Printf("\nDownloading %s...\n", want)
		if err := firmware.DownloadFileCtx(ctx, assetURL, tmpPath, nil); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("chmod failed: %w", err)
		}
		fmt.Printf("\n[OK] Downloaded to: %s\n", tmpPath)
		fmt.Printf("     To complete the update, run:\n")
		fmt.Printf("     move /Y \"%s\" \"%s\"\n", tmpPath, exePath)
		return nil
	}

	// Unix: download to temp file in same dir, then atomically rename
	tmpPath := filepath.Join(filepath.Dir(exePath), ".nothingctl-update-tmp")
	fmt.Printf("\nDownloading %s...\n", want)
	if err := firmware.DownloadFileCtx(ctx, assetURL, tmpPath, nil); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("chmod failed: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("could not replace binary: %w", err)
	}

	fmt.Printf("\n[OK] Updated to %s — restart nothingctl to use the new version.\n", latestTag)
	return nil
}
