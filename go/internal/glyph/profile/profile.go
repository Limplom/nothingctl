// Package profile loads and looks up Glyph device profiles from the embedded
// glyph_devices.json catalogue. Profiles describe — per device — which backend
// (sysfs/binder/HAL) drives the LEDs, the sysfs paths and tuning values, the
// addressable zones, and which high-level capabilities (on/off/blink/breath/
// dim/frame) the hardware supports.
//
// The package is purely declarative — it does not talk to a device. The
// adapter package consumes the profile to produce a working LED controller.
package profile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Limplom/nothingctl/internal/data"
)

// Backend names.
const (
	BackendSysfsNothLeds = "sysfs_noth_leds"
	BackendSysfsAW210xx  = "sysfs_aw210xx"
	BackendBinderHelper  = "binder_helper"
	BackendUnknown       = "unknown"
)

// Capability names — used with Device.Supports.
const (
	CapOn     = "on"
	CapOff    = "off"
	CapBlink  = "blink"
	CapBreath = "breath"
	CapDim    = "dim"
	CapFrame  = "frame"
)

// state_semantics values for SysfsCfg.StateSemantics.
const (
	StateBlinkPeriodMs  = "blink_period_ms"
	StateBreathPeriodMs = "breath_period_ms"
	StateNone           = "none"
)

// Device is one entry in glyph_devices.json.
type Device struct {
	Codename     string    `json:"codename"`
	Model        string    `json:"model"`
	Aliases      []string  `json:"aliases,omitempty"`
	Backend      string    `json:"backend"`
	Sysfs        *SysfsCfg `json:"sysfs,omitempty"`
	Binder       *BinderCfg `json:"binder,omitempty"`
	Zones        []Zone    `json:"zones"`
	Capabilities []string  `json:"capabilities"`
}

// SysfsCfg carries the kernel-sysfs tuning for sysfs_* backends.
type SysfsCfg struct {
	BrightnessPath     string `json:"brightness_path,omitempty"`
	StatePath          string `json:"state_path,omitempty"`
	BasePath           string `json:"base_path,omitempty"`
	MaxBrightness      int    `json:"max_brightness"`
	BrightnessIsBinary bool   `json:"brightness_is_binary"`
	StateSemantics     string `json:"state_semantics"`
	DriverRebindPath   string `json:"driver_rebind_path,omitempty"`
	DeviceID           string `json:"device_id,omitempty"`
}

// BinderCfg carries helper-DEX configuration for binder_helper backends.
type BinderCfg struct {
	HelperCmd string `json:"helper_cmd,omitempty"`
}

// Zone is one addressable LED group.
type Zone struct {
	Name      string `json:"name"`
	SysfsPath string `json:"sysfs_path,omitempty"`
	LightID   int    `json:"light_id,omitempty"`
}

// Supports returns true if the profile lists capability in its capabilities array.
func (d *Device) Supports(capability string) bool {
	for _, c := range d.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// ZoneNames returns the ordered list of zone display names.
func (d *Device) ZoneNames() []string {
	out := make([]string, 0, len(d.Zones))
	for _, z := range d.Zones {
		out = append(out, z.Name)
	}
	return out
}

// ---------------------------------------------------------------------------
// Catalogue loading + lookup
// ---------------------------------------------------------------------------

type catalogue struct {
	Devices []Device `json:"devices"`
}

var (
	once       sync.Once
	loaded     *catalogue
	loadErr    error
	matchIndex []matchEntry
)

type matchEntry struct {
	key  string // lowercased codename / model / alias
	dev  *Device
}

func load() error {
	once.Do(func() {
		var c catalogue
		if err := json.Unmarshal(data.GlyphDevicesJSON, &c); err != nil {
			loadErr = fmt.Errorf("parse glyph_devices.json: %w", err)
			return
		}
		loaded = &c

		// Build a flat match index: every codename / model / alias points at
		// the same *Device. We take addresses into the slice so callers share
		// one copy per device.
		for i := range loaded.Devices {
			d := &loaded.Devices[i]
			addKey := func(k string) {
				k = strings.ToLower(strings.TrimSpace(k))
				if k == "" {
					return
				}
				matchIndex = append(matchIndex, matchEntry{key: k, dev: d})
			}
			addKey(d.Codename)
			addKey(d.Model)
			for _, a := range d.Aliases {
				addKey(a)
			}
		}

		// Sort longest-first so "A001T" matches before "A001" etc.
		sort.Slice(matchIndex, func(i, j int) bool {
			return len(matchIndex[i].key) > len(matchIndex[j].key)
		})
	})
	return loadErr
}

// All returns every profile in catalogue order. The returned slice is shared
// backing storage — do not mutate.
func All() ([]Device, error) {
	if err := load(); err != nil {
		return nil, err
	}
	return loaded.Devices, nil
}

// Lookup finds the first device whose codename, model, or any alias is a
// substring of q (case-insensitive). Longer keys win ties, so specific
// identifiers like "A001T" are preferred over shorter prefixes.
//
// Typical callers pass the raw ro.product.model or ro.product.device string.
func Lookup(q string) (*Device, bool) {
	if err := load(); err != nil {
		return nil, false
	}
	ql := strings.ToLower(q)
	for _, e := range matchIndex {
		if strings.Contains(ql, e.key) {
			return e.dev, true
		}
	}
	return nil, false
}
