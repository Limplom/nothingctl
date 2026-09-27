package selfupdate

import (
	"runtime"
	"strings"
	"testing"

	"github.com/Limplom/nothingctl/internal/firmware"
)

func TestAssetName(t *testing.T) {
	got := assetName()
	want := "nothingctl-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got != want {
		t.Fatalf("assetName() = %q, want %q", got, want)
	}
	if runtime.GOOS != "windows" && strings.HasSuffix(got, ".exe") {
		t.Fatalf("non-windows asset name has .exe: %q", got)
	}
}

// TestAssetNameMatchesReleaseAssets verifies the platform binary is selected
// from a realistic release asset list and that checksum files or other
// platforms are never picked.
func TestAssetNameMatchesReleaseAssets(t *testing.T) {
	names := []string{
		"checksums.txt",
		"nothingctl-windows-amd64.exe",
		"nothingctl-linux-amd64",
		"nothingctl-linux-arm64",
		"nothingctl-darwin-amd64",
		"nothingctl-darwin-arm64",
	}
	var assets []any
	for _, n := range names {
		assets = append(assets, map[string]any{"name": n, "browser_download_url": "https://example/" + n})
	}
	release := map[string]any{"assets": assets}

	want := assetName()
	supported := false
	for _, n := range names {
		if n == want {
			supported = true
		}
	}
	if !supported {
		t.Skipf("platform %s not in release matrix", want)
	}
	name, url, ok := firmware.FindAsset(release, want)
	if !ok || name != want || url != "https://example/"+want {
		t.Fatalf("FindAsset(%q) = (%q,%q,%v)", want, name, url, ok)
	}
}

func TestIsUpToDate(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.2.0", "v1.2.0", true},
		{"1.2.0", "v1.2.0", true},
		{"v1.2.0", "1.2.0", true},
		{"v1.2.0", "v1.3.0", false},
		{"v1.3.0", "v1.2.0", false}, // not equal => update offered (no ordering)
		{"dev", "v9.9.9", true},
		{"", "v1.0.0", false},
	}
	for _, c := range cases {
		if got := isUpToDate(c.current, c.latest); got != c.want {
			t.Errorf("isUpToDate(%q,%q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

// TestIsUpToDateDevCommitBuild: local builds use -X main.Version=dev-<commit>
// (see CLAUDE.md); self-update must not overwrite them with a release.
func TestIsUpToDateDevCommitBuild(t *testing.T) {
	if !isUpToDate("dev-abc1234", "v1.0.0") {
		t.Fatal("dev-<commit> build would be replaced by release")
	}
}
