package modules

import (
	"regexp"
	"strings"
	"testing"
)

func TestModulesLoad(t *testing.T) {
	mods, err := loadModules()
	if err != nil {
		t.Fatalf("loadModules: %v", err)
	}
	if len(mods) == 0 {
		t.Fatal("modules catalogue is empty")
	}
}

var repoRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)

func TestModuleEntriesValid(t *testing.T) {
	mods, err := loadModules()
	if err != nil {
		t.Fatal(err)
	}
	validCategory := map[string]bool{"framework": true, "privacy": true, "utility": true, "apps": true}
	validInstall := map[string]bool{"zip": true, "apk": true}

	ids := map[string]bool{}
	for _, m := range mods {
		if m.ID == "" || m.ID != strings.TrimSpace(m.ID) || strings.Contains(m.ID, ",") {
			t.Errorf("module %+v: bad id %q", m, m.ID)
		}
		if ids[m.ID] {
			t.Errorf("duplicate module id %q", m.ID)
		}
		ids[m.ID] = true
		if m.Name == "" {
			t.Errorf("module %q: empty name", m.ID)
		}
		if !validCategory[m.Category] {
			t.Errorf("module %q: unknown category %q", m.ID, m.Category)
		}
		if !validInstall[m.InstallType] {
			t.Errorf("module %q: unknown install_type %q", m.ID, m.InstallType)
		}
		switch m.Source {
		case "github":
			if !repoRe.MatchString(m.Repo) {
				t.Errorf("module %q: repo %q is not owner/repo", m.ID, m.Repo)
			}
			if m.AssetPattern == "" {
				t.Errorf("module %q: github source without asset_pattern", m.ID)
			}
			// findAsset uses regexp.MustCompile — a bad pattern would panic.
			if _, err := regexp.Compile("(?i)" + m.AssetPattern); err != nil {
				t.Errorf("module %q: asset_pattern does not compile: %v", m.ID, err)
			}
			wantExt := "." + m.InstallType
			if !strings.Contains(strings.ToLower(m.AssetPattern), strings.ToLower(wantExt)) &&
				!strings.Contains(m.AssetPattern, `\`+wantExt) {
				t.Errorf("module %q: asset_pattern %q does not target %s files", m.ID, m.AssetPattern, wantExt)
			}
		case "ksu_store":
			if m.Repo != "" {
				t.Errorf("module %q: ksu_store source should not set repo (%q)", m.ID, m.Repo)
			}
		default:
			t.Errorf("module %q: unknown source %q", m.ID, m.Source)
		}
	}
}

// TestModuleIDsDoNotShadow: isInstalled/installedVersion use substring
// matching on normalised IDs. If one catalogue ID's key were a substring of
// another's, installing the longer one would report the shorter as installed.
func TestModuleIDsDoNotShadow(t *testing.T) {
	mods, err := loadModules()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range mods {
		for _, b := range mods {
			if a.ID == b.ID {
				continue
			}
			ka, kb := normalizeModKey(a.ID), normalizeModKey(b.ID)
			if strings.Contains(kb, ka) {
				t.Errorf("module key %q (%s) is a substring of %q (%s)", ka, a.ID, kb, b.ID)
			}
		}
	}
}

func TestNormalizeModKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"zygisk-next", "zygisknext"},
		{"Zygisk_Next", "zygisknext"},
		{"play-integrity-fix", "playintegrityfix"},
		{"playintegrityfix", "playintegrityfix"},
		{"LSPosed", "lsposed"},
		{"", ""},
		{"-_-", ""},
	}
	for _, tc := range cases {
		if got := normalizeModKey(tc.in); got != tc.want {
			t.Errorf("normalizeModKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func dirs(ds ...string) map[string]bool {
	m := map[string]bool{}
	for _, d := range ds {
		m[d] = true
	}
	return m
}

func TestIsInstalledAndVersion(t *testing.T) {
	installed := dirs("zygisk_lsposed", "playintegrityfix", "zygisksu")
	versions := map[string]string{
		"zygisk_lsposed":   "v1.10.1",
		"playintegrityfix": "v15",
	}
	cases := []struct {
		id      string
		want    bool
		wantVer string
	}{
		{"lsposed", true, "v1.10.1"},
		{"play-integrity-fix", true, "v15"},
		{"zygisk-su", true, ""}, // installed, no module.prop version
		{"zygisk-next", false, ""},
		{"shamiko", false, ""},
	}
	for _, tc := range cases {
		m := ModuleInfo{ID: tc.id}
		if got := isInstalled(m, installed); got != tc.want {
			t.Errorf("isInstalled(%q) = %v, want %v", tc.id, got, tc.want)
		}
		if got := installedVersion(m, installed, versions); got != tc.wantVer {
			t.Errorf("installedVersion(%q) = %q, want %q", tc.id, got, tc.wantVer)
		}
	}
	if isInstalled(ModuleInfo{ID: "lsposed"}, nil) {
		t.Error("isInstalled with no dirs should be false")
	}
}

func TestFuzzyFindDir(t *testing.T) {
	cases := []struct {
		name      string
		id        string
		installed map[string]bool
		want      string
	}{
		{"exact", "shamiko", dirs("shamiko", "zygisksu"), "shamiko"},
		{"normalised", "zygisk-next", dirs("zygisk_next"), "zygisk_next"},
		{"substring", "lsposed", dirs("zygisk_lsposed"), "zygisk_lsposed"},
		{"no match", "shamiko", dirs("zygisk_lsposed"), ""},
		{"empty map", "shamiko", nil, ""},
		// Exact normalised match wins over a substring match regardless of
		// map iteration order.
		{"prefer exact", "lsposed", dirs("a_lsposed_old", "lsposed", "zygisk_lsposed"), "lsposed"},
		// Several substring matches: deterministic (lexicographically first).
		{"deterministic", "lsposed", dirs("zygisk_lsposed", "lsposed_it", "b_lsposed"), "b_lsposed"},
		// IDs that normalise to "" must not match every directory — this
		// drives a "touch disable" / "rm disable" under /data/adb/modules.
		{"empty key", "_", dirs("shamiko", "zygisksu"), ""},
		{"empty id", "", dirs("shamiko"), ""},
	}
	for _, tc := range cases {
		// Repeat to shake out map-order nondeterminism.
		for i := 0; i < 20; i++ {
			if got := fuzzyFindDir(tc.id, tc.installed); got != tc.want {
				t.Errorf("%s: fuzzyFindDir(%q) = %q, want %q", tc.name, tc.id, got, tc.want)
				break
			}
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := func(names ...string) []githubAsset {
		var out []githubAsset
		for _, n := range names {
			out = append(out, githubAsset{Name: n})
		}
		return out
	}
	cases := []struct {
		name    string
		assets  []githubAsset
		pattern string
		want    string // "" = nil
	}{
		{"none", nil, `.*\.zip`, ""},
		{"no match", assets("foo.apk"), `.*\.zip`, ""},
		{"single", assets("notes.txt", "Shamiko-v1.2.1-383-release.zip"), `Shamiko-v[\d.]+-[\d]+-release\.zip`, "Shamiko-v1.2.1-383-release.zip"},
		{"case-insensitive", assets("YOUTUBE-revanced.APK"), `youtube.*\.apk`, "YOUTUBE-revanced.APK"},
		{"double (?i) prefix", assets("youtube-arm-v7a.apk"), `(?i)youtube.*\.apk`, "youtube-arm-v7a.apk"},
		{"prefers arm64", assets("youtube-arm-v7a.apk", "youtube-x86.apk", "youtube-arm64-v8a.apk"), `youtube.*\.apk`, "youtube-arm64-v8a.apk"},
		{"first when no arm64", assets("a-youtube.apk", "b-youtube.apk"), `youtube.*\.apk`, "a-youtube.apk"},
	}
	for _, tc := range cases {
		got := findAsset(tc.assets, tc.pattern)
		gotName := ""
		if got != nil {
			gotName = got.Name
		}
		if gotName != tc.want {
			t.Errorf("%s: findAsset = %q, want %q", tc.name, gotName, tc.want)
		}
	}
}

// TestCatalogueAssetPatterns runs each catalogue pattern against a realistic
// release asset name so a typo in modules.json fails here, not on-device.
func TestCatalogueAssetPatterns(t *testing.T) {
	samples := map[string]string{
		"lsposed":             "LSPosed-v1.10.1-7115-zygisk-release.zip",
		"zygisk-next":         "Zygisk-Next-1.2.9-512-abcdef0-release.zip",
		"shamiko":             "Shamiko-v1.2.1-383-release.zip",
		"play-integrity-fix":  "PlayIntegrityFork-v15.zip",
		"disable-flag-secure": "DisableFlagSecure-v1.3.0.zip",
		"revanced":            "youtube-revanced-extended-v19.16.39-arm64-v8a.apk",
	}
	mods, err := loadModules()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mods {
		sample, ok := samples[m.ID]
		if !ok || m.Source != "github" {
			continue
		}
		if a := findAsset([]githubAsset{{Name: "checksums.txt"}, {Name: sample}}, m.AssetPattern); a == nil || a.Name != sample {
			t.Errorf("module %q: pattern %q does not select %q", m.ID, m.AssetPattern, sample)
		}
	}
}
