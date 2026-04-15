package adapter

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/glyph/dexhelper"
	"github.com/Limplom/nothingctl/internal/glyph/profile"
)

// binderHelperAdapter drives Glyph zones via the companion DEX helper running
// under app_process root. Used on devices where direct sysfs access is not
// viable (Phone 2 / pong, and future Binder-only devices).
//
// The helper accepts brightness in the 0–4095 range; this adapter scales
// 0–100 input to that range. Because the helper's "on"/"off"/"pulse" commands
// all address every Glyph zone at once, zone-scoped calls degrade to all-on /
// all-off.
type binderHelperAdapter struct {
	serial string
	dev    *profile.Device
}

const helperMaxBrightness = 4095

func newBinderHelper(serial string, dev *profile.Device) (Adapter, error) {
	if !dexhelper.Available() {
		return nil, fmt.Errorf("binder_helper adapter: glyph-helper DEX not embedded in this build — " +
			"either rebuild with a real DEX artifact or add a native sysfs backend for this device")
	}
	if dev.Binder == nil || dev.Binder.HelperCmd == "" {
		// Not fatal — we have a default. Log nothing; just continue.
	}
	return &binderHelperAdapter{serial: serial, dev: dev}, nil
}

func (a *binderHelperAdapter) Zones() []string                 { return a.dev.ZoneNames() }
func (a *binderHelperAdapter) Supports(capability string) bool { return a.dev.Supports(capability) }

func (a *binderHelperAdapter) On(zone string, brightness int) error {
	// The helper only exposes all-zones-on. Single-zone calls behave identically.
	v := scaleBrightness(brightness, helperMaxBrightness, false)
	if err := dexhelper.Deploy(a.serial); err != nil {
		return err
	}
	_, errOut, code := dexhelper.Invoke(a.serial, "on", fmt.Sprintf("%d", v))
	if code != 0 {
		return fmt.Errorf("binder_helper on: %s", strings.TrimSpace(errOut))
	}
	return nil
}

func (a *binderHelperAdapter) Off(zone string) error {
	// Helper "off" is all-zones; zone parameter is informational.
	return a.OffAll()
}

func (a *binderHelperAdapter) OffAll() error {
	if err := dexhelper.Deploy(a.serial); err != nil {
		return err
	}
	_, errOut, code := dexhelper.Invoke(a.serial, "off")
	if code != 0 {
		return fmt.Errorf("binder_helper off: %s", strings.TrimSpace(errOut))
	}
	return nil
}

func (a *binderHelperAdapter) Blink(zone string, periodMs int) error {
	// The current helper does not expose a hardware blink command.
	return ErrUnsupported
}

func (a *binderHelperAdapter) Breath(zone string, periodMs int) error {
	if !a.Supports(profile.CapBreath) {
		return ErrUnsupported
	}
	// The helper's "pulse" command is one sine half-cycle per invocation.
	// We can approximate a breath by passing a step count tuned to periodMs.
	// The helper maps steps 1–100; pick roughly one step per 50 ms.
	steps := periodMs / 50
	if steps < 1 {
		steps = 1
	}
	if steps > 100 {
		steps = 100
	}
	if err := dexhelper.Deploy(a.serial); err != nil {
		return err
	}
	_, errOut, code := dexhelper.Invoke(a.serial, "pulse",
		fmt.Sprintf("%d", helperMaxBrightness), fmt.Sprintf("%d", steps))
	if code != 0 {
		return fmt.Errorf("binder_helper breath: %s", strings.TrimSpace(errOut))
	}
	return nil
}
