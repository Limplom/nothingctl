package glyph

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
)

// pulseSteps defines the brightness curve for one pulse cycle (0–4095 range).
// 10 steps × ~200 ms ≈ 2 s per cycle (~0.5 Hz) — visibly smooth on USB ADB.
var pulseSteps = []int{300, 800, 1500, 2200, 2800, 3000, 2800, 2200, 1500, 800}

const pulseStepInterval = 150 * time.Millisecond

const (
	feedbackBrightness    = 2000 // out of 4095 max
	sequentialOffInterval = 1 * time.Second
)

// feedbackZone pairs a zone name with its absolute sysfs brightness file path.
// maxBr is the maximum value the driver accepts (0 means the aw210xx default of 4095).
// Brightness values from pulseSteps (0–4095 range) are scaled to [0, maxBr] when maxBr > 0.
type feedbackZone struct {
	name  string
	file  string // absolute sysfs path
	maxBr int    // 0 = inherit aw210xx scale (4095), >0 = scale to this value
}

const aw210xxBase = "/sys/class/leds/aw210xx_led/"

// orderedFeedbackZones maps lowercase device codename to the ordered zone list
// for the sequential-off animation (top-to-bottom visual order).
// Devices not listed here are a silent no-op.
var orderedFeedbackZones = map[string][]feedbackZone{
	// Nothing Phone (1) — confirmed live on Spacewar / A063
	"spacewar": {
		{"Camera",      aw210xxBase + "rear_cam_led_br",  0},
		{"Diagonal",    aw210xxBase + "front_cam_led_br", 0},
		{"Battery dot", aw210xxBase + "dot_led_br",       0},
		{"Battery bar", aw210xxBase + "round_leds_br",    0},
		{"USB",         aw210xxBase + "vline_leds_br",    0},
	},
	"a063": {
		{"Camera",      aw210xxBase + "rear_cam_led_br",  0},
		{"Diagonal",    aw210xxBase + "front_cam_led_br", 0},
		{"Battery dot", aw210xxBase + "dot_led_br",       0},
		{"Battery bar", aw210xxBase + "round_leds_br",    0},
		{"USB",         aw210xxBase + "vline_leds_br",    0},
	},
	// Nothing Phone (3a Lite / galaxian) — confirmed on A001T hardware.
	// Single noth_leds node controls all zones; max brightness is 255.
	"galaxian": {
		{"All zones", "/sys/class/leds/noth_leds/brightness", 255},
	},
	// Phone (2), (2a), (3a) — sysfs mappings not yet confirmed.
	// Add entries here once tested on real hardware.
}

// Feedback provides non-blocking Glyph LED visual feedback during a long-running
// operation. All methods are safe to call concurrently and are no-ops on
// devices with no confirmed sysfs zone map and no helper support, or when root
// is unavailable.
//
// For Phone 1 (spacewar/a063), brightness is written directly to the aw210xx
// sysfs driver. For Phone 2 and newer, the glyph-helper DEX is deployed and
// invoked via app_process (requires the embedded DEX to be present).
//
// Typical usage:
//
//	fb := glyph.NewFeedback(serial, codename)
//	fb.StartWithContext(ctx)
//	defer fb.Cancel()
//	err := doWork(ctx)
//	if err == nil {
//	    fb.Done()
//	}
type Feedback struct {
	serial     string
	zones      []feedbackZone
	useHelper  bool // true when sysfs zones are absent and helper DEX is embedded
	doneCh     chan struct{}
	cancelCh   chan struct{}
	finished   chan struct{} // closed by goroutine when it exits
	startOnce  sync.Once   // prevents double-launch panic on close(finished)
	doneOnce   sync.Once
	cancelOnce sync.Once
}

// NewFeedback constructs a Feedback for the given device.
// codename is ro.product.device (e.g. "spacewar", "pong").
// Devices with direct sysfs zone maps use them; all others fall back to the
// glyph-helper DEX if it is embedded (non-empty placeholder).
func NewFeedback(serial, codename string) *Feedback {
	zones := orderedFeedbackZones[strings.ToLower(codename)]
	// Fall back to helper for devices that have Glyph zones but no direct sysfs access.
	useHelper := len(zones) == 0 && len(glyphHelperDex) > 0
	return &Feedback{
		serial:    serial,
		zones:     zones,
		useHelper: useHelper,
		doneCh:    make(chan struct{}),
		cancelCh:  make(chan struct{}),
		finished:  make(chan struct{}),
	}
}

// active reports whether this Feedback instance has any LED path available.
func (f *Feedback) active() bool { return len(f.zones) > 0 || f.useHelper }

// Start lights all known zones and launches the background goroutine.
// Returns immediately — all ADB work happens in the goroutine.
// Safe to call multiple times; only the first call has effect.
func (f *Feedback) Start() {
	if !f.active() {
		return
	}
	f.startOnce.Do(func() {
		if f.useHelper {
			f.startHelper()
			return
		}
		f.startSysfs()
	})
}

func (f *Feedback) startSysfs() {
	go func() {
		defer close(f.finished)
		// Pulse loop: cycle through brightness steps until signalled.
		i := 0
	pulse:
		for {
			select {
			case <-f.doneCh:
				break pulse
			case <-f.cancelCh:
				f.writeAllBr(0)
				return
			default:
				f.writeAllBr(pulseSteps[i%len(pulseSteps)])
				i++
				time.Sleep(pulseStepInterval)
			}
		}
		// Sequential off: top-to-bottom, 1 s apart.
		for _, z := range f.zones {
			writeBr(f.serial, z.file, 0)
			time.Sleep(sequentialOffInterval)
		}
	}()
}

// startHelper deploys the embedded DEX and runs pulse cycles via the glyph-helper
// until Done() or Cancel() is called. Each invokeHelper("pulse") call blocks for
// approximately steps * 2 * 150 ms (≈ 3 s for default 10 steps), so cancellation
// has up to one pulse cycle of lag.
func (f *Feedback) startHelper() {
	go func() {
		defer close(f.finished)

		if err := deployHelper(f.serial); err != nil {
			// Silent no-op: helper unavailable (e.g. placeholder build or push failed).
			return
		}

		for {
			select {
			case <-f.doneCh:
				invokeHelper(f.serial, "off") // best-effort; ignore exit code on shutdown
				return
			case <-f.cancelCh:
				invokeHelper(f.serial, "off") // best-effort; ignore exit code on shutdown
				return
			default:
				// One pulse cycle: sine ramp up then down (~3 s).
				invokeHelper(f.serial, "pulse", fmt.Sprintf("%d", feedbackBrightness), "10")
			}
		}
	}()
}

// StartWithContext calls Start and watches ctx: if the context is cancelled
// before Done() is called, Cancel() is triggered automatically.
func (f *Feedback) StartWithContext(ctx context.Context) {
	f.Start()
	if !f.active() {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
			f.Cancel()
		case <-f.doneCh:
		case <-f.cancelCh:
		}
	}()
}

// Done signals successful completion and blocks until the goroutine exits.
// For sysfs-based devices, zones turn off one-by-one top-to-bottom.
// For helper-based devices, zones turn off immediately via the helper.
func (f *Feedback) Done() {
	if !f.active() {
		return
	}
	f.doneOnce.Do(func() { close(f.doneCh) })
	<-f.finished
}

// Cancel signals error or cancellation, turns all zones off immediately,
// and waits for the goroutine to exit. Safe to call even if Start was never called.
func (f *Feedback) Cancel() {
	if !f.active() {
		return
	}
	f.cancelOnce.Do(func() { close(f.cancelCh) })
	<-f.finished
}

// scaleBr scales a brightness value from the 0–4095 range to [0, maxBr].
// If maxBr is 0 the raw value is returned unchanged (aw210xx devices accept 0–4095).
func scaleBr(br, maxBr int) int {
	if maxBr <= 0 {
		return br
	}
	return br * maxBr / 4095
}

// writeAllBr sets all zones to the same brightness in a single ADB call,
// avoiding the per-zone round-trip cost during the pulse animation.
func (f *Feedback) writeAllBr(brightness int) {
	if len(f.zones) == 0 {
		return
	}
	var sb strings.Builder
	for _, z := range f.zones {
		fmt.Fprintf(&sb, "echo %d > %s; ", scaleBr(brightness, z.maxBr), z.file)
	}
	adb.Run([]string{"adb", "-s", f.serial, "shell",
		fmt.Sprintf("su -c '%s'", sb.String())})
}

// writeBr writes a brightness value to an absolute sysfs path.
// Requires root on device. Errors are silently ignored.
func writeBr(serial, file string, brightness int) {
	if file == "" {
		return
	}
	adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("su -c 'echo %d > %s'", brightness, file)})
}
