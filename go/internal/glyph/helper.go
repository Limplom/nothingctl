package glyph

import (
	"github.com/Limplom/nothingctl/internal/glyph/dexhelper"
)

// deployHelper, invokeHelper, and helperAvailable are thin wrappers around the
// dexhelper package, used by feedback.go. New code should prefer the
// internal/glyph/adapter package directly.

func deployHelper(serial string) error { return dexhelper.Deploy(serial) }

func invokeHelper(serial string, args ...string) (string, string, int) {
	return dexhelper.Invoke(serial, args...)
}

// helperAvailable reports whether the embedded DEX is present (non-empty).
// Used by feedback.go to decide whether the helper path is usable at all.
func helperAvailable() bool { return dexhelper.Available() }
