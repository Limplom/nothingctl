package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/glyph"
	"github.com/Limplom/nothingctl/internal/nothingsettings"
)

// ---------------------------------------------------------------------------
// Nothing-Specific Commands
// ---------------------------------------------------------------------------

var glyphCmd = &cobra.Command{
	Use:   "glyph",
	Short: "Show Glyph interface status",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyph(serial, adb.Model(serial), flagValue)
	},
}

var glyphPatternCmd = &cobra.Command{
	Use:   "glyph-pattern",
	Short: "Run a Glyph light pattern",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyphPattern(serial, adb.Model(serial), flagProfile)
	},
}

var glyphNotifyCmd = &cobra.Command{
	Use:   "glyph-notify",
	Short: "Show Glyph notification settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return glyph.ActionGlyphNotify(serial, adb.Model(serial))
	},
}

var nothingSettingsCmd = &cobra.Command{
	Use:   "nothing-settings",
	Short: "Show or change Nothing-specific settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return nothingsettings.ActionNothingSettings(serial, adb.Model(serial), flagKey, flagValue)
	},
}

var essentialSpaceCmd = &cobra.Command{
	Use:   "essential-space",
	Short: "Enable or disable Essential Space",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return nothingsettings.ActionEssentialSpace(serial, adb.Model(serial), &flagEnable)
	},
}
