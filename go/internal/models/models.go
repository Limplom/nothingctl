// Package models defines shared data structures used across nothingctl packages.
package models

import "fmt"

// RootManager identifies which root manager is active on a device.
type RootManager string

const (
	RootManagerNone     RootManager = "none"
	RootManagerMagisk   RootManager = "magisk"
	RootManagerKernelSU RootManager = "kernelsu"
	RootManagerAPatch   RootManager = "apatch"
)

// MagiskStatus captures both local and remote (GitHub) Magisk state.
type MagiskStatus struct {
	AppInstalled      bool
	RootActive        bool   // /data/adb/magisk present + su works
	InstalledVersion  *int   // daemon version code (e.g. 30700)
	LatestVersion     *int   // from GitHub (e.g. 30700)
	LatestVersionStr  *string // human-readable (e.g. "30.7")
	LatestApkURL      *string
}

// IsOutdated returns true when the installed version is known and older than
// the latest GitHub release.
func (m MagiskStatus) IsOutdated() bool {
	if !m.AppInstalled || m.LatestVersion == nil {
		return false
	}
	return m.InstalledVersion != nil && *m.InstalledVersion < *m.LatestVersion
}

// StateLabel returns a human-readable summary of the Magisk installation state.
func (m MagiskStatus) StateLabel() string {
	if !m.AppInstalled {
		return "NOT INSTALLED"
	}
	if !m.RootActive {
		return "APP ONLY (boot not patched)"
	}
	if m.IsOutdated() {
		installed := 0
		if m.InstalledVersion != nil {
			installed = *m.InstalledVersion
		}
		latest := 0
		if m.LatestVersion != nil {
			latest = *m.LatestVersion
		}
		return fmt.Sprintf("ACTIVE but OUTDATED (v%d < v%d)", installed, latest)
	}
	installed := 0
	if m.InstalledVersion != nil {
		installed = *m.InstalledVersion
	}
	return fmt.Sprintf("ACTIVE  v%d", installed)
}

// BootTarget describes which boot partition image is used for Magisk patching.
type BootTarget struct {
	Filename      string // "init_boot.img" or "boot.img"
	PartitionBase string // "init_boot" or "boot"
	IsGKI2        bool   // true = GKI 2.0 device (Nothing Phone 2+)
}

// DeviceInfo holds the identifying properties of a connected Nothing device.
type DeviceInfo struct {
	Serial      string
	Model       string
	Codename    string // e.g. "Galaxian", "Spacewar", "Pong"
	CurrentSlot string // "_a" or "_b" or ""
}

// FirmwareState represents a downloaded and extracted firmware ready to flash.
type FirmwareState struct {
	ExtractedDir string
	Version      string
	IsNewer      bool
	BootTarget   BootTarget
}
