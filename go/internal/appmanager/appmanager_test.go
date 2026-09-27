package appmanager

import (
	"strings"
	"testing"
)

func TestFmtBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.0 B"},
		{1023, "1023.0 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{3 * 1024 * 1024 * 1024, "3.0 GB"},
		{2 * 1024 * 1024 * 1024 * 1024, "2.0 TB"},
	}
	for _, c := range cases {
		if got := fmtBytes(c.in); got != c.want {
			t.Errorf("fmtBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInstallerLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "Unknown / sideloaded"},
		{"null", "Unknown / sideloaded"},
		{"com.android.vending", "Google Play Store"},
		{"org.fdroid.fdroid", "F-Droid"},
		{"com.aurora.store", "com.aurora.store"},
	}
	for _, c := range cases {
		if got := installerLabel(c.in); got != c.want {
			t.Errorf("installerLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

const sampleDumpsysPackage = `Activity Resolver Table:
  Non-Data Actions:
      android.intent.action.MAIN:
        abc1234 com.example.app/.MainActivity filter def5678

Packages:
  Package [com.example.app] (a1b2c3d):
    userId=10123
    codePath=/data/app/~~AbC==/com.example.app-XyZ==
    versionCode=42 minSdk=26 targetSdk=34
    versionName=1.2.3
    timeStamp=2024-05-01 10:00:00
    lastUpdateTime=2024-05-02 11:22:33
    installerPackageName=com.android.vending
    User 0: ceDataInode=123 installed=true hidden=false stopped=false enabled=3 instant=false
      firstInstallTime=2024-01-01 09:00:00
    User 10: ceDataInode=456 installed=true hidden=false stopped=false enabled=0 instant=false
      firstInstallTime=2024-03-03 08:00:00
  Package [com.example.app.helper] (e4f5a6b):
    versionCode=7 minSdk=21 targetSdk=33
    versionName=0.1
`

func TestExtractPackagesSection(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		in := strings.ReplaceAll(sampleDumpsysPackage, "\n", eol)
		block := extractPackagesSection(in, "com.example.app")
		if block == "" {
			t.Fatalf("eol %q: empty block", eol)
		}
		// Must stop before the next package — and must not match the
		// longer "com.example.app.helper" name as a prefix.
		if strings.Contains(block, "versionName=0.1") {
			t.Errorf("eol %q: block leaked into next package", eol)
		}

		checks := []struct {
			name string
			got  string
			want string
		}{
			{"versionName", reFirst(versionNameRe, block), "1.2.3"},
			{"versionCode", reFirst(versionCodeRe, block), "42"},
			{"minSdk", reFirst(minSdkRe, block), "26"},
			{"targetSdk", reFirst(targetSdkRe, block), "34"},
			{"codePath", reFirst(codePathRe, block), "/data/app/~~AbC==/com.example.app-XyZ=="},
			{"installer", reFirst(installerRe, block), "com.android.vending"},
			{"lastUpdate", reFirst(lastUpdateTimeRe, userSplitRe.Split(block, 2)[0]), "2024-05-02 11:22:33"},
			{"firstInstall(user0)", reFirst(firstInstallRe, user0Re.FindString(block)), "2024-01-01 09:00:00"},
			{"enabled(user0)", reFirst(enabledRe, block), "3"},
		}
		for _, c := range checks {
			if c.got != c.want {
				t.Errorf("eol %q: %s = %q, want %q", eol, c.name, c.got, c.want)
			}
		}

		helper := extractPackagesSection(in, "com.example.app.helper")
		if reFirst(versionNameRe, helper) != "0.1" {
			t.Errorf("eol %q: helper block = %q", eol, helper)
		}
	}
}

func TestExtractPackagesSectionMissing(t *testing.T) {
	if got := extractPackagesSection(sampleDumpsysPackage, "com.not.installed"); got != "" {
		t.Errorf("missing package: got %q, want empty", got)
	}
	if got := extractPackagesSection("", "com.example.app"); got != "" {
		t.Errorf("empty input: got %q, want empty", got)
	}
}

func TestReFirst(t *testing.T) {
	if got := reFirst(versionNameRe, "no match"); got != "" {
		t.Errorf("no match: got %q", got)
	}
	if got := reFirst(lastUpdateTimeRe, "lastUpdateTime=2024-01-01 00:00:00 \r"); got != "2024-01-01 00:00:00" {
		t.Errorf("trailing space/CR: got %q", got)
	}
}

func TestParsePackageList(t *testing.T) {
	in := strings.Join([]string{
		"package:/data/app/~~x==/org.fdroid.fdroid-y==/base.apk=org.fdroid.fdroid versionCode:1019050",
		"package:/data/app/com.example.a=b-1/base.apk=com.example.ab versionCode:3",
		"package:/system/app/Legacy/Legacy.apk=com.legacy",
		"",
		"WARNING: linker: something",
		"package:com.bare",
	}, "\r\n")
	got := parsePackageList(in)
	want := []pkgRow{
		{"com.bare", "", ""},
		{"com.example.ab", "3", "/data/app/com.example.a=b-1/base.apk"},
		{"com.legacy", "", "/system/app/Legacy/Legacy.apk"},
		{"org.fdroid.fdroid", "1019050", "/data/app/~~x==/org.fdroid.fdroid-y==/base.apk"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := parsePackageList(""); len(got) != 0 {
		t.Errorf("empty input: got %+v", got)
	}
}
