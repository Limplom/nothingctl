package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Limplom/nothingctl/internal/adb"
	"github.com/Limplom/nothingctl/internal/network"
)

// ---------------------------------------------------------------------------
// Network & Connectivity Commands
// ---------------------------------------------------------------------------

var networkInfoCmd = &cobra.Command{
	Use:     "network-info",
	GroupID: "network",
	Short:   "Show WiFi, IP, and DNS info",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionNetworkInfo(serial, "")
	},
}

var dnsSetCmd = &cobra.Command{
	Use:     "dns-set",
	GroupID: "network",
	Short:   "Set Private DNS provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionDNSSet(serial, "", flagProvider)
	},
}

var portForwardCmd = &cobra.Command{
	Use:     "port-forward",
	GroupID: "network",
	Short:   "Set up or clear ADB port forwarding",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionPortForward(serial, "", flagLocalPort, flagRemPort, flagClear)
	},
}

var wifiScanCmd = &cobra.Command{
	Use:     "wifi-scan",
	GroupID: "network",
	Short:   "Scan for nearby WiFi networks",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiScan(serial, "")
	},
}

var wifiProfilesCmd = &cobra.Command{
	Use:     "wifi-profiles",
	GroupID: "network",
	Short:   "List saved WiFi profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiProfiles(serial, "", "")
	},
}

var forgetWifiCmd = &cobra.Command{
	Use:     "forget-wifi",
	GroupID: "network",
	Short:   "Forget a saved WiFi network",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiProfiles(serial, "", flagSSID)
	},
}

var wifiADBCmd = &cobra.Command{
	Use:     "wifi-adb",
	GroupID: "network",
	Short:   "Switch ADB to wireless TCP/IP mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		serial, err := adb.EnsureDevice(flagSerial)
		if err != nil {
			return err
		}
		return network.ActionWifiADB(serial)
	},
}

var adbPairCmd = &cobra.Command{
	Use:     "adb-pair",
	GroupID: "network",
	Short:   "Pair device for wireless ADB (Android 11+)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return network.ActionADBPair(flagPort)
	},
}
