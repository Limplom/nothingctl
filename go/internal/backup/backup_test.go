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

// TestListBackupsLabelledOrdering documents that labelled auto-backups
// (backup_pre_flash_<tag>, backup_pre_ota_<tag>, backup_pre_patch_flash) sort
// by name, not by time: they always appear before timestamped backups and
// PickBackup's default [0] may therefore be an old labelled backup.
func TestListBackupsLabelledOrdering(t *testing.T) {
	t.Skip("ambiguous: ListBackups claims newest-first but labelled backups sort lexically; see report")
	base := t.TempDir()
	root := filepath.Join(base, "Backups", "partition-backup")
	for _, d := range []string{"backup_pre_flash_Spacewar_V2.0-240101-0000", "backup_20260301_080000"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := ListBackups(base)
	if filepath.Base(got[0]) != "backup_20260301_080000" {
		t.Fatalf("newest backup not first: %v", got)
	}
}

func TestActionVerifyBackup(t *testing.T) {
	cases := []struct {
		name        string
		files       map[string]string // on-disk images
		checksums   string            // checksums.sha256 content
		wantResults string
		wantMarker  string
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
		},
		{
			name:        "one missing",
			files:       map[string]string{"boot_a.img": "A"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("B") + "  boot_b.img\n",
			wantResults: "Results: 1 match  /  0 changed  /  1 missing",
			wantMarker:  "not present in the backup directory",
		},
		{
			name:        "mixed",
			files:       map[string]string{"boot_a.img": "A", "dtbo_a.img": "x", "extra.img": "untracked"},
			checksums:   sum("A") + "  boot_a.img\n" + sum("D") + "  dtbo_a.img\n" + sum("V") + "  vbmeta_a.img\n",
			wantResults: "Results: 1 match  /  1 changed  /  1 missing",
			wantMarker:  "Changed files: dtbo_a",
		},
		{
			name:        "uppercase hash is a mismatch",
			files:       map[string]string{"boot_a.img": "A"},
			checksums:   strings.ToUpper(sum("A")) + "  boot_a.img\n",
			wantResults: "Results: 0 match  /  1 changed  /  0 missing",
			wantMarker:  "Changed files: boot_a",
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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

// TestActionVerifyBackupMismatchExitCode documents that verify-backup returns
// nil (exit 0) even when files are changed or missing, so scripts cannot
// detect a corrupted backup from the exit status.
func TestActionVerifyBackupMismatchExitCode(t *testing.T) {
	t.Skip("ambiguous: ActionVerifyBackup returns nil on CHANGED/MISSING; see report")
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
