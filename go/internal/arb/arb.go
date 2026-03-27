// Package arb implements Anti-Rollback Protection (ARB) index checking for
// Nothing Phone devices.
//
// Nothing phones use Android Verified Boot (AVB). The vbmeta partition contains
// a rollback_index that the bootloader compares against a one-time-programmable
// eFuse counter. If firmware_index < fuse_value the bootloader refuses to boot —
// permanently, because eFuses cannot be reset.
package arb

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Limplom/nothingctl/internal/adb"
	nterrors "github.com/Limplom/nothingctl/internal/errors"
)

// AVB vbmeta header layout (all fields big-endian):
//
//	  0  magic[4]                          "AVB0"
//	  4  required_libavb_version_major[4]
//	  8  required_libavb_version_minor[4]
//	 12  authentication_data_block_size[8]
//	 20  auxiliary_data_block_size[8]
//	 28  algorithm_type[4]
//	 32  hash_offset[8]
//	 40  hash_size[8]
//	 48  signature_offset[8]
//	 56  signature_size[8]
//	 64  public_key_offset[8]
//	 72  public_key_size[8]
//	 80  public_key_metadata_offset[8]
//	 88  public_key_metadata_size[8]
//	 96  descriptor_offset[8]
//	104  descriptor_size[8]
//	112  rollback_index[8]   ← what we need
//	120  flags[4]
//	124  rollback_index_location[4]

const (
	avbMagic      = "AVB0"
	arbOffset     = 112
	headerReadLen = arbOffset + 8 // 120 bytes
)

// ParseVbmeta reads a vbmeta.img file and returns its rollback_index field.
// Returns an error if the file cannot be read, is too short, or lacks the AVB
// magic bytes.
func ParseVbmeta(vbmetaPath string) (uint64, error) {
	f, err := os.Open(vbmetaPath)
	if err != nil {
		return 0, nterrors.FirmwareError("opening vbmeta.img: " + err.Error())
	}
	defer f.Close()

	header := make([]byte, headerReadLen)
	n, err := f.Read(header)
	if err != nil || n < headerReadLen {
		return 0, nterrors.FirmwareError(
			fmt.Sprintf("vbmeta.img too short (read %d bytes, need %d)", n, headerReadLen),
		)
	}
	if string(header[:4]) != avbMagic {
		return 0, nterrors.FirmwareError(
			fmt.Sprintf("invalid AVB magic in %s (got %q)", vbmetaPath, string(header[:4])),
		)
	}

	index := binary.BigEndian.Uint64(header[arbOffset:])
	return index, nil
}

// deviceARBIndex pulls vbmeta_a from the live device (requires ADB root) and
// extracts its rollback_index. Returns (0, false) when the check cannot be
// completed.
func deviceARBIndex(serial string) (uint64, bool) {
	remote := "/data/local/tmp/_arb_check_vbmeta.img"
	localPath := filepath.Join(os.TempDir(), "_arb_check_vbmeta.img")
	defer os.Remove(localPath)

	stdout, _, code := adb.Run([]string{
		"adb", "-s", serial, "shell",
		"su -c 'dd if=/dev/block/by-name/vbmeta_a of=" + remote +
			" bs=4096 count=1 2>/dev/null && echo __OK__'",
	})
	if code != 0 || !strings.Contains(stdout, "__OK__") {
		return 0, false
	}

	if err := adb.AdbPull(serial, remote, localPath); err != nil {
		return 0, false
	}
	adb.Run([]string{"adb", "-s", serial, "shell", "rm -f " + remote})

	idx, err := ParseVbmeta(localPath)
	if err != nil {
		return 0, false
	}
	return idx, true
}

// CheckARB compares the rollback_index of the firmware about to be flashed
// against the current device eFuse counter.
//
// firmwareDir must contain vbmeta.img from the extracted firmware package.
// serial is the ADB serial of the device (root access required for the device
// side of the check).
//
// Returns a FirmwareError if flashing would trigger ARB (permanent boot loop).
// Prints a warning and returns nil if the check cannot be completed.
func CheckARB(serial, firmwareDir string) error {
	vbmetaPath := filepath.Join(firmwareDir, "vbmeta.img")
	if _, err := os.Stat(vbmetaPath); err != nil {
		fmt.Println("  ARB check : SKIP — vbmeta.img not in firmware package")
		return nil
	}

	fwIndex, err := ParseVbmeta(vbmetaPath)
	if err != nil {
		fmt.Println("  ARB check : SKIP — could not parse vbmeta.img (no AVB magic)")
		return nil
	}
	fmt.Printf("  Firmware ARB index : %d\n", fwIndex)

	devIndex, ok := deviceARBIndex(serial)
	if !ok {
		fmt.Println("  Device   ARB index : unknown (root unavailable or partition missing)")
		fmt.Println("  ARB check : WARNING — cannot verify, proceed only if not downgrading")
		return nil
	}
	fmt.Printf("  Device   ARB index : %d\n", devIndex)

	if fwIndex < devIndex {
		return nterrors.FirmwareError(
			fmt.Sprintf(
				"\nDOWNGRADE BLOCKED — Anti-Rollback Protection would prevent boot.\n"+
					"  Firmware rollback index : %d\n"+
					"  Device fuse value       : %d\n\n"+
					"Flashing this firmware will cause a permanent boot loop.\n"+
					"You must use firmware with rollback index >= %d.",
				fwIndex, devIndex, devIndex,
			),
		)
	}

	if fwIndex == devIndex {
		fmt.Println("  ARB check : OK  (same index — no fuse change)")
	} else {
		fmt.Printf("  ARB check : OK  (upgrade: fuse %d -> %d after first boot)\n", devIndex, fwIndex)
	}
	return nil
}
