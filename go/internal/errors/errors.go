// Package errors defines sentinel errors and constructor functions for nothingctl.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors — use errors.Is() to test for these.
var (
	ErrAdb             = errors.New("adb error")
	ErrFirmware        = errors.New("firmware error")
	ErrFlash           = errors.New("flash error")
	ErrFastbootTimeout = errors.New("fastboot timeout")
	ErrMagisk          = errors.New("magisk error")
)

// Constructor functions that wrap sentinels for errors.Is() compatibility.

func AdbError(msg string) error        { return fmt.Errorf("%w: %s", ErrAdb, msg) }
func FirmwareError(msg string) error   { return fmt.Errorf("%w: %s", ErrFirmware, msg) }
func FlashError(msg string) error      { return fmt.Errorf("%w: %s", ErrFlash, msg) }
func FastbootTimeout(msg string) error { return fmt.Errorf("%w: %s", ErrFastbootTimeout, msg) }
func MagiskError(msg string) error     { return fmt.Errorf("%w: %s", ErrMagisk, msg) }

// IsKnownError returns true if err is one of the nothingctl-specific error types.
// Used by main() to distinguish expected errors from unexpected panics.
func IsKnownError(err error) bool {
	return errors.Is(err, ErrAdb) ||
		errors.Is(err, ErrFirmware) ||
		errors.Is(err, ErrFlash) ||
		errors.Is(err, ErrFastbootTimeout) ||
		errors.Is(err, ErrMagisk)
}
