package backup

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func sum(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

// captureStdout runs fn and returns what it printed to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() { os.Stdout = old }()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

func TestSha256File(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, content string
	}{
		{"empty.img", ""},
		{"abc.img", "abc"},
		{"big.img", strings.Repeat("\x00\xff", 300_000)}, // > io.Copy buffer
	}
	for _, c := range cases {
		p := write(t, dir, c.name, c.content)
		got, err := sha256File(p)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != sum(c.content) {
			t.Errorf("%s: got %s, want %s", c.name, got, sum(c.content))
		}
	}
	// Known vector.
	if got, _ := sha256File(filepath.Join(dir, "abc.img")); got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("abc vector mismatch: %s", got)
	}
	if _, err := sha256File(filepath.Join(dir, "missing.img")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSaveChecksums(t *testing.T) {
	dir := t.TempDir()
	b := write(t, dir, "boot_b.img", "B")
	a := write(t, dir, "boot_a.img", "A")
	v := write(t, dir, "vbmeta_a.img", "V")

	captureStdout(t, func() {
		if err := saveChecksums([]string{v, b, a}, dir); err != nil {
			t.Fatal(err)
		}
	})
	raw, err := os.ReadFile(filepath.Join(dir, "checksums.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	want := sum("A") + "  boot_a.img\n" + sum("B") + "  boot_b.img\n" + sum("V") + "  vbmeta_a.img\n"
	if string(raw) != want {
		t.Errorf("checksums file:\n%s\nwant:\n%s", raw, want)
	}
	// Round trip through the verify parser.
	parsed := parseChecksums(string(raw))
	wantMap := map[string]string{"boot_a.img": sum("A"), "boot_b.img": sum("B"), "vbmeta_a.img": sum("V")}
	if !reflect.DeepEqual(parsed, wantMap) {
		t.Errorf("round trip: got %v, want %v", parsed, wantMap)
	}
}

func TestSaveChecksumsMissingImage(t *testing.T) {
	dir := t.TempDir()
	err := saveChecksums([]string{filepath.Join(dir, "gone.img")}, dir)
	if err == nil {
		t.Fatal("expected error for missing image")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "checksums.sha256")); statErr == nil {
		t.Error("checksums.sha256 must not be written when hashing fails")
	}
}

func TestParseChecksums(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"crlf and blank lines", "aa  boot_a.img\r\n\r\nbb  boot_b.img\r\n",
			map[string]string{"boot_a.img": "aa", "boot_b.img": "bb"}},
		{"single space ignored", "aa boot_a.img\n", map[string]string{}},
		{"filename with spaces kept", "aa  my file.img\n", map[string]string{"my file.img": "aa"}},
		{"duplicate last wins", "aa  x.img\nbb  x.img\n", map[string]string{"x.img": "bb"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseChecksums(c.raw); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestSortedKeysAndJoin(t *testing.T) {
	cases := []struct {
		m        map[string]string
		wantKeys []string
		wantJoin string
	}{
		{map[string]string{}, []string{}, ""},
		{map[string]string{"b": "", "a": "", "c": ""}, []string{"a", "b", "c"}, "a, b, c"},
		{map[string]string{"vbmeta_a": "", "boot_b": "", "boot_a": ""}, []string{"boot_a", "boot_b", "vbmeta_a"}, "boot_a, boot_b, vbmeta_a"},
	}
	for _, c := range cases {
		if got := sortedKeys(c.m); !reflect.DeepEqual(got, c.wantKeys) {
			t.Errorf("sortedKeys = %v, want %v", got, c.wantKeys)
		}
		if got := joinMapKeys(c.m); got != c.wantJoin {
			t.Errorf("joinMapKeys = %q, want %q", got, c.wantJoin)
		}
	}
}

func TestListBackups(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Backups", "partition-backup")
	for _, d := range []string{"backup_20250101_120000", "backup_20260301_080000", "backup_20251231_235959", "other_dir"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, root, "backup_file_not_dir", "x")

	got, err := ListBackups(base)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "backup_20260301_080000"),
		filepath.Join(root, "backup_20251231_235959"),
		filepath.Join(root, "backup_20250101_120000"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestListBackupsMissingRoot(t *testing.T) {
	got, err := ListBackups(t.TempDir())
	if err != nil || got != nil {
		t.Fatalf("got (%v,%v), want (nil,nil)", got, err)
	}
}

// TestListBackupsLabelledOrdering: labelled auto-backups (backup_pre_flash_<tag>,
// backup_pre_patch_flash, …) have no timestamp in their name and must be
// ordered by when they were taken, not sort ahead of newer timestamped ones —
// PickBackup's default [0] has to be the newest backup.
func TestListBackupsLabelledOrdering(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Backups", "partition-backup")
	local := func(s string) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	// Labelled backups: date comes from checksums.sha256 (written last) or,
	// without it, from the directory mtime.
	labelled := []struct {
		name     string
		when     time.Time
		checksum bool
	}{
		{"backup_pre_flash_Spacewar_V2.0-240101-0000", local("2025-06-01 10:00"), true},
		{"backup_pre_patch_flash", local("2026-05-01 09:00"), true},
		{"backup_pre_ota_Spacewar_V3.0-250101-0000", local("2026-02-01 12:00"), false},
	}
	for _, l := range labelled {
		dir := filepath.Join(root, l.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if l.checksum {
			p := write(t, dir, "checksums.sha256", "")
			if err := os.Chtimes(p, l.when, l.when); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(dir, l.when, l.when); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{"backup_20260301_080000", "backup_20250101_120000"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ListBackups(base)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range got {
		names = append(names, filepath.Base(g))
	}
	want := []string{
		"backup_pre_patch_flash",                     // 2026-05-01
		"backup_20260301_080000",                     // 2026-03-01
		"backup_pre_ota_Spacewar_V3.0-250101-0000",   // 2026-02-01 (dir mtime)
		"backup_pre_flash_Spacewar_V2.0-240101-0000", // 2025-06-01
		"backup_20250101_120000",                     // 2025-01-01
	}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got  %v\nwant %v", names, want)
	}
}

func TestBackupTime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "backup_20260301_080000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The name wins over file times for timestamped backups.
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	p := write(t, dir, "checksums.sha256", "")
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 1, 8, 0, 0, 0, time.Local)
	if got := backupTime(dir); !got.Equal(want) {
		t.Errorf("backupTime = %v, want %v", got, want)
	}
	// Missing directory: zero time, sorts last.
	if got := backupTime(filepath.Join(t.TempDir(), "backup_gone")); !got.IsZero() {
		t.Errorf("missing dir: got %v, want zero time", got)
	}
}

func TestActionVerifyBackup(t *testing.T) {
	cases := []struct {
		name        string
		files       map[string]string // on-disk images
		checksums   string            // checksums.sha256 content
		wantResults string
		wantMarker  string
		wantErr     bool
	}{
		{
			name:        "all match",
			files:       map[string]string{"boot_a.img": "A", "boot_b.img": "B"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("B") + "  boot_b.img\n",
			wantResults: "Results: 2 match  /  0 changed  /  0 missing",
			wantMarker:  "[OK] All files match",
		},
		{
			name:        "one changed",
			files:       map[string]string{"boot_a.img": "A", "boot_b.img": "tampered"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("B") + "  boot_b.img\n",
			wantResults: "Results: 1 match  /  1 changed  /  0 missing",
			wantMarker:  "Changed files: boot_b",
			wantErr:     true,
		},
		{
			name:        "one missing",
			files:       map[string]string{"boot_a.img": "A"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("B") + "  boot_b.img\n",
			wantResults: "Results: 1 match  /  0 changed  /  1 missing",
			wantMarker:  "not present in the backup directory",
			wantErr:     true,
		},
		{
			name:        "mixed",
			files:       map[string]string{"boot_a.img": "A", "dtbo_a.img": "x", "extra.img": "untracked"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("D") + "  dtbo_a.img\n" + sum("V") + "  vbmeta_a.img\n",
			wantResults: "Results: 1 match  /  1 changed  /  1 missing",
			wantMarker:  "Changed files: dtbo_a",
			wantErr:     true,
		},
		{
			name:        "uppercase hash is a mismatch",
			files:       map[string]string{"boot_a.img": "A"},
			checksums:   strings.ToUpper(sum("A")) + "  boot_a.img\n",
			wantResults: "Results: 0 match  /  1 changed  /  0 missing",
			wantMarker:  "Changed files: boot_a",
			wantErr:     true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, content := range c.files {
				write(t, dir, n, content)
			}
			write(t, dir, "checksums.sha256", c.checksums)
			var err error
			out := captureStdout(t, func() { err = ActionVerifyBackup(dir) })
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !strings.Contains(out, c.wantResults) {
				t.Errorf("output missing %q:\n%s", c.wantResults, out)
			}
			if !strings.Contains(out, c.wantMarker) {
				t.Errorf("output missing %q:\n%s", c.wantMarker, out)
			}
		})
	}
}

func TestActionVerifyBackupErrors(t *testing.T) {
	if err := ActionVerifyBackup(""); err == nil {
		t.Error("empty dir: expected error")
	}
	if err := ActionVerifyBackup(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("missing dir: expected error")
	}
	if err := ActionVerifyBackup(t.TempDir()); err == nil {
		t.Error("no checksums.sha256: expected error")
	}
}

// TestActionVerifyBackupMismatchExitCode: a changed or missing file must give
// a non-nil error (non-zero exit) so scripts can detect a corrupted backup.
func TestActionVerifyBackupMismatchExitCode(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "boot_a.img", "tampered")
	write(t, dir, "checksums.sha256", sum("A")+"  boot_a.img\n")
	if err := ActionVerifyBackup(dir); err == nil {
		t.Fatal("expected non-nil error for a mismatching backup")
	}
}

// TestBackupPartitionClassification checks that every dumped partition is
// classified exactly once for restore, and that no partition is both safe and
// risky (risky ones must never be flashed by a default restore).
func TestBackupPartitionClassification(t *testing.T) {
	for _, p := range backupPartitions {
		if restoreSafe[p] && restoreRisky[p] {
			t.Errorf("%s is both safe and risky", p)
		}
		if !restoreSafe[p] && !restoreRisky[p] {
			t.Errorf("%s is backed up but neither safe nor risky", p)
		}
	}
	for _, p := range []string{"persist", "nvram", "nvdata", "nvcfg", "factory", "seccfg", "proinfo", "preloader_raw_a", "lk_a", "tee_a"} {
		if restoreSafe[p] {
			t.Errorf("calibration/early-boot partition %s must not be restore-safe", p)
		}
	}
}

// TestActionRestoreDryRun exercises the restore selection logic without a
// device: only restore-safe images are selected, risky/unknown are skipped,
// and the partition filter narrows the set.
func TestActionRestoreDryRun(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"boot_a.img", "boot_b.img", "vbmeta_a.img", "persist.img", "nvram.img", "userdata.img", "notes.txt"} {
		write(t, dir, n, "x")
	}

	out := captureStdout(t, func() {
		if err := ActionRestore("serial", dir, true, nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Will flash 3 partitions") {
		t.Errorf("expected 3 safe partitions:\n%s", out)
	}
	for _, p := range []string{"persist", "nvram", "userdata"} {
		if strings.Contains(out, "fastboot flash "+p) {
			t.Errorf("dry run would flash non-safe partition %s:\n%s", p, out)
		}
	}

	out = captureStdout(t, func() {
		if err := ActionRestore("serial", dir, true, []string{"boot_a", "persist"}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Will flash 1 partitions") || strings.Contains(out, "fastboot flash persist") {
		t.Errorf("filter should select only boot_a:\n%s", out)
	}

	var err error
	captureStdout(t, func() { err = ActionRestore("serial", dir, true, []string{"persist"}) })
	if err == nil {
		t.Error("filter selecting only risky partitions must error, not flash")
	}
	if err := ActionRestore("serial", "", true, nil); err == nil {
		t.Error("empty backup dir: expected error")
	}
}
