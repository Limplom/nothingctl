package adapter

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/glyph/profile"
)

// aw210xxAdapter drives the multi-zone AW210xx LED controller used on the
// original Nothing Phone (1) (spacewar / A063). Each zone has its own sysfs
// brightness file, and brightness is real 12-bit PWM (0–4095).
//
// The hardware supports no kernel-timed blink or breath — those would need
// SDK-level animations — so this adapter advertises only on/off/dim.
type aw210xxAdapter struct {
	serial string
	dev    *profile.Device
	cfg    *profile.SysfsCfg
}

func newAW210xx(serial string, dev *profile.Device) (Adapter, error) {
	if dev.Sysfs == nil {
		return nil, fmt.Errorf("aw210xx adapter: profile %q missing sysfs config", dev.Codename)
	}
	if len(dev.Zones) == 0 {
		return nil, fmt.Errorf("aw210xx adapter: profile %q has no zones", dev.Codename)
	}
	return &aw210xxAdapter{serial: serial, dev: dev, cfg: dev.Sysfs}, nil
}

func (a *aw210xxAdapter) Zones() []string                 { return a.dev.ZoneNames() }
func (a *aw210xxAdapter) Supports(capability string) bool { return a.dev.Supports(capability) }

func (a *aw210xxAdapter) On(zone string, brightness int) error {
	path, err := a.resolveZonePath(zone)
	if err != nil {
		return err
	}
	v := scaleBrightness(brightness, a.cfg.MaxBrightness, a.cfg.BrightnessIsBinary)
	return a.writeRoot(path, fmt.Sprintf("%d", v))
}

func (a *aw210xxAdapter) Off(zone string) error {
	path, err := a.resolveZonePath(zone)
	if err != nil {
		return err
	}
	return a.writeRoot(path, "0")
}

func (a *aw210xxAdapter) OffAll() error {
	var firstErr error
	for _, z := range a.dev.Zones {
		p := z.SysfsPath
		if p == "" {
			continue
		}
		if err := a.writeRoot(p, "0"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *aw210xxAdapter) Blink(zone string, periodMs int) error  { return ErrUnsupported }
func (a *aw210xxAdapter) Breath(zone string, periodMs int) error { return ErrUnsupported }

// ---------------------------------------------------------------------------

func (a *aw210xxAdapter) resolveZonePath(zone string) (string, error) {
	z := findZone(a.dev, zone)
	if z == nil {
		return "", fmt.Errorf("aw210xx: unknown zone %q on %s", zone, a.dev.Model)
	}
	if z.SysfsPath != "" {
		return z.SysfsPath, nil
	}
	if a.cfg.BasePath != "" {
		return a.cfg.BasePath + zone, nil
	}
	return "", fmt.Errorf("aw210xx: zone %q has no sysfs_path and profile has no base_path", zone)
}

func (a *aw210xxAdapter) writeRoot(path, value string) error {
	_, stderr, code := adb.Run([]string{"adb", "-s", a.serial, "shell",
		fmt.Sprintf("su -c 'echo %s > %s'", value, path)})
	if code != 0 {
		return fmt.Errorf("aw210xx write %s=%s: %s", path, value, strings.TrimSpace(stderr))
	}
	return nil
}
