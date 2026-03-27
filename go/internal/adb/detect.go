package adb

import (
	"strings"

	nterrors "github.com/Limplom/nothingctl/internal/errors"
	"github.com/Limplom/nothingctl/internal/models"
)

// DetectDevice detects the connected Nothing device over ADB and returns a
// populated DeviceInfo. If serial is empty and exactly one device is attached
// its serial is used automatically. Multiple devices without an explicit serial
// is an error. Non-Nothing devices are rejected.
func DetectDevice(serial string) (*models.DeviceInfo, error) {
	detectedSerial, err := EnsureDevice(serial)
	if err != nil {
		return nil, err
	}

	// Prefer the human-friendly brand name (e.g. "Nothing Phone (1)") over the
	// raw model code (e.g. "A063") which Nothing uses for EEA variants.
	brandName, _, _ := Run([]string{"adb", "-s", detectedSerial, "shell",
		"getprop ro.product.brand_device_name"})
	brandName = strings.TrimSpace(brandName)

	modelCode, _, _ := Run([]string{"adb", "-s", detectedSerial, "shell",
		"getprop ro.product.model"})
	modelCode = strings.TrimSpace(modelCode)

	// Strip the manufacturer prefix so callers can prepend "Nothing " uniformly
	// without duplication (e.g. "Nothing Phone (1)" → "Phone (1)").
	if strings.HasPrefix(strings.ToLower(brandName), "nothing ") {
		brandName = brandName[8:]
	}
	model := brandName
	if model == "" {
		model = modelCode
	}

	manufacturer, _, _ := Run([]string{"adb", "-s", detectedSerial, "shell",
		"getprop ro.product.manufacturer"})
	manufacturer = strings.TrimSpace(manufacturer)

	codename, _, _ := Run([]string{"adb", "-s", detectedSerial, "shell",
		"getprop ro.product.device"})
	codename = strings.TrimSpace(codename)
	if len(codename) > 0 {
		codename = strings.ToUpper(codename[:1]) + codename[1:]
	}

	slot, _, _ := Run([]string{"adb", "-s", detectedSerial, "shell",
		"getprop ro.boot.slot_suffix"})
	slot = strings.TrimSpace(slot)

	if !strings.Contains(strings.ToLower(manufacturer), "nothing") {
		return nil, nterrors.FirmwareError(
			"not a Nothing device (manufacturer: " + manufacturer + "). " +
				"This tool only supports Nothing devices.",
		)
	}

	return &models.DeviceInfo{
		Serial:      detectedSerial,
		Model:       model,
		Codename:    codename,
		CurrentSlot: slot,
	}, nil
}
