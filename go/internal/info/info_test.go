package info

import (
	"strings"
	"testing"
)

func TestKbToGB(t *testing.T) {
	cases := []struct {
		kb   int64
		want string
	}{
		{0, "0.0 GB"},
		{1024 * 1024, "1.0 GB"},
		{7812345, "7.5 GB"},
		{11811160, "11.3 GB"},
	}
	for _, c := range cases {
		if got := kbToGB(c.kb); got != c.want {
			t.Errorf("kbToGB(%d) = %q, want %q", c.kb, got, c.want)
		}
	}
}

func TestParseMeminfo(t *testing.T) {
	sample := "MemTotal:        7812345 kB\nMemFree:          234567 kB\n"
	cases := []struct{ name, in, want string }{
		{"lf", sample, "7.5 GB"},
		{"crlf", strings.ReplaceAll(sample, "\n", "\r\n"), "7.5 GB"},
		{"not first line", "MemFree: 1 kB\nMemTotal: 1048576 kB\n", "1.0 GB"},
		{"empty", "", "not available"},
		{"missing value", "MemTotal:\n", "not available"},
		{"garbage value", "MemTotal: abc kB\n", "not available"},
	}
	for _, c := range cases {
		if got := parseMeminfo(c.in); got != c.want {
			t.Errorf("%s: parseMeminfo = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseDf(t *testing.T) {
	sample := "Filesystem     1K-blocks     Used Available Use% Mounted on\n" +
		"/dev/block/dm-45 115343360 47185920  68157440  41% /data\n"
	cases := []struct{ name, in, want string }{
		{"lf", sample, "45.0 GB used of 110.0 GB"},
		{"crlf", strings.ReplaceAll(sample, "\n", "\r\n"), "45.0 GB used of 110.0 GB"},
		{"trailing blank lines", sample + "\n\n", "45.0 GB used of 110.0 GB"},
		{"empty", "", "not available"},
		{"header only", "Filesystem     1K-blocks     Used Available Use% Mounted on\n", "not available"},
		{"too few fields", "/dev/x 123\n", "not available"},
		{"non-numeric", "/dev/x abc def ghi /data\n", "not available"},
	}
	for _, c := range cases {
		if got := parseDf(c.in); got != c.want {
			t.Errorf("%s: parseDf = %q, want %q", c.name, got, c.want)
		}
	}
}
