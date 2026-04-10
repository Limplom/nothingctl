package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/battery"
	"github.com/Limplom/nothingctl/internal/diagnostics"
	"github.com/Limplom/nothingctl/internal/info"
	"github.com/Limplom/nothingctl/internal/procmon"
	"github.com/Limplom/nothingctl/internal/sysmon"
	"github.com/Limplom/nothingctl/internal/thermal"
)

// ---------------------------------------------------------------------------
// System Info & Monitoring Commands
// ---------------------------------------------------------------------------

var infoCmd = &cobra.Command{
	Use:     "info",
	GroupID: "device",
	Short:   "Show full device dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagSerial == "all" {
			return runOnAllDevices(func(s string) error {
				return info.ActionInfo(s)
			})
		}
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return info.ActionInfo(serial)
	},
}

var batteryCmd = &cobra.Command{
	Use:     "battery",
	GroupID: "device",
	Short:   "Show battery health report",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagSerial == "all" {
			return runOnAllDevices(func(s string) error {
				return battery.ActionBattery(s)
			})
		}
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionBattery(serial)
	},
}

var batteryStatsCmd = &cobra.Command{
	Use:     "battery-stats",
	GroupID: "device",
	Short:   "Show per-app wakelock drain since last charge",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionBatteryStats(serial)
	},
}

var chargingControlCmd = &cobra.Command{
	Use:     "charging-control",
	GroupID: "device",
	Short:   "Set charging limit",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return battery.ActionChargingControl(serial, flagLimit)
	},
}

var thermalCmd = &cobra.Command{
	Use:     "thermal",
	GroupID: "monitor",
	Short:   "Show thermal zone temperatures",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return thermal.ActionThermal(serial, flagWatch)
	},
}

var memoryCmd = &cobra.Command{
	Use:     "memory",
	GroupID: "monitor",
	Short:   "Show RAM usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sysmon.ActionMemory(serial, flagPackage, flagWatch)
	},
}

var cpuUsageCmd = &cobra.Command{
	Use:     "cpu-usage",
	GroupID: "monitor",
	Short:   "Show CPU frequencies and top processes",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return sysmon.ActionCPUUsage(serial, flagTopN, flagWatch)
	},
}

var processTreeCmd = &cobra.Command{
	Use:     "process-tree",
	GroupID: "monitor",
	Short:   "Show running process tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionProcessTree(serial, flagPackage)
	},
}

var dozeStatusCmd = &cobra.Command{
	Use:     "doze-status",
	GroupID: "monitor",
	Short:   "Show Doze mode status and whitelist",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionDozeStatus(serial, flagWhitelistAdd, flagWhitelistRemove)
	},
}

var locationCmd = &cobra.Command{
	Use:     "location",
	GroupID: "control",
	Short:   "Show or set location mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return procmon.ActionLocation(serial, flagLocationMode)
	},
}

var logcatCmd = &cobra.Command{
	Use:     "logcat",
	GroupID: "monitor",
	Short:   "Capture logcat to file",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionLogcat(serial, flagBaseDir, flagPackage, flagTag, flagLevel, flagLines)
	},
}

var bugreportCmd = &cobra.Command{
	Use:     "bugreport",
	GroupID: "monitor",
	Short:   "Capture full bugreport",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionBugreport(serial, flagBaseDir)
	},
}

var anrDumpCmd = &cobra.Command{
	Use:     "anr-dump",
	GroupID: "monitor",
	Short:   "Collect ANR traces and tombstones",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return diagnostics.ActionANRDump(serial, flagBaseDir)
	},
}
