package glyph

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/glyph/dexhelper"
)

// deployHelper and invokeHelper are thin wrappers retained for feedback.go
// and the legacy HelperOn/HelperOff/HelperPulse API. New code should use
// internal/glyph/adapter instead.

func deployHelper(serial string) error { return dexhelper.Deploy(serial) }

func invokeHelper(serial string, args ...string) (string, string, int) {
	return dexhelper.Invoke(serial, args...)
}

// helperAvailable reports whether the embedded DEX is present (non-empty).
// Used by feedback.go to decide whether the helper path is usable at all.
func helperAvailable() bool { return dexhelper.Available() }

// HelperInfo returns the helper's device report (codename, model, zones, supported flag).
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
