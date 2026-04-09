package cmd

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

// runOnAllDevices detects all connected ADB devices and calls fn(serial)
// for each one sequentially, printing a device header before each run.
// Errors are collected and reported at the end without aborting other devices.
func runOnAllDevices(fn func(serial string) error) error {
	serials, err := adb.ListDevices()
	if err != nil {
		return err
	}

	var failed []string
	for _, serial := range serials {
		fmt.Printf("\n══════════════════════════════════════\n")
		fmt.Printf("  Device: %s\n", serial)
		fmt.Printf("══════════════════════════════════════\n")
		if err := fn(serial); err != nil {
			fmt.Printf("\n[ERROR] %s: %v\n", serial, err)
			failed = append(failed, serial)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed on %d device(s): %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}
