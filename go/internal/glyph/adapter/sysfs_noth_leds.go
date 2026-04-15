package adapter

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/glyph/profile"
)

// nothLedsAdapter drives the single-zone Nothing "noth_leds" kernel driver
// used on galaxian (Phone 3a Lite) and potentially future breathing-lights
// SoCs.
//
// Hardware characteristics verified on galaxian:
//   - state is the MASTER ENABLE. state=0 forces the LED off regardless of
//     brightness; state=1 is "solid on"; state=N>1 is a hardware square-wave
//     blink with period N ms. brightness alone does nothing.
//   - write order matters: state must be set BEFORE brightness, otherwise the
//     driver silently ignores the brightness write.
//   - brightness is binary (0 = off, anything non-zero = full on) — no PWM
//     granularity despite max_brightness reporting 255.
//   - writing anything to /sys/class/leds/noth_leds/trigger destroys the state
//     attribute — we never touch trigger.
//   - if state vanishes, rebinding the platform driver restores it.
type nothLedsAdapter struct {
	serial string
	dev    *profile.Device
	cfg    *profile.SysfsCfg
}

func newNothLeds(serial string, dev *profile.Device) (Adapter, error) {
	if dev.Sysfs == nil {
		return nil, fmt.Errorf("noth_leds adapter: profile %q missing sysfs config", dev.Codename)
	}
	if dev.Sysfs.BrightnessPath == "" {
		return nil, fmt.Errorf("noth_leds adapter: profile %q missing sysfs.brightness_path", dev.Codename)
	}
	return &nothLedsAdapter{serial: serial, dev: dev, cfg: dev.Sysfs}, nil
}

func (a *nothLedsAdapter) Zones() []string                   { return a.dev.ZoneNames() }
func (a *nothLedsAdapter) Supports(capability string) bool   { return a.dev.Supports(capability) }

func (a *nothLedsAdapter) On(zone string, brightness int) error {
	if _, err := a.resolveZonePath(zone); err != nil {
		return err
	}
	v := scaleBrightness(brightness, a.cfg.MaxBrightness, a.cfg.BrightnessIsBinary)
	if v == 0 {
		return a.Off(zone)
	}
	// state is the master enable on this driver — write state=1 (solid on)
	// BEFORE brightness, otherwise the driver ignores the brightness write.
	if a.cfg.StatePath != "" {
		if err := a.ensureStateAttr(); err != nil {
			return err
		}
		if err := a.writeRoot(a.cfg.StatePath, "1"); err != nil {
			return err
		}
	}
	return a.writeRoot(a.cfg.BrightnessPath, fmt.Sprintf("%d", v))
}

func (a *nothLedsAdapter) Off(zone string) error {
	if _, err := a.resolveZonePath(zone); err != nil {
		return err
	}
	// Disable the master enable first so any running blink stops cleanly,
	// then clear brightness for a known-good idle state.
	if a.cfg.StatePath != "" {
		_ = a.writeRoot(a.cfg.StatePath, "0")
	}
	return a.writeRoot(a.cfg.BrightnessPath, "0")
}

func (a *nothLedsAdapter) OffAll() error {
	// Single-zone device — delegate to the only zone.
	if len(a.dev.Zones) == 0 {
		return a.Off("")
	}
	return a.Off(a.dev.Zones[0].Name)
}

func (a *nothLedsAdapter) Blink(zone string, periodMs int) error {
	if !a.Supports(profile.CapBlink) {
		return ErrUnsupported
	}
	if a.cfg.StatePath == "" || a.cfg.StateSemantics != profile.StateBlinkPeriodMs {
		return ErrUnsupported
	}
	if periodMs < 1 {
		periodMs = 1
	}
	if err := a.ensureStateAttr(); err != nil {
		return err
	}
	// Brightness must be non-zero before state, otherwise the driver ignores it.
	if err := a.writeRoot(a.cfg.BrightnessPath,
		fmt.Sprintf("%d", scaleBrightness(100, a.cfg.MaxBrightness, a.cfg.BrightnessIsBinary))); err != nil {
		return err
	}
	return a.writeRoot(a.cfg.StatePath, fmt.Sprintf("%d", periodMs))
}

func (a *nothLedsAdapter) Breath(zone string, periodMs int) error {
	// noth_leds on galaxian produces a square wave, not a sine — no breathing.
	if !a.Supports(profile.CapBreath) {
		return ErrUnsupported
	}
	return ErrUnsupported
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveZonePath returns the sysfs brightness path for the given zone name.
// On single-zone devices an empty zone name (or the profile's only zone) both
// resolve to the profile's top-level BrightnessPath.
func (a *nothLedsAdapter) resolveZonePath(zone string) (string, error) {
	if zone == "" {
		return a.cfg.BrightnessPath, nil
	}
	z := findZone(a.dev, zone)
	if z == nil {
		return "", fmt.Errorf("noth_leds: unknown zone %q on %s", zone, a.dev.Model)
	}
	if z.SysfsPath != "" {
		return z.SysfsPath, nil
	}
	return a.cfg.BrightnessPath, nil
}

// writeRoot writes value into the sysfs path via `su -c`. Returns an error
// if the command exits non-zero.
func (a *nothLedsAdapter) writeRoot(path, value string) error {
	_, stderr, code := adb.Run([]string{"adb", "-s", a.serial, "shell",
		fmt.Sprintf("su -c 'echo %s > %s'", value, path)})
	if code != 0 {
		return fmt.Errorf("noth_leds write %s=%s: %s", path, value, strings.TrimSpace(stderr))
	}
	return nil
}

// ensureStateAttr verifies the state sysfs file exists, and rebinds the
// platform driver if it doesn't. Necessary because some operations (or third-
// party writes to trigger) can unbind the state attribute.
func (a *nothLedsAdapter) ensureStateAttr() error {
	if a.cfg.StatePath == "" {
		return nil
	}
	exists := adb.ShellStr(a.serial,
		fmt.Sprintf("su -c 'test -e %s && echo yes || echo no'", a.cfg.StatePath))
	if exists == "yes" {
		return nil
	}
	if a.cfg.DriverRebindPath == "" || a.cfg.DeviceID == "" {
		return fmt.Errorf("noth_leds: state path %s missing and driver_rebind_path / device_id not configured",
			a.cfg.StatePath)
	}
	// Rebind: unbind then bind the platform device.
	_, _, _ = adb.Run([]string{"adb", "-s", a.serial, "shell",
		fmt.Sprintf("su -c 'echo %s > %s/unbind'", a.cfg.DeviceID, a.cfg.DriverRebindPath)})
	_, _, code := adb.Run([]string{"adb", "-s", a.serial, "shell",
		fmt.Sprintf("su -c 'echo %s > %s/bind'", a.cfg.DeviceID, a.cfg.DriverRebindPath)})
	if code != 0 {
		return fmt.Errorf("noth_leds: driver rebind failed")
	}
	// Verify the state file re-appeared.
	check := adb.ShellStr(a.serial,
		fmt.Sprintf("su -c 'test -e %s && echo yes || echo no'", a.cfg.StatePath))
	if check != "yes" {
		return fmt.Errorf("noth_leds: rebind completed but %s still missing", a.cfg.StatePath)
	}
	return nil
}
