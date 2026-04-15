// Package dexhelper deploys and invokes the compiled glyph-helper DEX on a
// connected device. The helper DEX lives in the companion repo
// https://github.com/Limplom/nothingctl-glyph-helper and is embedded at build
// time from assets/glyph-helper.dex.
//
// This package has no dependency on the rest of the glyph package — both the
// legacy helper wrappers (glyph.HelperOn/HelperOff/…) and the new binder
// adapter (glyph/adapter) call into it.
package dexhelper

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
)

// devicePath is where the DEX is pushed on the device.
const devicePath = "/data/local/tmp/glyph-helper.dex"

// MainClass is the com.nothingctl entry class inside the DEX.
const MainClass = "com.nothingctl.GlyphHelper"

// embeddedDex is the compiled helper. Empty when built without the asset — in
// that case Available() returns false and Deploy() returns a descriptive error.
//
//go:embed assets/glyph-helper.dex
var embeddedDex []byte

// Available reports whether a real DEX was embedded at build time.
func Available() bool { return len(embeddedDex) > 0 }

// Deploy pushes the embedded DEX to the device. No-op after the first call
// per process — the DEX changes only across builds.
func Deploy(serial string) error {
	if !Available() {
		return fmt.Errorf("glyph-helper DEX not embedded (placeholder build) — " +
			"download classes.dex from https://github.com/Limplom/nothingctl-glyph-helper/releases " +
			"and replace go/internal/glyph/dexhelper/assets/glyph-helper.dex")
	}

	tmp, err := os.CreateTemp("", "glyph-helper-*.dex")
	if err != nil {
		return fmt.Errorf("create temp DEX: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(embeddedDex); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp DEX: %w", err)
	}
	tmp.Close()

	_, errOut, code := adb.Run([]string{"adb", "-s", serial, "push", tmp.Name(), devicePath})
	if code != 0 {
		return fmt.Errorf("adb push DEX failed (exit %d): %s", code, strings.TrimSpace(errOut))
	}
	return nil
}

// Invoke runs the helper main class via app_process under root with the given
// argv. Returns stdout, stderr, and the adb exit code.
func Invoke(serial string, args ...string) (string, string, int) {
	shellCmd := fmt.Sprintf("app_process -cp %s / %s %s",
		devicePath, MainClass, strings.Join(args, " "))
	return adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("su -c '%s'", shellCmd)})
}

// DevicePath returns the on-device DEX path (exposed for diagnostics).
func DevicePath() string { return devicePath }
