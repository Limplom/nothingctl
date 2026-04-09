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

	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print nothingctl version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("nothingctl %s\n", version)
	},
}
