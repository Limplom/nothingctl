// Package adapter provides backend-specific Glyph LED controllers behind a
// uniform interface. Each Adapter implementation knows how to talk to one
// hardware family (noth_leds sysfs, aw210xx sysfs, DEX helper Binder, …);
// For(serial, *profile.Device) selects the right one based on the profile's
// Backend field.
//
// Brightness values are always normalised 0–100 at the API boundary. Each
// adapter scales that to the hardware range declared in its profile, and
// degrades gracefully for binary-brightness devices (any non-zero → on).
package adapter

import (
	"errors"
	"fmt"

	"github.com/Limplom/nothingctl/internal/glyph/profile"
)

// ErrUnsupported is returned when an Adapter does not support a requested
// operation (e.g. Breath on a device that only blinks). Callers can check
// with errors.Is.
var ErrUnsupported = errors.New("operation not supported on this device")

// Adapter is the uniform Glyph LED interface. Brightness is 0–100.
type Adapter interface {
	// Zones returns the ordered list of addressable zone names.
	Zones() []string

	// Supports reports whether capability (profile.Cap*) is declared for the
	// device. It does not guarantee success — the kernel may still refuse —
	// but a false answer means the adapter will definitely return
	// ErrUnsupported.
	Supports(capability string) bool

	// On turns zone on at the given 0–100 brightness.
	On(zone string, brightness int) error

	// Off turns a single zone off.
	Off(zone string) error

	// OffAll turns every zone off.
	OffAll() error

	// Blink starts a hardware-timed square-wave on zone with the given period
	// in milliseconds. Stop by calling Off / OffAll.
	Blink(zone string, periodMs int) error

	// Breath starts a hardware-timed breathing cycle on zone with the given
	// period in milliseconds. Returns ErrUnsupported on hardware that only
	// blinks (e.g. galaxian).
	Breath(zone string, periodMs int) error
}

// For returns the Adapter implementation matching the device's Backend.
// Serial is the ADB device serial used for all shell calls.
func For(serial string, dev *profile.Device) (Adapter, error) {
	if dev == nil {
		return nil, fmt.Errorf("adapter.For: nil device profile")
	}
	switch dev.Backend {
	case profile.BackendSysfsNothLeds:
		return newNothLeds(serial, dev)
	case profile.BackendSysfsAW210xx:
		return newAW210xx(serial, dev)
	case profile.BackendBinderHelper:
		return newBinderHelper(serial, dev)
	case profile.BackendUnknown, "":
		return nil, fmt.Errorf("adapter.For: device %q (%s) has no verified LED backend — "+
			"hardware control unavailable until a profile is added to glyph_devices.json",
			dev.Model, dev.Codename)
	default:
		return nil, fmt.Errorf("adapter.For: unknown backend %q for device %q",
			dev.Backend, dev.Codename)
	}
}

// ---------------------------------------------------------------------------
// Helpers shared by all adapters
// ---------------------------------------------------------------------------

// scaleBrightness maps a 0–100 user value onto the device range.
// When brightnessIsBinary is true, any non-zero input returns max (on),
// zero returns 0 (off).
func scaleBrightness(pct, max int, brightnessIsBinary bool) int {
	if pct <= 0 {
		return 0
	}
	if pct > 100 {
		pct = 100
	}
	if brightnessIsBinary {
		return max
	}
	// Round half-up: (pct*max + 50) / 100
	return (pct*max + 50) / 100
}

// findZone returns the profile.Zone matching name (case-insensitive), or nil.
func findZone(dev *profile.Device, name string) *profile.Zone {
	for i := range dev.Zones {
		if equalFold(dev.Zones[i].Name, name) {
			return &dev.Zones[i]
		}
	}
	return nil
}

// equalFold is strings.EqualFold but allocation-free for our short zone names.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
