"""Partition backup and restore via root dd + fastboot."""

import datetime
import hashlib
from pathlib import Path

from .device import (
    adb_shell, adb_pull, fastboot_flash, fastboot_run,
    reboot_to_bootloader, run, confirm,
)
from .exceptions import AdbError, FirmwareError, FlashError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Partition lists
# ---------------------------------------------------------------------------

# Partitions dumped by --backup (and auto-backup before flash).
# Excludes: super, userdata (too large), sda/sdb/sdc (raw block aliases).
BACKUP_PARTITIONS = [
    "init_boot_a", "init_boot_b",
    "boot_a",      "boot_b",
    "dtbo_a",      "dtbo_b",
    "vendor_boot_a", "vendor_boot_b",
    "vbmeta_a",    "vbmeta_b",
    "vbmeta_system_a", "vbmeta_system_b",
    "vbmeta_vendor_a", "vbmeta_vendor_b",
    "lk_a",        "lk_b",
    "logo_a",      "logo_b",
    "preloader_raw_a", "preloader_raw_b",
    "modem_a",     "modem_b",
    "tee_a",       "tee_b",
    "nvram",       "nvdata",  "nvcfg",
    "factory",     "persist", "seccfg",  "proinfo",
]
BACKUP_TEMP_DIR = "/sdcard/Download/partition_backup"

# Partitions safe to flash via fastboot restore (no calibration data, no very-early bootloader).
RESTORE_SAFE = {
    "init_boot_a", "init_boot_b", "boot_a", "boot_b",
    "dtbo_a", "dtbo_b", "vendor_boot_a", "vendor_boot_b",
    "vbmeta_a", "vbmeta_b", "vbmeta_system_a", "vbmeta_system_b",
    "vbmeta_vendor_a", "vbmeta_vendor_b",
    "logo_a", "logo_b", "modem_a", "modem_b",
}

# Partitions that carry calibration data or very-early boot code.
# Flashing wrong data here can cause WiFi/BT/modem failure or brick.
# Only included with --restore-full.
RESTORE_RISKY = {
    "lk_a", "lk_b",
    "preloader_raw_a", "preloader_raw_b",
    "tee_a", "tee_b",
    "nvram", "nvdata", "nvcfg",
    "factory", "persist", "seccfg", "proinfo",
}


# ---------------------------------------------------------------------------
# Root check
# ---------------------------------------------------------------------------

def check_adb_root(serial: str) -> bool:
    """Return True if ADB shell has root access via su."""
    r = run(["adb", "-s", serial, "shell", "su -c id"])
    return r.returncode == 0 and "uid=0" in r.stdout


# ---------------------------------------------------------------------------
# Backup
# ---------------------------------------------------------------------------

def action_backup(device: DeviceInfo, device_dir: Path, label: str = "") -> Path:
    """
    Dump all BACKUP_PARTITIONS from the device to local storage.
    Requires ADB root (Magisk 'Apps and ADB' mode).
    Returns the local backup directory path.
    """
    if not check_adb_root(device.serial):
        raise AdbError(
            "Root not available via ADB shell.\n"
            "Enable in Magisk: Settings -> Superuser access -> Apps and ADB."
        )

    timestamp = label or datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    local_dir = device_dir / "Backups" / "partition-backup" / f"backup_{timestamp}"
    local_dir.mkdir(parents=True, exist_ok=True)

    print(f"\nBacking up partitions to: {local_dir}")
    print("Checking which partitions exist on device...")

    existing     = adb_shell("su -c 'ls /dev/block/by-name/'", device.serial).splitlines()
    existing_set = set(existing)
    to_dump      = [p for p in BACKUP_PARTITIONS if p in existing_set]
    skipped      = [p for p in BACKUP_PARTITIONS if p not in existing_set]
    if skipped:
        print(f"  Skipping (not present): {', '.join(skipped)}")

    adb_shell(f"su -c 'mkdir -p {BACKUP_TEMP_DIR}'", device.serial)

    failed = []
    for part in to_dump:
        print(f"  Dumping {part}...", end="", flush=True)
        r = run(["adb", "-s", device.serial, "shell",
                 f"su -c 'dd if=/dev/block/by-name/{part} "
                 f"of={BACKUP_TEMP_DIR}/{part}.img bs=4096 2>/dev/null'"])
        if r.returncode == 0:
            print(" OK")
        else:
            print(" FAIL")
            failed.append(part)

    if failed:
        print(f"  WARNING: Failed to dump: {', '.join(failed)}")

    print(f"\nPulling {len(to_dump) - len(failed)} images to PC...")
    r = run(["adb", "-s", device.serial, "pull",
             f"{BACKUP_TEMP_DIR}/.", str(local_dir)])
    if r.returncode != 0:
        raise AdbError(f"adb pull failed: {r.stderr.strip()}")

    adb_shell(f"su -c 'rm -rf {BACKUP_TEMP_DIR}'", device.serial)

    pulled   = list(local_dir.glob("*.img"))
    total_mb = sum(f.stat().st_size for f in pulled) / 1024 / 1024

    # Save SHA256 checksums alongside the images so --verify-backup can use them
    _save_checksums(pulled, local_dir)

    print(f"[OK] {len(pulled)} partitions backed up ({total_mb:.0f} MB) -> {local_dir}")
    return local_dir


def _sha256(path: Path) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(1 << 20):
            h.update(chunk)
    return h.hexdigest()


def _save_checksums(images: list[Path], dest_dir: Path) -> None:
    checksum_file = dest_dir / "checksums.sha256"
    lines = [f"{_sha256(img)}  {img.name}" for img in sorted(images)]
    checksum_file.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"  Checksums : {checksum_file.name} ({len(lines)} entries)")


# ---------------------------------------------------------------------------
# Restore helpers
# ---------------------------------------------------------------------------

def list_backups(device_dir: Path) -> list[Path]:
    backup_root = device_dir / "Backups" / "partition-backup"
    if not backup_root.exists():
        return []
    return sorted(
        [d for d in backup_root.iterdir() if d.is_dir() and d.name.startswith("backup_")],
        reverse=True,  # newest first
    )


def pick_backup(device_dir: Path, restore_dir: str | None) -> Path:
    if restore_dir:
        p = Path(restore_dir)
        if not p.exists():
            raise FirmwareError(f"Restore directory not found: {restore_dir}")
        return p

    backups = list_backups(device_dir)
    if not backups:
        raise FirmwareError(f"No partition backups found in {device_dir / 'Backups' / 'partition-backup'}")

    print("\nAvailable backups:")
    for i, b in enumerate(backups):
        imgs    = list(b.glob("*.img"))
        size_mb = sum(f.stat().st_size for f in imgs) / 1024 / 1024
        print(f"  [{i}] {b.name}  ({len(imgs)} partitions, {size_mb:.0f} MB)")

    try:
        choice = input("\nSelect backup [0]: ").strip()
        idx    = int(choice) if choice else 0
    except (ValueError, EOFError, KeyboardInterrupt):
        idx = 0

    if not 0 <= idx < len(backups):
        raise FirmwareError(f"Invalid selection: {idx}")
    return backups[idx]


def action_restore(device: DeviceInfo, device_dir: Path,
                   restore_dir: str | None, full_restore: bool) -> None:
    backup_path = pick_backup(device_dir, restore_dir)
    images      = {f.stem: f for f in backup_path.glob("*.img")}

    safe    = {k: v for k, v in images.items() if k in RESTORE_SAFE}
    risky   = {k: v for k, v in images.items() if k in RESTORE_RISKY}
    unknown = {k: v for k, v in images.items()
               if k not in RESTORE_SAFE and k not in RESTORE_RISKY}

    print(f"\nBackup : {backup_path.name}")
    print(f"Safe   ({len(safe):2d}): {', '.join(sorted(safe))}")
    if risky:
        print(f"Risky  ({len(risky):2d}): {', '.join(sorted(risky))}")
    if unknown:
        print(f"Unknown ({len(unknown):2d}): {', '.join(sorted(unknown))}  — skipped")

    to_flash = dict(safe)
    if full_restore and risky:
        print("\nWARNING: --restore-full includes calibration/bootloader partitions.")
        print("         Only do this if you are restoring to the exact same device.")
        to_flash.update(risky)
    elif risky:
        print("\nRisky partitions skipped. Use --restore-full to include them.")

    print(f"\nWill flash {len(to_flash)} partitions to device.")
    confirm("Reboot to bootloader and restore?")

    reboot_to_bootloader(device.serial)

    failed = []
    for part, img in sorted(to_flash.items()):
        try:
            fastboot_flash(part, img, device.serial)
        except FlashError as e:
            print(f"  WARN: {e}")
            failed.append(part)

    fastboot_run("reboot", serial=device.serial)

    if failed:
        print(f"\nWARNING: Failed to flash: {', '.join(failed)}")
    ok = len(to_flash) - len(failed)
    print(f"[OK] Restore complete — {ok}/{len(to_flash)} partitions flashed.")


# ---------------------------------------------------------------------------
# Partition health / verify
# ---------------------------------------------------------------------------

def action_verify_backup(device: DeviceInfo, device_dir: Path,
                         restore_dir: str | None) -> None:
    """
    Compare live device partition hashes against a stored backup's checksums.sha256.

    Hashing runs on-device via 'dd | sha256sum' — only the 64-char hash is
    transferred over USB, so this is fast even for large partitions like modem.
    Requires ADB root.
    """
    if not check_adb_root(device.serial):
        raise AdbError(
            "Root not available via ADB shell.\n"
            "Enable in Magisk: Settings -> Superuser access -> Apps and ADB."
        )

    backup_path = pick_backup(device_dir, restore_dir)
    checksum_file = backup_path / "checksums.sha256"

    if not checksum_file.exists():
        raise FirmwareError(
            f"No checksums.sha256 found in {backup_path.name}.\n"
            "This backup was created before health-check support was added.\n"
            "Run --backup again to create a new backup with checksums."
        )

    # Parse stored checksums: "<hash>  <filename>"
    stored: dict[str, str] = {}
    for line in checksum_file.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line:
            parts = line.split(None, 1)
            if len(parts) == 2:
                stored[parts[1].strip()] = parts[0].strip()  # filename -> hash

    print(f"\nVerifying {len(stored)} partitions against live device...")
    print(f"Backup: {backup_path.name}\n")
    print(f"  {'Partition':<22} {'Result':<10} Details")
    print("  " + "─" * 60)

    match = changed = missing = 0
    changed_parts: list[str] = []

    for filename, expected_hash in sorted(stored.items()):
        part = filename.removesuffix(".img")
        print(f"  {part:<22}", end="", flush=True)

        # Hash on-device via pipe — no USB transfer of partition data
        r = run(["adb", "-s", device.serial, "shell",
                 f"su -c 'dd if=/dev/block/by-name/{part} bs=4096 2>/dev/null | sha256sum'"])
        live_output = r.stdout.strip()

        if not live_output or "No such" in live_output or r.returncode != 0:
            print(f" {'MISSING':<10} partition not found on device")
            missing += 1
            continue

        live_hash = live_output.split()[0]

        if live_hash == expected_hash:
            print(f" {'MATCH':<10}")
            match += 1
        else:
            print(f" {'CHANGED':<10} live hash differs from backup")
            changed += 1
            changed_parts.append(part)

    print()
    print(f"  Results: {match} match  /  {changed} changed  /  {missing} missing")

    if changed:
        print(f"\n  Changed partitions: {', '.join(changed_parts)}")
        print("  NOTE: init_boot change is expected if Magisk is installed.")
        print("        Other changes may indicate OTA updates or unexpected modifications.")
    elif missing:
        print("\n  Some partitions are not present on this device (model-specific).")
    else:
        print("\n[OK] All partitions match the backup.")
