package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/appbackup"
	"github.com/Limplom/nothingctl/internal/appmanager"
	"github.com/Limplom/nothingctl/internal/debloat"
	"github.com/Limplom/nothingctl/internal/modules"
	"github.com/Limplom/nothingctl/internal/permissions"
	"github.com/Limplom/nothingctl/internal/sideload"
)

// ---------------------------------------------------------------------------
// App Management Commands
// ---------------------------------------------------------------------------

var packageListCmd = &cobra.Command{
	Use:   "package-list",
	Short: "List installed packages",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionPackageList(serial, flagFormat)
	},
}

var appInfoCmd = &cobra.Command{
	Use:   "app-info",
	Short: "Show detailed app information",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionAppInfo(serial, flagPackage)
	},
}

var killAppCmd = &cobra.Command{
	Use:   "kill-app",
	Short: "Force-stop an app",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionKillApp(serial, flagPackage)
	},
}

var launchAppCmd = &cobra.Command{
	Use:   "launch-app",
	Short: "Launch an app or deep link",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appmanager.ActionLaunchApp(serial, flagPackage, flagDeepLink)
	},
}

var appBackupCmd = &cobra.Command{
	Use:   "app-backup",
	Short: "Backup APK and app data",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appbackup.ActionAppBackup(serial, flagBaseDir, splitCSV(flagPackages))
	},
}

var appRestoreCmd = &cobra.Command{
	Use:   "app-restore",
	Short: "Restore app backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return appbackup.ActionAppRestore(serial, flagBaseDir, splitCSV(flagPackages))
	},
}

var sideloadCmd = &cobra.Command{
	Use:   "sideload",
	Short: "Install APK or split-APK",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sideload.ActionSideload(serial, flagAPK, flagDowngrade)
	},
}

var permissionsCmd = &cobra.Command{
	Use:   "permissions",
	Short: "Audit dangerous app permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return permissions.ActionPermissions(serial, flagPackage)
	},
}

var debloatCmd = &cobra.Command{
	Use:   "debloat",
	Short: "Manage NothingOS bloatware",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		if flagInstall != "" {
			return debloat.ActionRestoreDebloat(serial, splitCSV(flagInstall))
		}
		return debloat.ActionDebloat(serial, splitCSV(flagRemove))
	},
}

var modulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List and install recommended Magisk modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModules(serial, flagBaseDir, splitCSV(flagInstall))
	},
}

var modulesStatusCmd = &cobra.Command{
	Use:   "modules-status",
	Short: "Show installed Magisk modules on device",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModulesStatus(serial)
	},
}

var modulesToggleCmd = &cobra.Command{
	Use:   "modules-toggle",
	Short: "Enable or disable Magisk modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return modules.ActionModulesToggle(serial, splitCSV(flagModuleIDs), flagEnable)
	},
}
