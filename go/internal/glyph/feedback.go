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

// feedbackZone pairs a zone name with its sysfs brightness file (relative to aw210xxBase).
type feedbackZone struct {
	name string
	file string
}

// orderedFeedbackZones maps lowercase device codename to the ordered zone list
// for the sequential-off animation (top-to-bottom visual order).
// Devices not listed here are a silent no-op.
var orderedFeedbackZones = map[string][]feedbackZone{
	// Nothing Phone (1) — confirmed live on Spacewar / A063
	"spacewar": {
		{"Camera", "rear_cam_led_br"},
		{"Diagonal", "front_cam_led_br"},
		{"Battery dot", "dot_led_br"},
		{"Battery bar", "round_leds_br"},
		{"USB", "vline_leds_br"},
	},
	"a063": {
		{"Camera", "rear_cam_led_br"},
		{"Diagonal", "front_cam_led_br"},
		{"Battery dot", "dot_led_br"},
		{"Battery bar", "round_leds_br"},
		{"USB", "vline_leds_br"},
	},
	// Phone (2), (2a), (3a), (3a Lite) — sysfs mappings not yet confirmed.
	// Add entries here once tested on real hardware.
}

// Feedback provides non-blocking Glyph LED visual feedback during a long-running
// operation. All methods are safe to call concurrently and are no-ops on
// devices with no confirmed sysfs zone map or when root is unavailable.
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
	doneCh     chan struct{}
	cancelCh   chan struct{}
	finished   chan struct{} // closed by goroutine when it exits
	doneOnce   sync.Once
	cancelOnce sync.Once
}

// NewFeedback constructs a Feedback for the given device.
// codename is ro.product.device (e.g. "spacewar", "pong").
// If the device has no confirmed zone map the returned Feedback is a silent no-op.
func NewFeedback(serial, codename string) *Feedback {
	zones := orderedFeedbackZones[strings.ToLower(codename)]
	return &Feedback{
		serial:   serial,
		zones:    zones,
		doneCh:   make(chan struct{}),
		cancelCh: make(chan struct{}),
		finished: make(chan struct{}),
	}
}

// Start lights all known zones and launches the background goroutine.
// Returns immediately — all ADB work happens in the goroutine.
// Safe to call multiple times; only the first call has effect.
func (f *Feedback) Start() {
	if len(f.zones) == 0 {
		return
	}
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

// StartWithContext calls Start and watches ctx: if the context is cancelled
// before Done() is called, Cancel() is triggered automatically.
func (f *Feedback) StartWithContext(ctx context.Context) {
	f.Start()
	if len(f.zones) == 0 {
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

// Done signals successful completion and blocks until the sequential-off
// animation finishes. Zones turn off one-by-one, top-to-bottom, with
// sequentialOffInterval between each.
func (f *Feedback) Done() {
	if len(f.zones) == 0 {
		return
	}
	f.doneOnce.Do(func() { close(f.doneCh) })
	<-f.finished
}

// Cancel signals error or cancellation, turns all zones off immediately,
// and waits for the goroutine to exit. Safe to call even if Start was never called.
func (f *Feedback) Cancel() {
	if len(f.zones) == 0 {
		return
	}
	f.cancelOnce.Do(func() { close(f.cancelCh) })
	<-f.finished
}

// writeAllBr sets all zones to the same brightness in a single ADB call,
// avoiding the per-zone round-trip cost during the pulse animation.
func (f *Feedback) writeAllBr(brightness int) {
	if len(f.zones) == 0 {
		return
	}
	var sb strings.Builder
	for _, z := range f.zones {
		fmt.Fprintf(&sb, "echo %d > %s%s; ", brightness, aw210xxBase, z.file)
	}
	adb.Run([]string{"adb", "-s", f.serial, "shell",
		fmt.Sprintf("su -c '%s'", sb.String())})
}

// writeBr writes a brightness value directly to an aw210xx sysfs file.
// Requires root on device. Errors are silently ignored.
func writeBr(serial, file string, brightness int) {
	if file == "" {
		return
	}
	adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf("su -c 'echo %d > %s%s'", brightness, aw210xxBase, file)})
}
