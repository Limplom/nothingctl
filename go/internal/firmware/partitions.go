package firmware

import (
	"os"
	"path/filepath"
	"strings"
)

// firmwarePartitions are flashed in regular fastboot mode (image-firmware.7z).
var firmwarePartitions = []string{
	"modem", "lk", "preloader_raw", "tee", "dsp", "keystore", "persist",
}

// bootPartitions are flashed in regular fastboot mode (image-boot.7z),
// excluding init_boot (handled separately for Magisk patching).
var bootPartitions = []string{
	"boot", "dtbo", "vendor_boot",
	"vbmeta_system", "vbmeta_vendor", "vbmeta",
}

// logicalPartitions are flashed in fastbootd mode (image-logical.7z).
var logicalPartitions = []string{
	"system", "vendor", "product", "odm", "system_ext",
	"vendor_dlkm", "odm_dlkm", "system_dlkm",
}

// ScanAvailableImages returns the base names (without .img extension) of all
// *.img files present in dir that appear in allowList.
func ScanAvailableImages(dir string, allowList []string) []string {
	allow := make(map[string]bool, len(allowList))
	for _, p := range allowList {
		allow[p] = true
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".img") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".img")
		if allow[base] {
			found = append(found, base)
		}
	}
	return found
}

// ImgPath returns the full path to <name>.img within dir.
func ImgPath(dir, name string) string {
	return filepath.Join(dir, name+".img")
}
