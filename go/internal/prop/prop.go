// Package prop provides Android system property read and write.
package prop

import (
	"fmt"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

var prefixGroups = []string{
	"ro.product",
	"ro.build",
	"ro.boot",
	"ro.hardware",
	"persist",
	"sys",
	"gsm",
	"net",
	"wifi",
}

func groupKey(propName string) string {
	for _, prefix := range prefixGroups {
		if strings.HasPrefix(propName, prefix) {
			return prefix
		}
	}
	return "other"
}

func parseGetprop(output string) [][2]string {
	var props [][2]string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "[") {
			continue
		}
		idx := strings.Index(line, "]:")
		if idx == -1 {
			continue
		}
		key := line[1:idx]
		rest := strings.TrimSpace(line[idx+2:])
		value := rest
		if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
			value = rest[1 : len(rest)-1]
		}
		if key != "" {
			props = append(props, [2]string{key, value})
		}
	}
	return props
}

// ActionPropGet reads one or all system properties.
// key="" means dump all grouped by prefix.
func ActionPropGet(serial, model, key string) error {
	if key != "" {
		out, _, _ := adb.Run([]string{"adb", "-s", serial, "shell",
			fmt.Sprintf("getprop %s", key)})
		val := strings.TrimSpace(out)
		if val == "" {
			fmt.Printf("  [WARN] Property '%s' is empty or not set.\n", key)
		} else {
			fmt.Printf("  %s = %s\n", key, val)
		}
		return nil
	}

	stdout, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell", "getprop"})
	if code != 0 {
		return nterrors.AdbError(fmt.Sprintf("getprop failed: %s", strings.TrimSpace(stderr)))
	}

	props := parseGetprop(stdout)
	if len(props) == 0 {
		return nterrors.AdbError("getprop returned no output.")
	}

	// Group by prefix
	grouped := make(map[string][][2]string)
	for _, g := range prefixGroups {
		grouped[g] = nil
	}
	grouped["other"] = nil

	for _, kv := range props {
		g := groupKey(kv[0])
		grouped[g] = append(grouped[g], kv)
	}

	fmt.Printf("\n  System Properties \u2014 Nothing %s\n\n", model)

	groupOrder := append(prefixGroups, "other")
	for _, g := range groupOrder {
		entries := grouped[g]
		if len(entries) == 0 {
			continue
		}
		fmt.Printf("  [%s.*]\n", g)
		maxLen := 0
		for _, kv := range entries {
			if len(kv[0]) > maxLen {
				maxLen = len(kv[0])
			}
		}
		for _, kv := range entries {
			fmt.Printf("    %-*s  = %s\n", maxLen, kv[0], kv[1])
		}
		fmt.Println()
	}
	return nil
}

// ActionPropSet writes a system property via root su.
// Requires Magisk root.
func ActionPropSet(serial, key, value string) error {
	if !adb.CheckAdbRoot(serial) {
		return nterrors.AdbError(
			"Root not available via ADB shell.\n" +
				"Enable in Magisk: Settings -> Superuser access -> Apps and ADB.")
	}

	fmt.Println("  NOTE: Most ro.* properties reset on reboot. Use persist.* for persistent changes.")

	if strings.HasPrefix(key, "ro.") {
		fmt.Println("  [WARN] ro.* properties are read-only at the system level \u2014 this may not persist.")
	}

	_, stderr, code := adb.Run([]string{"adb", "-s", serial, "shell",
		fmt.Sprintf(`su -c "setprop %s %s"`, key, value)})
	if code != 0 {
		errMsg := strings.TrimSpace(stderr)
		return nterrors.AdbError(fmt.Sprintf("setprop %s failed: %s", key, errMsg))
	}

	fmt.Printf("  [OK] Set %s = %s\n", key, value)
	return nil
}
