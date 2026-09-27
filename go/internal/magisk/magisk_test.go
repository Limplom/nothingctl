package magisk

import "testing"

func TestMagiskTagToCode(t *testing.T) {
	cases := []struct {
		tag  string
		want int
	}{
		{"v30.7", 30700},
		{"30.7", 30700},
		{"v26.4", 26400},
		{"v27.0", 27000},
		{"v25.2", 25200},
		{"", 0},
		{"canary", 0},
		{"v30", 0},
	}
	for _, tc := range cases {
		if got := magiskTagToCode(tc.tag); got != tc.want {
			t.Errorf("magiskTagToCode(%q) = %d, want %d", tc.tag, got, tc.want)
		}
	}
}

// TestMagiskTagOrdering guards the IsOutdated comparison: codes derived from
// tags must sort the same way as the releases themselves.
func TestMagiskTagOrdering(t *testing.T) {
	ordered := []string{"v25.2", "v26.1", "v26.4", "v27.0", "v28.1", "v29.0", "v30.7"}
	for i := 1; i < len(ordered); i++ {
		a, b := magiskTagToCode(ordered[i-1]), magiskTagToCode(ordered[i])
		if a >= b {
			t.Errorf("%s (%d) should be < %s (%d)", ordered[i-1], a, ordered[i], b)
		}
	}
}

func TestIsDigit(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"30700", true},
		{"0", true},
		{"", false},
		{" 30700", false},
		{"30700\n", false},
		{"-1", false},
		{"30.7", false},
		{"su: not found", false},
		{"٣", false}, // non-ASCII digit
	}
	for _, tc := range cases {
		if got := isDigit(tc.in); got != tc.want {
			t.Errorf("isDigit(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
