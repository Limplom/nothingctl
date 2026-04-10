package glyph

import (
	"fmt"
	"os"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

const (
	helperDexDevice = "/data/local/tmp/glyph-helper.dex"
	helperMainClass = "com.nothingctl.GlyphHelper"
)

// deployHelper writes the embedded DEX to a host temp file and pushes it to
// the device via adb. Returns an error if the embedded DEX is empty (placeholder)
// or if the push fails.
func deployHelper(serial string) error {
	if len(glyphHelperDex) == 0 {
		return fmt.Errorf("glyph-helper DEX not embedded (placeholder build) — " +
			"download classes.dex from https://github.com/Limplom/nothingctl-glyph-helper/releases " +
			"and replace go/internal/glyph/assets/glyph-helper.dex")
	}

	tmp, err := os.CreateTemp("", "glyph-helper-*.dex")
	if err != nil {
		return fmt.Errorf("create temp DEX: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(glyphHelperDex); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp DEX: %w", err)
	}
	tmp.Close()

	_, errOut, code := adb.Run([]string{"adb", "-s", serial, "push", tmp.Name(), helperDexDevice})
	if code != 0 {
		return fmt.Errorf("adb push DEX failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return nil
}

// invokeHelper runs the glyph-helper main class via app_process with root.
// Returns stdout, stderr, and the adb exit code.
func invokeHelper(serial string, args ...string) (string, string, int) {
	shellCmd := fmt.Sprintf(
		"app_process -cp %s / %s %s",
		helperDexDevice, helperMainClass, strings.Join(args, " "),
	)
	return adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("su -c '%s'", shellCmd)})
}

// HelperInfo returns the helper's device report (codename, model, zones, supported flag).
// Returns the raw key=value output as a string.
func HelperInfo(serial string) (string, error) {
	if err := deployHelper(serial); err != nil {
		return "", err
	}
	out, errOut, code := invokeHelper(serial, "info")
	if code != 0 {
		return "", fmt.Errorf("helper info failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return strings.TrimSpace(out), nil
}

// HelperOn turns all Glyph zones on at the given brightness (0–4095).
func HelperOn(serial string, brightness int) error {
	if brightness < 0 || brightness > 4095 {
		return fmt.Errorf("brightness %d out of range 0–4095", brightness)
	}
	if err := deployHelper(serial); err != nil {
		return err
	}
	_, errOut, code := invokeHelper(serial, "on", fmt.Sprintf("%d", brightness))
	if code != 0 {
		return fmt.Errorf("helper on failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return nil
}

// HelperOff turns all Glyph zones off.
func HelperOff(serial string) error {
	if err := deployHelper(serial); err != nil {
		return err
	}
	_, errOut, code := invokeHelper(serial, "off")
	if code != 0 {
		return fmt.Errorf("helper off failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return nil
}

// HelperPulse runs one sine-curve pulse cycle (up then down).
// brightness: peak brightness 0–4095. steps: number of steps per half-cycle, 1–100.
func HelperPulse(serial string, brightness, steps int) error {
	if brightness < 0 || brightness > 4095 {
		return fmt.Errorf("brightness %d out of range 0–4095", brightness)
	}
	if steps < 1 || steps > 100 {
		return fmt.Errorf("steps %d out of range 1–100", steps)
	}
	if err := deployHelper(serial); err != nil {
		return err
	}
	_, errOut, code := invokeHelper(serial, "pulse",
		fmt.Sprintf("%d", brightness), fmt.Sprintf("%d", steps))
	if code != 0 {
		return fmt.Errorf("helper pulse failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return nil
}
