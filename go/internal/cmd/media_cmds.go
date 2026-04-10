package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/audio"
	"github.com/Limplom/nothingctl/internal/capture"
	"github.com/Limplom/nothingctl/internal/display"
)

// ---------------------------------------------------------------------------
// Display & Audio Commands
// ---------------------------------------------------------------------------

var displayCmd = &cobra.Command{
	Use:     "display",
	GroupID: "display",
	Short:   "Show or change display settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return display.ActionDisplay(serial, "", flagKey, flagValue)
	},
}

var colorProfileCmd = &cobra.Command{
	Use:     "color-profile",
	GroupID: "display",
	Short:   "Set display color profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return display.ActionColorProfile(serial, "", flagProfile)
	},
}

var audioCmd = &cobra.Command{
	Use:     "audio",
	GroupID: "display",
	Short:   "Show or adjust audio volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return audio.ActionAudio(serial, "", flagStream, flagVolume)
	},
}

var audioRouteCmd = &cobra.Command{
	Use:     "audio-route",
	GroupID: "display",
	Short:   "Show active audio routing",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return audio.ActionAudioRoute(serial, "")
	},
}

var screenshotCmd = &cobra.Command{
	Use:     "screenshot",
	GroupID: "display",
	Short:   "Take a screenshot and pull to host",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return capture.ActionScreenshot(serial, flagBaseDir)
	},
}

var screenrecordCmd = &cobra.Command{
	Use:     "screenrecord",
	GroupID: "display",
	Short:   "Record screen to video file",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return capture.ActionScreenrecord(serial, flagBaseDir, flagDuration)
	},
}
