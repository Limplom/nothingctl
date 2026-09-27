package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func readHistory(t *testing.T, dir string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, historyFilename))
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	var recs []map[string]any
	if err := json.Unmarshal(raw, &recs); err != nil {
		t.Fatalf("parse history: %v", err)
	}
	return recs
}

func TestLogFlashAppends(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]any{
		{"operation": "flash-firmware", "version": "3.0-250101", "timestamp": "2026-01-01T10:00:00"},
		{"operation": "ota-update", "version": "3.1-250201", "timestamp": "2026-02-01T10:00:00"},
	}
	for _, e := range entries {
		if err := LogFlash(dir, e); err != nil {
			t.Fatalf("LogFlash: %v", err)
		}
	}
	recs := readHistory(t, dir)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	for i, e := range entries {
		if recs[i]["operation"] != e["operation"] || recs[i]["timestamp"] != e["timestamp"] {
			t.Errorf("record %d = %v, want %v", i, recs[i], e)
		}
	}
}

func TestLogFlashAddsTimestamp(t *testing.T) {
	dir := t.TempDir()
	if err := LogFlash(dir, map[string]any{"operation": "flash-firmware"}); err != nil {
		t.Fatalf("LogFlash: %v", err)
	}
	ts, ok := readHistory(t, dir)[0]["timestamp"].(string)
	if !ok {
		t.Fatal("timestamp missing or not a string")
	}
	if _, err := time.Parse("2006-01-02T15:04:05", ts); err != nil {
		t.Errorf("timestamp %q not in seconds-precision RFC3339 form: %v", ts, err)
	}
}

func TestLogFlashCorruptFileStartsFresh(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, historyFilename), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := LogFlash(dir, map[string]any{"operation": "x", "timestamp": "t"}); err != nil {
		t.Fatalf("LogFlash: %v", err)
	}
	if n := len(readHistory(t, dir)); n != 1 {
		t.Errorf("got %d records, want 1", n)
	}
}

func TestLogFlashMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	if err := LogFlash(dir, map[string]any{"operation": "x"}); err == nil {
		t.Error("expected error writing into a missing directory")
	}
}

func TestReverse(t *testing.T) {
	a, b, c := map[string]any{"i": 1}, map[string]any{"i": 2}, map[string]any{"i": 3}
	cases := []struct {
		in, want []map[string]any
	}{
		{nil, []map[string]any{}},
		{[]map[string]any{a}, []map[string]any{a}},
		{[]map[string]any{a, b, c}, []map[string]any{c, b, a}},
	}
	for _, tc := range cases {
		got := reverse(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("reverse(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
	// Input must not be mutated.
	in := []map[string]any{a, b}
	_ = reverse(in)
	if in[0]["i"] != 1 {
		t.Error("reverse mutated its input")
	}
}

func TestStringify(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "?"},
		{"abc", "abc"},
		{"", ""},
		{float64(3), "3"},
		{float64(-1), "-1"},
		{float64(1.5), "1.5"},
		{true, "true"},
		{42, "42"},
	}
	for _, tc := range cases {
		if got := stringify(tc.in); got != tc.want {
			t.Errorf("stringify(%#v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
