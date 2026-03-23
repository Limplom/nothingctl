"""Anti-Rollback Protection (ARB) index checking.

Nothing phones use Android Verified Boot (AVB). The vbmeta partition contains
a rollback_index that the bootloader compares against a one-time-programmable
eFuse counter. If firmware_index < fuse_value the bootloader refuses to boot —
permanently, because eFuses cannot be reset.

This module reads the firmware index from vbmeta.img (local file, no device
needed) and the device's current index by pulling vbmeta_a via ADB root.
"""

import struct
import tempfile
from pathlib import Path

from .device import run
from .exceptions import FirmwareError

# AVB vbmeta header layout (all fields big-endian):
#   0  magic[4]                          "AVB0"
#   4  required_libavb_version_major[4]
#   8  required_libavb_version_minor[4]
#  12  authentication_data_block_size[8]
#  20  auxiliary_data_block_size[8]
#  28  algorithm_type[4]
#  32  hash_offset[8]
#  40  hash_size[8]
#  48  signature_offset[8]
#  56  signature_size[8]
#  64  public_key_offset[8]
#  72  public_key_size[8]
#  80  public_key_metadata_offset[8]
#  88  public_key_metadata_size[8]
#  96  descriptor_offset[8]
# 104  descriptor_size[8]
# 112  rollback_index[8]                 ← what we need
# 120  flags[4]
# 124  rollback_index_location[4]

_AVB_MAGIC         = b"AVB0"
_ARB_OFFSET        = 112
_HEADER_READ_BYTES = _ARB_OFFSET + 8   # 120 bytes


def _parse_vbmeta(path: Path) -> int | None:
    """Return the rollback_index from a vbmeta.img, or None on failure."""
    try:
        with open(path, "rb") as f:
            header = f.read(_HEADER_READ_BYTES)
        if len(header) < _HEADER_READ_BYTES or header[:4] != _AVB_MAGIC:
            return None
        return struct.unpack_from(">Q", header, _ARB_OFFSET)[0]
    except OSError:
        return None


def _device_arb_index(serial: str) -> int | None:
    """
    Pull vbmeta_a from the live device and extract its rollback_index.
    vbmeta is tiny (~4 KB), so this is fast even over USB.
    Requires ADB root.
    """
    remote = "/data/local/tmp/_arb_check_vbmeta.img"
    local  = Path(tempfile.gettempdir()) / "_arb_check_vbmeta.img"
    try:
        r = run(["adb", "-s", serial, "shell",
                 f"su -c 'dd if=/dev/block/by-name/vbmeta_a "
                 f"of={remote} bs=4096 count=1 2>/dev/null && echo __OK__'"])
        if "__OK__" not in r.stdout:
            return None
        r2 = run(["adb", "-s", serial, "pull", remote, str(local)])
        run(["adb", "-s", serial, "shell", f"rm -f {remote}"])
        if r2.returncode != 0:
            return None
        return _parse_vbmeta(local)
    finally:
        local.unlink(missing_ok=True)


def check_arb(fw_dir: Path, serial: str) -> None:
    """
    Compare rollback_index of the firmware to be flashed against the device.

    - fw_dir : directory containing the extracted firmware (must have vbmeta.img)
    - serial : ADB serial of the device (must have root for vbmeta_a read)

    Raises FirmwareError if the firmware index is lower than the device fuse value
    (which would result in a permanent boot loop after flashing).
    Prints a warning and continues if the check cannot be completed.
    """
    fw_vbmeta = fw_dir / "vbmeta.img"
    if not fw_vbmeta.exists():
        print("  ARB check : SKIP — vbmeta.img not in firmware package")
        return

    fw_index = _parse_vbmeta(fw_vbmeta)
    if fw_index is None:
        print("  ARB check : SKIP — could not parse vbmeta.img (no AVB magic)")
        return

    print(f"  Firmware ARB index : {fw_index}")

    dev_index = _device_arb_index(serial)
    if dev_index is None:
        print("  Device   ARB index : unknown (root unavailable or partition missing)")
        print("  ARB check : WARNING — cannot verify, proceed only if not downgrading")
        return

    print(f"  Device   ARB index : {dev_index}")

    if fw_index < dev_index:
        raise FirmwareError(
            f"\nDOWNGRADE BLOCKED — Anti-Rollback Protection would prevent boot.\n"
            f"  Firmware rollback index : {fw_index}\n"
            f"  Device fuse value       : {dev_index}\n\n"
            f"Flashing this firmware will cause a permanent boot loop.\n"
            f"You must use firmware with rollback index >= {dev_index}."
        )

    if fw_index == dev_index:
        print("  ARB check : OK  (same index — no fuse change)")
    else:
        print(f"  ARB check : OK  (upgrade: fuse {dev_index} -> {fw_index} after first boot)")
