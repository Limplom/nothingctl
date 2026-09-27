package sysmon

import (
	"strings"
	"testing"
)

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

func TestMhz(t *testing.T) {
	// Input is kHz as read from scaling_cur_freq / cpuinfo_max_freq.
	cases := []struct {
		khz  int64
		want string
	}{
		{0, "0 MHz"},
		{300000, "300 MHz"},
		{999999, "1000 MHz"},
		{1000000, "1.00 GHz"},
		{2400000, "2.40 GHz"},
		{3187200, "3.19 GHz"},
	}
	for _, c := range cases {
		if got := mhz(c.khz); got != c.want {
			t.Errorf("mhz(%d) = %q, want %q", c.khz, got, c.want)
		}
	}
}

func TestMib(t *testing.T) {
	cases := []struct {
		kb   int64
		want string
	}{
		{0, "0 MiB"},
		{512 * 1024, "512 MiB"},
		{1024 * 1024, "1.00 GiB"},
		{7812345, "7.45 GiB"},
	}
	for _, c := range cases {
		if got := mib(c.kb); got != c.want {
			t.Errorf("mib(%d) = %q, want %q", c.kb, got, c.want)
		}
	}
}

func TestASCIIBar(t *testing.T) {
	cases := []struct {
		value, total float64
		width        int
		wantFilled   int
	}{
		{0, 100, 10, 0},
		{50, 100, 10, 5},
		{100, 100, 10, 10},
		{250, 100, 10, 10}, // clamped
		{5, 0, 10, 0},      // zero total → blank
	}
	for _, c := range cases {
		got := asciiBar(c.value, c.total, c.width)
		if n := len([]rune(got)); n != c.width {
			t.Errorf("asciiBar(%v,%v,%d) width = %d, want %d", c.value, c.total, c.width, n, c.width)
		}
		if n := strings.Count(got, "█"); n != c.wantFilled {
			t.Errorf("asciiBar(%v,%v,%d) filled = %d, want %d", c.value, c.total, c.width, n, c.wantFilled)
		}
	}
}

const sampleProcMeminfo = `MemTotal:        7812345 kB
MemFree:          234567 kB
MemAvailable:    3456789 kB
Buffers:            1234 kB
Cached:          2345678 kB
SwapTotal:       4194300 kB
SwapFree:        4000000 kB
Active(anon):     987654 kB
HugePages_Total:       0
`

func TestParseProcMeminfo(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleProcMeminfo, "crlf": crlf(sampleProcMeminfo)} {
		t.Run(name, func(t *testing.T) {
			got := parseProcMeminfo(in)
			want := map[string]int64{
				"MemTotal":        7812345,
				"MemFree":         234567,
				"MemAvailable":    3456789,
				"SwapTotal":       4194300,
				"Active(anon)":    987654,
				"HugePages_Total": 0,
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s = %d, want %d", k, got[k], v)
				}
			}
		})
	}
	if got := parseProcMeminfo(""); len(got) != 0 {
		t.Errorf("empty input: got %v", got)
	}
	if got := parseProcMeminfo("not meminfo\n: 12\n"); len(got) != 0 {
		t.Errorf("garbage input: got %v", got)
	}
}

const sampleRssSummary = `Applications Memory Usage (in Kilobytes):
Uptime: 123456 Realtime: 123456

Total RSS by process:
    412,345K: system (pid 1234)
    298,765K: com.android.systemui (pid 2345 / activities)
     12,000K: com.example.app (pid 3456)

Total RSS by OOM adjustment:
    500,000K: Native
`

func TestParseRssSummary(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleRssSummary, "crlf": crlf(sampleRssSummary)} {
		t.Run(name, func(t *testing.T) {
			got := parseRssSummary(in)
			want := [][2]interface{}{
				{int64(412345), "system"},
				{int64(298765), "com.android.systemui"},
				{int64(12000), "com.example.app"},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
				}
			}
		})
	}
	if got := parseRssSummary("no section here\n"); len(got) != 0 {
		t.Errorf("no section: got %v", got)
	}
}

const sampleAppMeminfo = ` App Summary
                       Pss(KB)                        Rss(KB)
                        ------                         ------
           Java Heap:     8392                          21436
         Native Heap:     5532                           7032
                Code:     2560                          25064
               Stack:      620                            628
            Graphics:     1968                           1968
       Private Other:     1732
              System:     3436
             Unknown:                                    1432

           TOTAL PSS:    24240            TOTAL RSS:    58560       TOTAL SWAP PSS:       46
`

func TestParseAppMeminfo(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleAppMeminfo, "crlf": crlf(sampleAppMeminfo)} {
		t.Run(name, func(t *testing.T) {
			got := parseAppMeminfo(in)
			want := [][2]interface{}{
				{"Java Heap", int64(8392)},
				{"Native Heap", int64(5532)},
				{"Code", int64(2560)},
				{"Stack", int64(620)},
				{"Graphics", int64(1968)},
				{"Private Other", int64(1732)},
				{"System", int64(3436)},
				{"TOTAL PSS", int64(24240)},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("row %d = %v, want %v", i, got[i], want[i])
				}
			}
		})
	}
	if got := parseAppMeminfo(""); len(got) != 0 {
		t.Errorf("empty input: got %v", got)
	}
}

func TestParseCPUFreqs(t *testing.T) {
	in := "0|1804800|1804800|1\r\n4|2419200|2419200|1\r\n7|0|3187200|0\r\nbogus|1|2|3\r\n1|2\r\n"
	got := parseCPUFreqs(in)
	want := []coreInfo{
		{0, 1804800, 1804800, true},
		{4, 2419200, 2419200, true},
		{7, 0, 3187200, false},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d cores, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("core %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := parseCPUFreqs(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}

func TestClassifyCluster(t *testing.T) {
	cases := []struct {
		soc   string
		core  int
		maxHz int64
		want  string
	}{
		{"snapdragon", 0, 1800000, "Silver"},
		{"snapdragon", 3, 1800000, "Silver"},
		{"snapdragon", 4, 2400000, "Gold"},
		{"snapdragon", 6, 2400000, "Gold"},
		{"snapdragon", 7, 3200000, "Prime"},
		{"unknown", 7, 0, "Prime"},
		{"mediatek", 0, 2000000, "Efficiency"},
		{"mediatek", 0, 2100000, "Efficiency"},
		{"mediatek", 7, 2800000, "Performance"},
	}
	for _, c := range cases {
		if got := classifyCluster(c.soc, c.core, c.maxHz); got != c.want {
			t.Errorf("classifyCluster(%q,%d,%d) = %q, want %q", c.soc, c.core, c.maxHz, got, c.want)
		}
	}
}

const sampleTop = `Tasks: 812 total,   1 running, 811 sleeping,   0 stopped,   0 zombie
  Mem:  7812345K total,  7500000K used,   312345K free,    12345K buffers
 Swap:  4194300K total,   194300K used,  4000000K free,  2345678K cached
800%cpu  12%user   0%nice  10%sys 776%idle   0%iow   2%irq   0%sirq   0%host
  PID USER         [%CPU %MEM ARGS
 1234 system        12.5  5.1 system_server
 2345 u0_a123        3.0  2.2 com.example.app --flag
 3456 root           0.0  0.1 kworker/u16:1
 4567 shell         45.0  0.0 top -b -n 1
  bad line
`

func TestParseTopProcesses(t *testing.T) {
	for name, in := range map[string]string{"lf": sampleTop, "crlf": crlf(sampleTop)} {
		t.Run(name, func(t *testing.T) {
			got := parseTopProcesses(in, 3)
			want := []procInfo{
				{45.0, 4567, "top"},
				{12.5, 1234, "system_server"},
				{3.0, 2345, "com.example.app"},
			}
			if len(got) != len(want) {
				t.Fatalf("got %d procs, want %d: %+v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("proc %d = %+v, want %+v", i, got[i], want[i])
				}
			}
		})
	}
	// Without a header line nothing is parsed.
	if got := parseTopProcesses(" 1 root 1.0 0.0 init\n", 5); len(got) != 0 {
		t.Errorf("no header: got %+v", got)
	}
}
