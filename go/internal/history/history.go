// Package history records and displays the flash operation history for
// nothingctl.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

const historyFilename = "flash_history.json"

// LogFlash appends a flash event to flash_history.json in baseDir. entry is a
// free-form map; if it does not contain a "timestamp" key the current time is
// added automatically.
func LogFlash(baseDir string, entry map[string]any) error {
	historyFile := filepath.Join(baseDir, historyFilename)

	var records []map[string]any
	if raw, err := os.ReadFile(historyFile); err == nil {
		// Ignore parse errors — start fresh if the file is corrupt.
		_ = json.Unmarshal(raw, &records)
	}

	if _, ok := entry["timestamp"]; !ok {
		entry["timestamp"] = time.Now().Format(time.RFC3339)[:19] // seconds precision
	}
	records = append(records, entry)

	out, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nterrors.AdbError("marshalling flash history: " + err.Error())
	}
	if err := os.WriteFile(historyFile, out, 0o644); err != nil {
		return nterrors.AdbError("writing flash history: " + err.Error())
	}
	return nil
}

// ActionHistory prints the flash history log to stdout.
func ActionHistory(baseDir string) error {
	historyFile := filepath.Join(baseDir, historyFilename)

	raw, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("\nNo flash history yet.")
			fmt.Println("History is recorded automatically after each flash-firmware or ota-update.")
			return nil
		}
		return nterrors.AdbError("reading flash history: " + err.Error())
	}

	var records []map[string]any
	if err := json.Unmarshal(raw, &records); err != nil {
		fmt.Printf("\nCould not read flash history: %v\n", err)
		return nil
	}

	if len(records) == 0 {
		fmt.Println("\nFlash history is empty.")
		return nil
	}

	fmt.Printf("\nFlash history (%d entries)  —  %s\n\n", len(records), historyFile)
	fmt.Printf("  %-4s %-22s %-18s %-36s %-5s %s\n",
		"#", "Timestamp", "Operation", "Version", "ARB", "Serial")
	fmt.Println("  " + strings.Repeat("─", 100))

	// Print in reverse order (newest first) matching Python's reversed(records).
	for i, r := range reverse(records) {
		ts := stringify(r["timestamp"])
		if len(ts) >= 19 {
			ts = strings.Replace(ts[:19], "T", " ", 1)
		}
		op := stringify(r["operation"])
		version := stringify(r["version"])
		arb := stringify(r["arb_index"])
		serial := stringify(r["serial"])
		fmt.Printf("  %-4d %-22s %-18s %-36s %-5s %s\n",
			i, ts, op, version, arb, serial)
	}
	fmt.Println()
	return nil
}

// reverse returns a new slice with elements in reverse order.
func reverse(s []map[string]any) []map[string]any {
	out := make([]map[string]any, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return out
}

// stringify converts an any value to a string for display purposes.
func stringify(v any) string {
	if v == nil {
		return "?"
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// JSON numbers decode to float64; format without unnecessary decimals.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", v)
	}
}
