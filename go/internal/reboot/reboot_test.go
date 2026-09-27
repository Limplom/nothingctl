package reboot

import (
	"strings"
	"testing"
)

// validTargets mirrors the switch in ActionReboot and its doc comment.
var validTargets = map[string]bool{
	"system": true, "bootloader": true, "recovery": true,
	"safe": true, "download": true, "sideload": true,
}

// TestTargetMapConsistent checks every interactive menu choice maps to a
// target that ActionReboot actually handles, and that the menu text lists
// every choice.
func TestTargetMapConsistent(t *testing.T) {
	if len(targetMap) != len(validTargets) {
		t.Errorf("targetMap has %d entries, want %d", len(targetMap), len(validTargets))
	}
	seen := map[string]bool{}
	for k, v := range targetMap {
		if !validTargets[v] {
			t.Errorf("targetMap[%q] = %q is not a handled target", k, v)
		}
		if seen[v] {
			t.Errorf("target %q reachable from more than one menu key", v)
		}
		seen[v] = true
		if !strings.Contains(menu, "["+k+"]") {
			t.Errorf("menu does not list choice [%s]", k)
		}
	}
	if targetMap["0"] != "system" {
		t.Errorf("default choice 0 = %q, want system", targetMap["0"])
	}
}

// TestActionRebootRejectsUnknownTarget exercises the default branch, which
// returns before any adb call is made.
func TestActionRebootRejectsUnknownTarget(t *testing.T) {
	for _, target := range []string{"edl", "fastbootd", "bootloader ", "reboot", "0"} {
		err := ActionReboot("NO-DEVICE", target)
		if err == nil {
			t.Errorf("ActionReboot(%q): expected error", target)
			continue
		}
		if !strings.Contains(err.Error(), "Unknown reboot target") {
			t.Errorf("ActionReboot(%q): unexpected error %v", target, err)
		}
	}
}
