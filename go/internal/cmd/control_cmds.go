package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/devoptions"
	"github.com/Limplom/nothingctl/internal/inputctl"
	"github.com/Limplom/nothingctl/internal/maintenance"
	"github.com/Limplom/nothingctl/internal/notifclip"
)

// ---------------------------------------------------------------------------
// Input & Control Commands
// ---------------------------------------------------------------------------

var inputCmd = &cobra.Command{
	Use:   "input",
	Short: "Send touch, swipe, text or key input",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return inputctl.ActionInput(serial, "", flagTap, flagSwipe, flagText, flagKeyevent)
	},
}

var devOptionsCmd = &cobra.Command{
	Use:   "dev-options",
	Short: "Show or change Developer Options",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return devoptions.ActionDevOptions(serial, "", flagKey, flagValue)
	},
}

var screenAlwaysOnCmd = &cobra.Command{
	Use:   "screen-always-on",
	Short: "Keep screen on while charging",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return devoptions.ActionScreenAlwaysOn(serial, "", &flagEnable)
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "cache-clear",
	Short: "Clear app or system cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return maintenance.ActionCacheClear(serial, "", flagPackage)
	},
}

var localeCmd = &cobra.Command{
	Use:   "locale",
	Short: "Set locale, timezone or time format",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		var h24 *bool
		if cmd.Flags().Changed("24h") {
			h24 = &flagHour24
		}
		return maintenance.ActionLocale(serial, "", flagLang, flagTimezone, h24)
	},
}

var notificationsCmd = &cobra.Command{
	Use:   "notifications",
	Short: "List active notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return notifclip.ActionNotifications(serial, "", flagPackage)
	},
}

var clipboardCmd = &cobra.Command{
	Use:   "clipboard",
	Short: "Read or write clipboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return notifclip.ActionClipboard(serial, "", flagText)
	},
}
