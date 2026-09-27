package arb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// makeVbmeta builds a synthetic AVB vbmeta header with the given magic and
// rollback index, padded/truncated to size bytes.
func makeVbmeta(magic string, rollback uint64, size int) []byte {
	buf := make([]byte, 256)
	copy(buf, magic)
	binary.BigEndian.PutUint32(buf[4:], 1) // libavb major
	binary.BigEndian.PutUint64(buf[arbOffset:], rollback)
	binary.BigEndian.PutUint32(buf[120:], 0) // flags
	binary.BigEndian.PutUint32(buf[124:], 0) // rollback_index_location
	if size <= len(buf) {
		return buf[:size]
	}
	return append(buf, make([]byte, size-len(buf))...)
}

func writeFile(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "vbmeta.img")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseVbmeta(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		want    uint64
		wantErr bool
	}{
		{"zero index", makeVbmeta("AVB0", 0, 4096), 0, false},
		{"small index", makeVbmeta("AVB0", 3, 4096), 3, false},
		{"large index", makeVbmeta("AVB0", 1_700_000_000, 4096), 1_700_000_000, false},
		{"max uint64", makeVbmeta("AVB0", ^uint64(0), 4096), ^uint64(0), false},
		// Exactly the bytes needed (header up to and including rollback_index).
		{"exact minimum length", makeVbmeta("AVB0", 7, headerReadLen), 7, false},
		{"one byte too short", makeVbmeta("AVB0", 7, headerReadLen-1), 0, true},
		{"empty file", []byte{}, 0, true},
		{"bad magic", makeVbmeta("AVB1", 5, 4096), 0, true},
		{"lowercase magic", makeVbmeta("avb0", 5, 4096), 0, true},
		{"zero magic", makeVbmeta("\x00\x00\x00\x00", 5, 4096), 0, true},
		// Android boot image magic — a wrong file passed as vbmeta.
		{"boot image not vbmeta", append([]byte("ANDROID!"), make([]byte, 4096)...), 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseVbmeta(writeFile(t, c.data))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

// TestParseVbmetaBigEndian makes sure the index is decoded big-endian (AVB
// spec). A little-endian decode of 1 would yield 1<<56 and wrongly let a
// downgrade through the ARB check.
func TestParseVbmetaBigEndian(t *testing.T) {
	data := makeVbmeta("AVB0", 0, 4096)
	data[arbOffset+7] = 1
	got, err := ParseVbmeta(writeFile(t, data))
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
}

// TestParseVbmetaIgnoresNeighbours ensures fields adjacent to rollback_index
// (descriptor_size before, flags after) do not bleed into the result.
func TestParseVbmetaIgnoresNeighbours(t *testing.T) {
	data := makeVbmeta("AVB0", 42, 4096)
	for i := 104; i < arbOffset; i++ {
		data[i] = 0xFF
	}
	for i := arbOffset + 8; i < 128; i++ {
		data[i] = 0xFF
	}
	got, err := ParseVbmeta(writeFile(t, data))
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

func TestParseVbmetaMissingFile(t *testing.T) {
	if _, err := ParseVbmeta(filepath.Join(t.TempDir(), "missing.img")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParseVbmetaDirectory(t *testing.T) {
	if _, err := ParseVbmeta(t.TempDir()); err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

// TestCheckARBSkipsWithoutVbmeta verifies CheckARB never blocks (and never
// touches the device) when the firmware package lacks vbmeta.img or it is
// unparseable. Both branches return before any adb call.
func TestCheckARBSkipsWithoutVbmeta(t *testing.T) {
	dir := t.TempDir()
	if err := CheckARB("no-such-serial", dir); err != nil {
		t.Fatalf("missing vbmeta: got %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vbmeta.img"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckARB("no-such-serial", dir); err != nil {
		t.Fatalf("garbage vbmeta: got %v, want nil", err)
	}
}
