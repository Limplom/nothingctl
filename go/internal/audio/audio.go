// Package audio provides audio volume and routing management for Nothing phones.
package audio

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

type stream struct {
	id   int
	name string
}

var streams = []stream{
	{0, "Voice Call"},
	{1, "System"},
	{2, "Ring"},
	{3, "Media"},
	{4, "Alarm"},
	{5, "Notification"},
}

var streamAliases = map[string]int{
	"voice": 0, "call": 0,
	"system":       1,
	"ring":         2,
	"media":        3, "music": 3,
	"alarm":        4,
	"notification": 5, "notify": 5,
}

var deviceLabels = map[string]string{
	"AUDIO_DEVICE_OUT_SPEAKER":                   "Speaker",
	"AUDIO_DEVICE_OUT_WIRED_HEADPHONE":           "Wired Headphones",
	"AUDIO_DEVICE_OUT_WIRED_HEADSET":             "Wired Headset",
	"AUDIO_DEVICE_OUT_BLUETOOTH_A2DP":            "Bluetooth A2DP",
	"AUDIO_DEVICE_OUT_BLUETOOTH_A2DP_HEADPHONES": "Bluetooth Headphones",
	"AUDIO_DEVICE_OUT_BLUETOOTH_A2DP_SPEAKER":    "Bluetooth Speaker",
	"AUDIO_DEVICE_OUT_BLUETOOTH_SCO":             "Bluetooth SCO",
	"AUDIO_DEVICE_OUT_BLUETOOTH_SCO_HEADSET":     "Bluetooth SCO Headset",
	"AUDIO_DEVICE_OUT_USB_HEADSET":               "USB Headset",
	"AUDIO_DEVICE_OUT_USB_DEVICE":                "USB Audio",
	"AUDIO_DEVICE_OUT_EARPIECE":                  "Earpiece",
	"AUDIO_DEVICE_OUT_HDMI":                      "HDMI",
}

const barWidth = 20

func resolveStream(s string) (int, error) {
	var sid int
	if _, err := fmt.Sscanf(s, "%d", &sid); err == nil {
		for _, st := range streams {
			if st.id == sid {
				return sid, nil
			}
		}
		return 0, nterrors.AdbError(fmt.Sprintf("Unknown stream ID %d. Valid IDs: 0-5", sid))
	}
	if id, ok := streamAliases[strings.ToLower(s)]; ok {
		return id, nil
	}
	return 0, nterrors.AdbError(fmt.Sprintf("Unknown stream '%s'.", s))
}

var volRe = regexp.MustCompile(`volume is\s+(\d+)\s+in range\s+\[(\d+)\.\.(\d+)\]`)

func getStreamVolume(serial string, streamID int) (int, int, error) {
	output := adb.ShellStr(serial, fmt.Sprintf("cmd media_session volume --stream %d --get", streamID))
	if m := volRe.FindStringSubmatch(output); m != nil {
		var cur, max int
		fmt.Sscanf(m[1], "%d", &cur)
		fmt.Sscanf(m[3], "%d", &max)
		return cur, max, nil
	}
	return 0, 0, nterrors.AdbError(fmt.Sprintf("Could not parse volume output for stream %d: %s", streamID, output))
}

func bar(current, maximum int) string {
	if maximum <= 0 {
		return strings.Repeat(" ", barWidth)
	}
	filled := int(float64(current)/float64(maximum)*float64(barWidth) + 0.5)
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat(" ", barWidth-filled)
}

// ActionAudio reads or sets audio stream volumes.
func ActionAudio(serial, model, streamName string, volume int) error {
	if streamName != "" && volume >= 0 {
		streamID, err := resolveStream(streamName)
		if err != nil {
			return err
		}
		streamDisplayName := ""
		for _, st := range streams {
			if st.id == streamID {
				streamDisplayName = st.name
				break
			}
		}
		adb.Run([]string{"adb", "-s", serial, "shell",
			"cmd", "media_session", "volume",
			"--stream", fmt.Sprintf("%d", streamID),
			"--set", fmt.Sprintf("%d", volume)})
		cur, max, err := getStreamVolume(serial, streamID)
		if err == nil {
			fmt.Printf("\n  %s volume set to %d/%d on %s\n\n", streamDisplayName, cur, max, model)
		} else {
			fmt.Printf("\n  %s volume set to %d on %s\n\n", streamDisplayName, volume, model)
		}
		return nil
	}

	if (streamName == "") != (volume < 0) {
		return nterrors.AdbError("Provide both --stream and --volume to set volume, or neither to read all.")
	}

	fmt.Printf("\n  Audio Volumes \u2014 %s\n\n", model)
	fmt.Printf("  %-16s %5s  %5s   %s\n", "Stream", "Vol", "Max", "Bar")
	fmt.Println("  " + strings.Repeat("\u2500", 16+5+5+barWidth+12))

	for _, st := range streams {
		cur, max, err := getStreamVolume(serial, st.id)
		if err != nil {
			cur, max = 0, 0
		}
		barStr := bar(cur, max)
		fmt.Printf("  %-16s %5d  %5d   [%s]\n", st.name, cur, max, barStr)
	}
	fmt.Println()
	return nil
}

var sysfsDeviceLabels = map[string]string{
	"speaker":         "Speaker",
	"earpiece":        "Earpiece",
	"bt_a2dp":         "Bluetooth A2DP",
	"bt_sco":          "Bluetooth SCO",
	"usb_headset":     "USB Headset",
	"wired_headphone": "Wired Headphones",
	"wired_headset":   "Wired Headset",
	"hdmi":            "HDMI",
	"ble_headset":     "Bluetooth LE Headset",
}

var devicesRe = regexp.MustCompile(`Devices:\s*(\w+)\(`)
var outputDeviceRe = regexp.MustCompile(`Output device:\s*(AUDIO_DEVICE_OUT_\w+)`)
var anyDeviceRe = regexp.MustCompile(`(AUDIO_DEVICE_OUT_\w+)`)
var btNameRe = regexp.MustCompile(`name:\s*(.+)`)
var connStateRe = regexp.MustCompile(`connectionState:\s*2`)

// ActionAudioRoute shows active audio output path and connected Bluetooth audio devices.
func ActionAudioRoute(serial, model string) error {
	audioDump := adb.ShellStr(serial, "dumpsys audio")

	activeOutput := "Unknown"

	// Strategy 1: Music stream device (stream index 4)
	inStreamSection := false
	streamIdx := -1
	for _, line := range strings.Split(audioDump, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.Contains(line, "Stream volumes") {
			inStreamSection = true
			streamIdx = 0
			continue
		}
		if inStreamSection {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "Current:") {
				streamIdx++
			} else if strings.HasPrefix(stripped, "Devices:") {
				if streamIdx == 4 {
					if m := devicesRe.FindStringSubmatch(line); m != nil {
						dev := strings.ToLower(m[1])
						if label, ok := sysfsDeviceLabels[dev]; ok {
							activeOutput = label
						} else {
							activeOutput = dev
						}
					}
					break
				}
			} else if stripped == "" && streamIdx > 6 {
				break
			}
		}
	}

	// Strategy 2: explicit "Output device:"
	if activeOutput == "Unknown" {
		for _, line := range strings.Split(audioDump, "\n") {
			line = strings.TrimRight(line, "\r")
			if m := outputDeviceRe.FindStringSubmatch(line); m != nil {
				raw := m[1]
				if label, ok := deviceLabels[raw]; ok {
					activeOutput = label
				} else {
					activeOutput = raw
				}
				break
			}
		}
	}

	// Strategy 3: any AUDIO_DEVICE_OUT_
	if activeOutput == "Unknown" {
		for _, line := range strings.Split(audioDump, "\n") {
			line = strings.TrimRight(line, "\r")
			if m := anyDeviceRe.FindStringSubmatch(line); m != nil {
				raw := m[1]
				if label, ok := deviceLabels[raw]; ok {
					activeOutput = label
				} else {
					activeOutput = raw
				}
				if raw != "AUDIO_DEVICE_OUT_SPEAKER" {
					break
				}
			}
		}
	}

	// Bluetooth devices
	btDump := adb.ShellStr(serial, "dumpsys bluetooth_manager")
	type btDevice struct {
		name, profile, state string
	}
	var btDevices []btDevice
	seen := make(map[string]bool)

	lines := strings.Split(btDump, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if m := btNameRe.FindStringSubmatch(line); m != nil {
			candidateName := strings.TrimSpace(m[1])
			if candidateName == "" || strings.ToLower(candidateName) == "null" {
				continue
			}
			end := i + 10
			if end > len(lines) {
				end = len(lines)
			}
			window := strings.Join(lines[i:end], "\n")
			if !connStateRe.MatchString(window) {
				continue
			}
			profile := "\u2014"
			start := i - 5
			if start < 0 {
				start = 0
			}
			for _, bl := range lines[start:i] {
				if strings.Contains(bl, "A2DP") {
					profile = "A2DP"
					break
				}
				if strings.Contains(bl, "HFP") || strings.Contains(bl, "HeadsetService") {
					profile = "HFP"
					break
				}
				if strings.Contains(bl, "HID") {
					profile = "HID"
					break
				}
				if strings.Contains(bl, "LE") {
					profile = "BLE"
					break
				}
			}
			key := candidateName + "|" + profile
			if !seen[key] {
				seen[key] = true
				btDevices = append(btDevices, btDevice{candidateName, profile, "Connected"})
			}
		}
	}

	fmt.Printf("\n  Audio Route \u2014 %s\n\n", model)
	fmt.Printf("  Active Output   : %s\n\n", activeOutput)
	fmt.Println("  Bluetooth Devices:")

	if len(btDevices) > 0 {
		nameW := 18
		for _, d := range btDevices {
			if len(d.name) > nameW {
				nameW = len(d.name)
			}
		}
		for _, d := range btDevices {
			fmt.Printf("    %-*s  %-6s  %s\n", nameW, d.name, d.profile, d.state)
		}
	} else {
		fmt.Println("    (none)")
	}
	fmt.Println()
	return nil
}
