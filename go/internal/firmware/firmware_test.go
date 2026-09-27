package firmware

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// touch creates empty files (or directories, when the name ends in "/") in dir.
func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		p := filepath.Join(dir, n)
		if n[len(n)-1] == '/' {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectBootTarget(t *testing.T) {
	cases := []struct {
		name    string
		files   []string
		want    string
		wantErr bool
	}{
		{"gki2 init_boot only", []string{"init_boot.img"}, "init_boot.img", false},
		{"legacy boot only", []string{"boot.img"}, "boot.img", false},
		// Both present (every GKI 2.0 package ships boot.img too): init_boot
		// must win, otherwise Magisk would patch the wrong image.
		{"both present prefers init_boot", []string{"boot.img", "init_boot.img"}, "init_boot.img", false},
		{"empty dir", nil, "", true},
		{"only unrelated images", []string{"vendor_boot.img", "dtbo.img", "init_boot.img.bak", "boot.IMG"}, "", true},
		{"magisk patched name is not a boot target", []string{"magisk_patched-27000_abcde.img"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, c.files...)
			got, err := DetectBootTarget(dir)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectBootTargetMissingDir(t *testing.T) {
	if _, err := DetectBootTarget(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestBuildPartitionList(t *testing.T) {
	cases := []struct {
		name    string
		files   []string
		want    []string
		wantErr bool
	}{
		{"gki2", []string{"init_boot.img", "boot.img"}, []string{"init_boot", "boot", "dtbo", "vendor_boot"}, false},
		{"legacy", []string{"boot.img"}, []string{"boot", "dtbo", "vendor_boot"}, false},
		// Extra files in the package must never widen the flash list.
		{"extra files ignored", []string{"boot.img", "modem.img", "preloader_raw.img", "persist.img", "system.img"},
			[]string{"boot", "dtbo", "vendor_boot"}, false},
		{"no boot image", []string{"dtbo.img"}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, c.files...)
			got, err := BuildPartitionList(dir)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestBuildPartitionListIndependentSlices guards against aliasing: mutating
// one returned list must not affect a later call.
func TestBuildPartitionListIndependentSlices(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "boot.img")
	a, _ := BuildPartitionList(dir)
	a[0] = "preloader_raw"
	b, _ := BuildPartitionList(dir)
	if b[0] != "boot" {
		t.Fatalf("second call returned %v — slices are aliased", b)
	}
}

func TestScanAvailableImages(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		allow []string
		want  []string
	}{
		{"empty dir", nil, firmwarePartitions, nil},
		{"only allowed are returned",
			[]string{"modem.img", "tee.img", "boot.img", "system.img", "random.img"},
			firmwarePartitions, []string{"modem", "tee"}},
		{"non-img files ignored",
			[]string{"modem.bin", "modem.img.bak", "modem", "tee.IMG", "lk.img.7z"},
			firmwarePartitions, nil},
		{"directory named like an image ignored",
			[]string{"modem.img/", "dsp.img"},
			firmwarePartitions, []string{"dsp"}},
		// Slot-suffixed files must not match base names (the caller adds _a/_b).
		{"slot suffixed names not matched",
			[]string{"boot_a.img", "vbmeta_b.img", "vbmeta.img"},
			bootPartitions, []string{"vbmeta"}},
		{"init_boot not in bootPartitions",
			[]string{"init_boot.img", "boot.img", "dtbo.img"},
			bootPartitions, []string{"boot", "dtbo"}},
		{"nil allow list", []string{"boot.img"}, nil, nil},
		{"logical partitions",
			[]string{"system.img", "vendor.img", "super.img", "userdata.img", "system_ext.img"},
			logicalPartitions, []string{"system", "system_ext", "vendor"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			touch(t, dir, c.files...)
			got := ScanAvailableImages(dir, c.allow)
			sort.Strings(got)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestScanAvailableImagesMissingDir(t *testing.T) {
	if got := ScanAvailableImages(filepath.Join(t.TempDir(), "nope"), firmwarePartitions); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

// TestPartitionListsDisjoint ensures no partition is flashed twice in one
// full-flash run (e.g. once in bootloader mode and again in fastbootd), and
// that never-flash partitions are absent from every list.
func TestPartitionListsDisjoint(t *testing.T) {
	seen := map[string]string{}
	lists := map[string][]string{
		"firmware": firmwarePartitions,
		"boot":     bootPartitions,
		"logical":  logicalPartitions,
	}
	for ln, l := range lists {
		for _, p := range l {
			if prev, ok := seen[p]; ok {
				t.Errorf("partition %q in both %s and %s lists", p, prev, ln)
			}
			seen[p] = ln
		}
	}
	for _, forbidden := range []string{"userdata", "super", "metadata", "init_boot", "frp", "seccfg", "nvram", "nvdata", "proinfo"} {
		if l, ok := seen[forbidden]; ok {
			t.Errorf("forbidden partition %q present in %s list", forbidden, l)
		}
	}
}

func TestImgPath(t *testing.T) {
	cases := []struct {
		dir, name, want string
	}{
		{"fw", "boot", filepath.Join("fw", "boot.img")},
		{filepath.Join("a", "b"), "init_boot", filepath.Join("a", "b", "init_boot.img")},
		{"", "vbmeta", "vbmeta.img"},
	}
	for _, c := range cases {
		if got := ImgPath(c.dir, c.name); got != c.want {
			t.Errorf("ImgPath(%q,%q) = %q, want %q", c.dir, c.name, got, c.want)
		}
	}
}

func rel(tag string) map[string]any { return map[string]any{"tag_name": tag} }

func TestLatestFromList(t *testing.T) {
	cases := []struct {
		name string
		tags []string
		want string
	}{
		{"single", []string{"Spacewar_V3.0-250101-1200"}, "Spacewar_V3.0-250101-1200"},
		{"picks newest date regardless of order",
			[]string{"Spacewar_V2.5-240301-1000", "Spacewar_V3.0-250113-1723", "Spacewar_V2.6-240615-0900"},
			"Spacewar_V3.0-250113-1723"},
		// Date key wins over version string: a hotfix for an older branch must
		// not be ranked above a newer build just because of its position.
		{"date beats list position",
			[]string{"Pong_V3.1-250301-0000", "Pong_V3.0-250401-0000"},
			"Pong_V3.0-250401-0000"},
		{"year boundary", []string{"Pong_V2.6-241231-2359", "Pong_V3.0-250101-0001"}, "Pong_V3.0-250101-0001"},
		{"tie keeps first", []string{"Pong_A-250101-1111", "Pong_B-250101-2222"}, "Pong_A-250101-1111"},
		{"undated tags fall back to first", []string{"Pong_foo", "Pong_bar"}, "Pong_foo"},
		{"dated beats undated", []string{"Pong_foo", "Pong_V3.0-250101-0001"}, "Pong_V3.0-250101-0001"},
		{"missing tag_name tolerated", []string{"", "Pong_V3.0-250101-0001"}, "Pong_V3.0-250101-0001"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rs []map[string]any
			for _, tag := range c.tags {
				if tag == "" {
					rs = append(rs, map[string]any{})
					continue
				}
				rs = append(rs, rel(tag))
			}
			got, _ := latestFromList(rs)["tag_name"].(string)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestLatestFromListEmpty documents that latestFromList panics on an empty
// slice. Its only caller (FetchLatestReleaseCtx) guards against that.
func TestLatestFromListEmpty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty list (caller must guard)")
		}
	}()
	latestFromList(nil)
}

func releaseWith(assets ...map[string]any) map[string]any {
	arr := make([]any, len(assets))
	for i, a := range assets {
		arr[i] = a
	}
	return map[string]any{"assets": arr}
}

func asset(name, url string) map[string]any {
	return map[string]any{"name": name, "browser_download_url": url}
}

func TestFindAsset(t *testing.T) {
	rel := releaseWith(
		asset("Spacewar_V3.0-image-firmware.7z", "u-fw"),
		asset("Spacewar_V3.0-image-boot.7z", "u-boot"),
		asset("Spacewar_V3.0-image-boot.7z.sha256", "u-boot-sha"),
		asset("nothingctl-linux-amd64", "u-linux"),
		asset("nothingctl-linux-amd64.sha256", "u-linux-sha"),
	)
	cases := []struct {
		name      string
		release   map[string]any
		pattern   string
		wantName  string
		wantURL   string
		wantFound bool
	}{
		{"boot archive", rel, "-image-boot.7z", "Spacewar_V3.0-image-boot.7z", "u-boot", true},
		{"suffix match, not substring", rel, "image-boot", "", "", false},
		{"platform binary not checksum", rel, "nothingctl-linux-amd64", "nothingctl-linux-amd64", "u-linux", true},
		{"missing", rel, "-image-logical.7z.001", "", "", false},
		{"no assets key", map[string]any{}, "-image-boot.7z", "", "", false},
		{"assets wrong type", map[string]any{"assets": "nope"}, "-image-boot.7z", "", "", false},
		{"non-map entries skipped", map[string]any{"assets": []any{"junk", 42, asset("x-image-boot.7z", "u")}},
			"-image-boot.7z", "x-image-boot.7z", "u", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, u, ok := FindAsset(c.release, c.pattern)
			if n != c.wantName || u != c.wantURL || ok != c.wantFound {
				t.Errorf("got (%q,%q,%v), want (%q,%q,%v)", n, u, ok, c.wantName, c.wantURL, c.wantFound)
			}
		})
	}
}

func TestFindAssetBySuffix(t *testing.T) {
	assets := []map[string]any{
		asset("X-image-logical.7z.001", "u1"),
		asset("X-image-logical.7z.002", "u2"),
		asset("X-image-logical.7z.010", "u10"),
		{"name": 5, "browser_download_url": "bad"},
	}
	cases := []struct {
		suffix, wantURL, wantName string
		wantErr                   bool
	}{
		{"-image-logical.7z.001", "u1", "X-image-logical.7z.001", false},
		{"-image-logical.7z.002", "u2", "X-image-logical.7z.002", false},
		{"-image-logical.7z.003", "", "", true},
		{"-image-logical.7z.010", "u10", "X-image-logical.7z.010", false},
		{"-image-firmware.7z", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.suffix, func(t *testing.T) {
			u, n, err := findAssetBySuffix(assets, c.suffix)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if u != c.wantURL || n != c.wantName {
				t.Errorf("got (%q,%q), want (%q,%q)", u, n, c.wantURL, c.wantName)
			}
		})
	}
	if _, _, err := findAssetBySuffix(nil, "x"); err == nil {
		t.Error("expected error for nil asset list")
	}
}
