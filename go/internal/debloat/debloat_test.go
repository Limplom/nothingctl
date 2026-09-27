package debloat

import (
	"regexp"
	"strings"
	"testing"
)

// criticalPackages must never appear in the debloat list. Even though
// "pm uninstall --user 0" is reversible via "pm install-existing", removing
// any of these leaves the device bootlooping, without a launcher/UI, without
// telephony, or without a way to re-enable adb/settings — i.e. the user may
// be unable to run the restore command at all. "--remove all" and every
// profile would apply them unconditionally.
var criticalPackages = []string{
	"android",                            // framework-res; system_server will not start
	"com.android.systemui",               // status bar, lockscreen, nav — UI unusable
	"com.android.settings",               // no way to toggle USB debugging back on
	"com.android.phone",                  // telephony stack; crash loop on SIM devices
	"com.android.providers.settings",     // Settings.* storage; bootloop
	"com.android.providers.telephony",    // SMS/APN db; phone process crash loop
	"com.android.providers.media.module", // MediaProvider (mainline); storage breaks
	"com.android.providers.contacts",     // contacts/dialer crash loop
	"com.android.shell",                  // adb shell uid; breaks adb-based restore
	"com.android.packageinstaller",       // app install/uninstall UI
	"com.google.android.packageinstaller",
	"com.android.permissioncontroller", // runtime permissions; bootloop on 10+
	"com.google.android.permissioncontroller",
	"com.android.externalstorage", // storage access framework
	"com.android.inputdevices",
	"com.android.keychain",
	"com.android.location.fused",
	"com.android.networkstack", // mainline network stack; no connectivity
	"com.google.android.networkstack",
	"com.android.se", // secure element / NFC payments
	"com.android.nfc",
	"com.android.bluetooth",
	"com.android.launcher3", // stock launcher (home screen)
	"com.nothing.launcher",  // Nothing launcher (home screen)
	"com.nothing.systemui",
	"com.google.android.gms", // Play services; many system apps crash
	"com.google.android.gsf", // Google Services Framework
	"com.android.vending",    // Play Store (needed to restore apps)
	"com.qualcomm.qti.telephonyservice",
	"com.android.ims",
	"org.codeaurora.ims",
	"com.android.server.telecom",
	"com.android.wifi.resources",
	"com.android.certinstaller",
	"com.android.webview",
	"com.google.android.webview",
	"com.google.android.trichromelibrary",
}

func TestPackagesLoad(t *testing.T) {
	pkgs, err := loadPackages()
	if err != nil {
		t.Fatalf("loadPackages: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("debloat list is empty")
	}
}

var (
	idRe  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	pkgRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)
)

func TestPackageEntriesValid(t *testing.T) {
	pkgs, err := loadPackages()
	if err != nil {
		t.Fatal(err)
	}
	validCategory := map[string]bool{"social": true, "nothing": true, "util": true}
	validProfile := map[string]bool{"minimal": true, "recommended": true, "aggressive": true}

	ids := map[string]bool{}
	names := map[string]bool{}
	for _, p := range pkgs {
		if !idRe.MatchString(p.ID) {
			t.Errorf("entry %+v: id %q is not a lowercase-dash identifier", p, p.ID)
		}
		if p.ID == "all" {
			t.Errorf("id %q collides with the --remove all keyword", p.ID)
		}
		if ids[p.ID] {
			t.Errorf("duplicate id %q", p.ID)
		}
		ids[p.ID] = true

		if !pkgRe.MatchString(p.Package) {
			t.Errorf("id %q: package %q is not a valid Android package name", p.ID, p.Package)
		}
		if names[p.Package] {
			t.Errorf("duplicate package %q", p.Package)
		}
		names[p.Package] = true

		if strings.TrimSpace(p.Name) == "" {
			t.Errorf("id %q: empty name", p.ID)
		}
		if !validCategory[p.Category] {
			t.Errorf("id %q: unknown category %q", p.ID, p.Category)
		}
		seen := map[string]bool{}
		for _, pr := range p.Profiles {
			if !validProfile[pr] {
				t.Errorf("id %q: unknown profile %q", p.ID, pr)
			}
			if seen[pr] {
				t.Errorf("id %q: duplicate profile %q", p.ID, pr)
			}
			seen[pr] = true
		}
	}
}

// TestProfilesEscalate: minimal ⊆ recommended ⊆ aggressive, and every profile
// that ActionDebloatProfile advertises selects at least one package.
func TestProfilesEscalate(t *testing.T) {
	pkgs, err := loadPackages()
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, p := range pkgs {
		has := map[string]bool{}
		for _, pr := range p.Profiles {
			has[pr] = true
			count[pr]++
		}
		if has["minimal"] && !has["recommended"] {
			t.Errorf("id %q is in minimal but not recommended", p.ID)
		}
		if has["recommended"] && !has["aggressive"] {
			t.Errorf("id %q is in recommended but not aggressive", p.ID)
		}
	}
	for _, pr := range []string{"minimal", "recommended", "aggressive"} {
		if count[pr] == 0 {
			t.Errorf("profile %q selects no packages", pr)
		}
	}
}

func TestNoCriticalPackages(t *testing.T) {
	pkgs, err := loadPackages()
	if err != nil {
		t.Fatal(err)
	}
	deny := map[string]bool{}
	for _, c := range criticalPackages {
		deny[c] = true
	}
	for _, p := range pkgs {
		if deny[p.Package] {
			t.Errorf("SAFETY: id %q removes critical package %q", p.ID, p.Package)
		}
		// Content providers and mainline modules are infrastructure, not bloat.
		if strings.HasPrefix(p.Package, "com.android.providers.") ||
			strings.HasPrefix(p.Package, "com.google.android.providers.") {
			t.Errorf("SAFETY: id %q removes content provider %q", p.ID, p.Package)
		}
	}
}
