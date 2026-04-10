package cmd

import (
	"github.com/spf13/cobra"
)

var version string

// Persistent flags shared across all subcommands
var (
	flagSerial        string
	flagBaseDir       string
	flagForceDownload bool
	flagNoBackup      bool
)

var rootCmd = &cobra.Command{
	Use:   "nothingctl",
	Short: "Nothing Phone device management CLI",
	Long: `nothingctl — manage firmware, root, backups, and settings on Nothing Phone devices.

Supports: Nothing Phone (1), (2), (2a), (3a), (3a Lite), CMF Phone (1)`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string printed by 'nothingctl version'.
func SetVersion(v string) {
	version = v
}

// GetVersion returns the current nothingctl version string.
func GetVersion() string { return version }

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagSerial, "serial", "s", "", "target a specific device by serial number")
	rootCmd.PersistentFlags().StringVar(&flagBaseDir, "base-dir", "", "override default storage root (~/.nothingctl)")
	rootCmd.PersistentFlags().BoolVar(&flagForceDownload, "force-download", false, "re-download firmware even if already cached")
	rootCmd.PersistentFlags().BoolVar(&flagNoBackup, "no-backup", false, "skip automatic backup before flashing")

	rootCmd.AddGroup(
		&cobra.Group{ID: "firmware", Title: "Firmware & Root"},
		&cobra.Group{ID: "backup", Title: "Backup & Restore"},
		&cobra.Group{ID: "magisk", Title: "Magisk & Modules"},
		&cobra.Group{ID: "apps", Title: "Apps & Debloat"},
		&cobra.Group{ID: "device", Title: "Device Info & Battery"},
		&cobra.Group{ID: "monitor", Title: "System Monitoring"},
		&cobra.Group{ID: "display", Title: "Display & Audio"},
		&cobra.Group{ID: "network", Title: "Network & Connectivity"},
		&cobra.Group{ID: "control", Title: "Input & Control"},
		&cobra.Group{ID: "nothing", Title: "Nothing-Specific"},
		&cobra.Group{ID: "util", Title: "Utility"},
	)

	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print nothingctl version",
	GroupID: "util",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("nothingctl %s\n", version)
	},
}
