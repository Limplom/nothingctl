package audio

import (
	"strings"
	"testing"
)

func TestResolveStream(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"0", 0, false},
		{"3", 3, false},
		{"5", 5, false},
		{"6", 0, true},
		{"-1", 0, true},
		{"media", 3, false},
		{"MUSIC", 3, false},
		{"Notify", 5, false},
		{"call", 0, false},
		{"bogus", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := resolveStream(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("resolveStream(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("resolveStream(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseStreamVolume(t *testing.T) {
	cases := []struct {
		in       string
		cur, max int
		ok       bool
	}{
		{"[V] will control stream=3 (STREAM_MUSIC)\r\n[V] volume is 7 in range [0..15]\r\n", 7, 15, true},
		{"volume is 0 in range [1..7]", 0, 7, true},
		{"Error: stream not found", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		cur, max, ok := parseStreamVolume(c.in)
		if cur != c.cur || max != c.max || ok != c.ok {
			t.Errorf("parseStreamVolume(%q) = (%d, %d, %v), want (%d, %d, %v)",
				c.in, cur, max, ok, c.cur, c.max, c.ok)
		}
	}
}

func TestBar(t *testing.T) {
	cases := []struct {
		cur, max   int
		wantFilled int
	}{
		{0, 15, 0},
		{15, 15, barWidth},
		{7, 15, 9}, // 9.33 → rounds to 9
		{8, 15, 11},
		{20, 15, barWidth}, // clamped
		{-3, 15, 0},        // clamped
		{5, 0, 0},          // invalid max
	}
	for _, c := range cases {
		got := bar(c.cur, c.max)
		if n := len([]rune(got)); n != barWidth {
			t.Errorf("bar(%d,%d) width = %d, want %d", c.cur, c.max, n, barWidth)
		}
		if n := strings.Count(got, "█"); n != c.wantFilled {
			t.Errorf("bar(%d,%d) filled = %d, want %d", c.cur, c.max, n, c.wantFilled)
		}
	}
}
