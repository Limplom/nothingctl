package storage

import "testing"

func TestParseDU(t *testing.T) {
	cases := []struct {
		line     string
		wantKB   int
		wantPath string
		wantOK   bool
	}{
		{"1048576\t/sdcard/DCIM/", 1048576, "/sdcard/DCIM/", true},
		{"42\t/sdcard/My Folder/\r", 42, "/sdcard/My Folder/", true},
		{"0\t/sdcard/empty/", 0, "/sdcard/empty/", true},
		{"1024 /sdcard/spaces-not-tab/", 0, "", false},
		{"du: /sdcard/x: Permission denied", 0, "", false},
		{"abc\t/sdcard/", 0, "", false},
		{"", 0, "", false},
	}
	for _, c := range cases {
		kb, path, ok := parseDU(c.line)
		if kb != c.wantKB || path != c.wantPath || ok != c.wantOK {
			t.Errorf("parseDU(%q) = (%d, %q, %v), want (%d, %q, %v)",
				c.line, kb, path, ok, c.wantKB, c.wantPath, c.wantOK)
		}
	}
}

func TestFmtSize(t *testing.T) {
	cases := []struct {
		kb   int
		want string
	}{
		{0, "     0 KB"},
		{1023, "  1023 KB"},
		{1024, "   1.0 MB"},
		{1536, "   1.5 MB"},
		{1024 * 1024, "   1.0 GB"},
		{120 * 1024 * 1024, " 120.0 GB"},
	}
	for _, c := range cases {
		if got := fmtSize(c.kb); got != c.want {
			t.Errorf("fmtSize(%d) = %q, want %q", c.kb, got, c.want)
		}
	}
}

func TestParseDfFree(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/dev/fuse        111G  45G   66G  41% /storage/emulated", "free 66G of 111G"},
		{"/dev/block/dm-5  111G  45G   66G  41% /data\r\n", "free 66G of 111G"},
		{"Filesystem      Size  Used Avail Use% Mounted on", "unknown"}, // header only
		{"garbage", "unknown"},
		{"", "unknown"},
	}
	for _, c := range cases {
		if got := parseDfFree(c.in); got != c.want {
			t.Errorf("parseDfFree(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
