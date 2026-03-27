package network

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var inetAddrRe = regexp.MustCompile(`inet (\d+\.\d+\.\d+\.\d+)/`)
var srcAddrRe  = regexp.MustCompile(`src (\d+\.\d+\.\d+\.\d+)`)

func getDeviceIP(serial string) string {
	for _, iface := range []string{"wlan0", "wlan1", "wlan2"} {
		out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("ip -f inet addr show %s 2>/dev/null", iface)})
		if m := inetAddrRe.FindStringSubmatch(out); m != nil {
			return m[1]
		}
	}
	// Fallback: ip route
	out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell", "ip route"})
	if m := srcAddrRe.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return ""
}

// ActionWifiADB switches the device to TCP/IP mode (port 5555) and connects wirelessly.
func ActionWifiADB(serial string) error {
	fmt.Println("\nDetecting device IP address...")
	ip := getDeviceIP(serial)
	if ip == "" {
		return nterrors.AdbError(
			"Could not detect device IP. Make sure Wi-Fi is connected,\n" +
				"then run manually: adb tcpip 5555 && adb connect <device_ip>:5555")
	}
	fmt.Printf("  Device IP : %s\n", ip)

	fmt.Println("Switching ADB to TCP/IP mode (port 5555)...")
	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "tcpip", "5555"})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("adb tcpip failed: %s", strings.TrimSpace(stderr)))
	}

	// Brief pause — device needs a moment to reopen the ADB daemon in TCP mode
	time.Sleep(2 * time.Second)

	target := fmt.Sprintf("%s:5555", ip)
	fmt.Printf("Connecting to %s...\n", target)
	out, _, _ := adb.Run([]string{"adb", "connect", target})
	outStr := strings.TrimSpace(out)

	if strings.Contains(strings.ToLower(outStr), "connected") {
		fmt.Printf("[OK] Wireless ADB active on %s\n", target)
		fmt.Println("     You can now disconnect the USB cable.\n")
		fmt.Printf("Reconnect later with:  adb connect %s\n", target)
		fmt.Printf("Disconnect with:       adb disconnect %s\n", target)
		return nil
	}
	return nterrors.AdbError(fmt.Sprintf(
		"Connection to %s failed: %s\nCheck that phone and PC are on the same Wi-Fi network.",
		target, outStr))
}

// ActionADBPair guides through Android 11+ wireless ADB pairing.
// port is the final connection port (default 5555).
func ActionADBPair(port int) error {
	fmt.Println("\n  Wireless ADB Pairing (Android 11+)\n")
	fmt.Println("  On your phone:")
	fmt.Println("    1. Settings -> Developer options -> Wireless debugging")
	fmt.Println("    2. Tap \"Pair device with pairing code\"")
	fmt.Println("    3. Note the IP address, pairing port, and 6-digit code shown on screen\n")

	ipAddr, err := adb.Prompt("  Enter device IP address: ")
	if err != nil {
		return nterrors.AdbError("Pairing aborted by user.")
	}
	pairingPort, err := adb.Prompt("  Enter pairing port (shown on phone): ")
	if err != nil {
		return nterrors.AdbError("Pairing aborted by user.")
	}
	pairingCode, err := adb.Prompt("  Enter 6-digit pairing code: ")
	if err != nil {
		return nterrors.AdbError("Pairing aborted by user.")
	}

	if ipAddr == "" || pairingPort == "" || pairingCode == "" {
		return nterrors.AdbError("IP address, pairing port, and pairing code are all required.")
	}

	pairTarget := fmt.Sprintf("%s:%s", ipAddr, pairingPort)
	fmt.Printf("\n  Pairing with %s...\n", pairTarget)

	out1, err1, _ := adb.Run([]string{"adb", "pair", pairTarget, pairingCode})
	combined := strings.TrimSpace(out1 + err1)

	if !strings.Contains(strings.ToLower(combined), "successfully paired") {
		return nterrors.AdbError(fmt.Sprintf(
			"Pairing failed: %s\n"+
				"Make sure the code and port match exactly what is shown on the phone.\n"+
				"The pairing code expires after a short time — try again if needed.",
			combined))
	}
	fmt.Println("[OK] Device paired!\n")

	if port == 0 {
		port = 5555
	}
	connectTarget := fmt.Sprintf("%s:%d", ipAddr, port)
	fmt.Printf("  Connecting to %s...\n", connectTarget)
	out2, err2, _ := adb.Run([]string{"adb", "connect", connectTarget})
	combined2 := strings.TrimSpace(out2 + err2)

	if strings.Contains(strings.ToLower(combined2), "connected") {
		fmt.Printf("[OK] Wireless ADB active on %s\n", connectTarget)
		fmt.Printf("Reconnect later with:  adb connect %s\n", connectTarget)
		fmt.Printf("Disconnect with:       adb disconnect %s\n", connectTarget)
		return nil
	}
	return nterrors.AdbError(fmt.Sprintf(
		"Connection to %s failed: %s\n"+
			"Pairing succeeded but connection was refused. "+
			"Check that both devices are on the same Wi-Fi network.",
		connectTarget, combined2))
}
