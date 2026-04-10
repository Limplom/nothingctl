package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/performance"
	"github.com/Limplom/nothingctl/internal/prop"
	"github.com/Limplom/nothingctl/internal/reboot"
	"github.com/Limplom/nothingctl/internal/selfupdate"
	"github.com/Limplom/nothingctl/internal/storage"
)

// ---------------------------------------------------------------------------
// Utility Commands
// ---------------------------------------------------------------------------

var rebootCmd = &cobra.Command{
	Use:     "reboot",
	GroupID: "device",
	Short:   "Reboot to selected target",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return reboot.ActionReboot(serial, flagTarget)
	},
}

var propGetCmd = &cobra.Command{
	Use:     "prop-get",
	GroupID: "control",
	Short:   "Read system property or list all",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return prop.ActionPropGet(serial, "", flagPropKey)
	},
}

var propSetCmd = &cobra.Command{
	Use:     "prop-set",
	GroupID: "control",
	Short:   "Write system property (requires root)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return prop.ActionPropSet(serial, flagPropKey, flagPropValue)
	},
}

var performanceCmd = &cobra.Command{
	Use:     "performance",
	GroupID: "control",
	Short:   "Show or set CPU governor profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return performance.ActionPerformance(serial, flagProfile)
	},
}

var storageReportCmd = &cobra.Command{
	Use:     "storage-report",
	GroupID: "control",
	Short:   "Show storage usage report",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return storage.ActionStorageReport(serial, flagTopN)
	},
}

var apkExtractCmd = &cobra.Command{
	Use:     "apk-extract",
	GroupID: "apps",
	Short:   "Extract APK(s) from device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return storage.ActionAPKExtract(serial, flagBaseDir, flagIncludeSystem)
	},
}

var selfUpdateCmd = &cobra.Command{
	Use:     "self-update",
	GroupID: "util",
	Short:   "Check for a newer nothingctl release and replace the running binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		return selfupdate.ActionSelfUpdate(GetVersion(), flagDryRun)
	},
}
