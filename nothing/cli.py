"""
nothingctl — Nothing OS firmware manager + root maintenance tool.

Modes:
  (default)         Check for updates, download if newer, report patch target
  --backup          Dump all critical partitions from device to local storage (requires root)
  --restore         Flash partitions from a backup back to device (safe set by default)
  --flash-firmware  Flash all boot partitions from nothing_archive (replaces OTA)
  --ota-update      One-shot: download + auto-patch with Magisk CLI + flash (root required;
                    falls back to --push-for-patch workflow if no root)
  --unroot          Flash stock patch target to both slots (prepare for OTA)
  --push-for-patch  Push stock patch target to /sdcard/Download/ for Magisk patching
  --flash-patched   Pull magisk_patched*.img from device and flash to both slots
  --modules         List recommended Magisk modules with install status
  --debloat         List / remove pre-installed NothingOS bloatware (reversible)
  --wifi-adb        Switch to wireless ADB mode and connect automatically
  --verify-backup   Compare live partition hashes against a stored backup
  --glyph           Show Glyph interface status and zone map for this device
  --thermal         Show thermal zone temperatures (all Snapdragon sensors)
  --sideload        Install an APK or split-APK directory via ADB
  --app-backup      Backup APK + data for specified apps (requires root for data)
  --app-restore     Restore apps from a backup created by --app-backup
  --history         Display the flash operation history log
  --storage-report  Show top-N largest directories in /data/data/, /sdcard/
  --apk-extract     Pull APKs for all user-installed apps to local storage
  --logcat          Dump current logcat buffer to a local file
  --bugreport       Generate full adb bugreport ZIP (30-90 seconds)
  --anr-dump        Pull ANR traces and tombstones from device (requires root)

Flags:
  --no-backup       Skip the automatic backup before flash operations
  --restore-dir     Path to a specific backup directory (skips interactive selection)
  --restore-full    Also restore risky partitions: preloader, tee, nvram, calibration data
  --install         Module IDs to install (use with --modules). E.g.: lsposed,shamiko  or  all
  --remove          Package IDs to remove (use with --debloat). E.g.: facebook,linkedin  or  all
  --packages        App package names for --app-backup. E.g.: com.example.app,com.other.app
  --apk             Path to APK or split-APK directory for --sideload
  --downgrade       Allow version downgrade when sideloading (adb install -d)
  --watch           Refresh thermal display every 2 seconds (use with --thermal)
  --glyph-enable    Toggle Glyph interface: on / off (use with --glyph)
  --top-n           Number of results for --storage-report (default: 20)
  --include-system  Include system apps in --apk-extract
  --package         Package name filter for --logcat
  --tag             Log tag filter for --logcat
  --level           Minimum log level for --logcat: V/D/I/W/E (default: all)
  --lines           Max log lines for --logcat (default: 500)

Workflow — automated OTA (root preserved, one command):
  python nothingctl.py --ota-update     # download + patch + flash in one shot

Workflow — manual OTA (no root, or step-by-step control):
  python nothingctl.py --unroot         # remove root so OTA can proceed
  [user applies OTA on device]
  python nothingctl.py --push-for-patch # push init_boot/boot to device
  [user patches in Magisk app]
  python nothingctl.py --flash-patched  # flash patched image to both slots
"""

import argparse
import sys
from pathlib import Path

from .arb import check_arb
from .app_backup import action_app_backup, action_app_restore
from .backup import action_backup, action_restore, action_verify_backup, check_adb_root
from .battery import action_battery
from .capture import action_screenshot, action_screenrecord
from .debloat import action_debloat
from .diagnostics import action_anr_dump, action_bugreport, action_logcat
from .glyph import action_glyph, action_glyph_pattern
from .history import action_history, log_flash
from .info import action_info
from .performance import action_performance
from .permissions import action_permissions
from .prop import action_prop_get, action_prop_set
from .reboot import action_reboot
from .sideload import action_sideload
from .storage import action_apk_extract, action_storage_report
from .thermal import action_thermal
from .wifi_adb import action_wifi_adb, action_adb_pair
from .device import (
    adb_pull, adb_push, confirm, detect_device,
    fastboot_flash_ab, fastboot_run, query_current_slot, reboot_to_bootloader, run,
)
from .exceptions import AdbError, FastbootTimeoutError, FlashError, FirmwareError, MagiskError
from .firmware import (
    SDCARD_DOWNLOAD, build_partition_list, find_magisk_patched, resolve_firmware,
)
from .magisk import action_install_magisk, check_magisk, print_magisk_status, print_root_status
from .models import DeviceInfo, FirmwareState, MagiskStatus
from .modules import action_modules, action_modules_toggle, action_modules_status


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------

def action_check(device: DeviceInfo, fw: FirmwareState, ms: MagiskStatus) -> None:
    target_path = fw.extracted_dir / fw.boot_target.filename
    print(f"\nPatch target : {fw.boot_target.filename}"
          f"  ({'GKI 2.0' if fw.boot_target.is_gki2 else 'legacy boot'})")
    print(f"Location     : {target_path}")
    print_magisk_status(ms)
    print("\nNext steps:")
    if fw.is_newer:
        print("  1. python nothingctl.py --ota-update       (auto: download + patch + flash)")
        print("     — or manually —")
        print("     python nothingctl.py --flash-firmware   (flash new firmware)")
        print("     python nothingctl.py --push-for-patch   (push image for Magisk app)")
        print("     python nothingctl.py --flash-patched    (flash patched image)")
    else:
        print("  Device is up to date. When a new update arrives, run:")
        print("     python nothingctl.py --ota-update       (one-shot root-preserving update)")
    if not ms.app_installed or ms.is_outdated:
        print("  *  python nothingctl.py --install-magisk   (install/update Magisk)")


def action_flash_firmware(device: DeviceInfo, fw: FirmwareState,
                          base_dir: Path, skip_backup: bool = False) -> None:
    partitions = build_partition_list(fw.boot_target)
    print(f"\nWill flash to BOTH slots: {', '.join(partitions)}")

    print("\nAnti-Rollback Protection check:")
    check_arb(fw.extracted_dir, device.serial)

    if not skip_backup:
        if check_adb_root(device.serial):
            print("\nAuto-backup before flash (use --no-backup to skip)...")
            action_backup(device, base_dir / device.codename, label=f"pre_flash_{fw.version}")
        else:
            print("\nWARNING: Root not available — skipping auto-backup.")
            print("         Run with --no-backup to suppress this warning.")

    confirm("Reboot to bootloader and flash?")

    reboot_to_bootloader(device.serial)
    slot = query_current_slot(device.serial)
    print(f"Active slot: {slot}")

    for part in partitions:
        img = fw.extracted_dir / f"{part}.img"
        if not img.exists():
            print(f"  Skipping {part} (image not found in package)")
            continue
        fastboot_flash_ab(part, img, device.serial)

    print("\nFlash complete. Rebooting...")
    fastboot_run("reboot", serial=device.serial)
    log_flash(base_dir, {
        "operation": "flash-firmware",
        "version":   fw.version,
        "serial":    device.serial,
        "arb_index": None,  # filled by check_arb if available
    })
    print("[OK] Firmware flashed. Now run --push-for-patch, patch in Magisk, then --flash-patched.")


def action_unroot(device: DeviceInfo, fw: FirmwareState) -> None:
    img = fw.extracted_dir / fw.boot_target.filename
    print(f"\nWill flash STOCK {fw.boot_target.filename} to both slots (removes root).")
    confirm("Proceed?")

    reboot_to_bootloader(device.serial)
    fastboot_flash_ab(fw.boot_target.partition_base, img, device.serial)
    fastboot_run("reboot", serial=device.serial)
    print("[OK] Device is now unrooted. OTA update can proceed.")


def action_push_for_patch(device: DeviceInfo, fw: FirmwareState) -> None:
    img    = fw.extracted_dir / fw.boot_target.filename
    remote = f"{SDCARD_DOWNLOAD}/{fw.boot_target.filename}"
    print(f"\nPushing {fw.boot_target.filename} to {remote}...")
    adb_push(img, remote, device.serial)

    # trigger media scan so file picker sees the file
    run(["adb", "-s", device.serial, "shell",
         f"am broadcast -a android.intent.action.MEDIA_SCANNER_SCAN_FILE "
         f"-d file://{remote}"])

    print("[OK] File ready on device.")
    print(f"\nNow open Magisk -> Install -> Patch an Image -> Downloads -> {fw.boot_target.filename}")
    print("Then run: python nothingctl.py --flash-patched")


def _magisk_cli_patch(device: DeviceInfo, local_img: Path, fw: FirmwareState) -> Path:
    """Push stock image to /data/local/tmp/, patch via Magisk CLI, pull result back."""
    TEMP      = "/data/local/tmp"
    img_name  = fw.boot_target.filename
    remote_in = f"{TEMP}/{img_name}"

    print(f"  Pushing {img_name} to device...")
    adb_push(local_img, remote_in, device.serial)

    # Use a sentinel echo so we can detect success without relying on adb exit codes,
    # which are unreliable when tunnelled through 'su -c' on some Magisk builds.
    print("  Patching with Magisk CLI...")
    r = run(["adb", "-s", device.serial, "shell",
             f"su -c 'magisk --patch-file {remote_in}' && echo __PATCH_OK__"])

    if "__PATCH_OK__" not in r.stdout:
        run(["adb", "-s", device.serial, "shell", f"rm -f {remote_in}"])
        raise MagiskError(
            "Magisk CLI patch failed. Ensure Magisk is installed and root is granted.\n"
            f"Output: {(r.stdout + r.stderr).strip()}"
        )

    # Magisk writes the output to the same directory as the input
    r2 = run(["adb", "-s", device.serial, "shell",
              f"ls -t {TEMP}/magisk_patched_*.img 2>/dev/null | head -1"])
    remote_out = r2.stdout.strip()
    if not remote_out:
        run(["adb", "-s", device.serial, "shell", f"rm -f {remote_in}"])
        raise MagiskError("Patched image not found in /data/local/tmp/ after Magisk patch.")

    patch_name    = remote_out.split("/")[-1]
    local_patched = local_img.parent / patch_name
    print(f"  Pulling {patch_name}...")
    adb_pull(remote_out, local_patched, device.serial)

    run(["adb", "-s", device.serial, "shell", f"rm -f {remote_in} {remote_out}"])
    return local_patched


def action_ota_update(device: DeviceInfo, fw: FirmwareState,
                      base_dir: Path, skip_backup: bool = False) -> None:
    if not fw.is_newer:
        print("\nDevice is already on the latest firmware — nothing to update.")
        print("Use --force-download to re-patch the current version anyway.")
        return

    local_img = fw.extracted_dir / fw.boot_target.filename
    has_root  = check_adb_root(device.serial)

    if has_root:
        print("\n[Auto] Root detected — patching with Magisk CLI.")
        local_patched = _magisk_cli_patch(device, local_img, fw)
    else:
        print("\n[Manual] No root — pushing image for Magisk app to patch.")
        action_push_for_patch(device, fw)
        print("\nAfter patching in the Magisk app, run:")
        print("  python nothingctl.py --flash-patched")
        return

    print("\nAnti-Rollback Protection check:")
    check_arb(fw.extracted_dir, device.serial)

    if not skip_backup:
        print("\nAuto-backup before flash (use --no-backup to skip)...")
        action_backup(device, base_dir / device.codename, label=f"pre_ota_{fw.version}")

    print(f"\nWill flash patched {fw.boot_target.partition_base} to BOTH slots.")
    confirm("Reboot to bootloader and flash?")

    reboot_to_bootloader(device.serial)
    fastboot_flash_ab(fw.boot_target.partition_base, local_patched, device.serial)
    fastboot_run("reboot", serial=device.serial)
    log_flash(base_dir, {
        "operation": "ota-update",
        "version":   fw.version,
        "serial":    device.serial,
        "arb_index": None,
    })
    print(f"[OK] OTA complete. Root preserved on both slots. Now on {fw.version}.")


def action_flash_patched(device: DeviceInfo, fw: FirmwareState,
                         base_dir: Path, skip_backup: bool = False) -> None:
    print("\nLooking for magisk_patched*.img on device...")

    if not skip_backup:
        if check_adb_root(device.serial):
            print("Auto-backup before flash (use --no-backup to skip)...")
            action_backup(device, base_dir / device.codename, label="pre_patch_flash")
        else:
            print("WARNING: Root not available — skipping auto-backup.")

    remote = find_magisk_patched(device.serial)
    if not remote:
        raise AdbError(
            f"No magisk_patched*.img found in {SDCARD_DOWNLOAD}.\n"
            "Patch the image in Magisk first, then re-run --flash-patched."
        )

    filename = remote.split("/")[-1]
    local    = fw.extracted_dir / filename
    print(f"  Found: {filename}")
    print(f"Pulling to: {local}")
    adb_pull(remote, local, device.serial)

    print(f"\nWill flash patched {fw.boot_target.partition_base} to BOTH slots.")
    confirm("Reboot to bootloader and flash?")

    reboot_to_bootloader(device.serial)
    fastboot_flash_ab(fw.boot_target.partition_base, local, device.serial)
    fastboot_run("reboot", serial=device.serial)
    print("[OK] Patched image flashed. Device is rooted on both slots.")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Nothing OS firmware manager + Magisk root maintenance",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("--serial",         help="ADB serial (required with multiple devices)")
    parser.add_argument("--base-dir",
                        default=str(Path.home() / "tools" / "Nothing"),
                        help="Storage root (default: ~/tools/Nothing)")
    parser.add_argument("--force-download", action="store_true",
                        help="Re-download firmware even if cached")
    parser.add_argument("--no-backup",      action="store_true",
                        help="Skip automatic backup before --flash-firmware / --flash-patched")
    parser.add_argument("--restore-dir",
                        help="Specific backup directory to restore from (default: interactive list)")
    parser.add_argument("--restore-full",   action="store_true",
                        help="Include risky partitions (preloader, tee, nvram) in --restore")
    parser.add_argument("--install",        metavar="ID[,ID,...]",
                        help="Module IDs to install with --modules (or 'all'). "
                             "E.g.: lsposed,shamiko,play-integrity-fix")
    parser.add_argument("--remove",         metavar="ID[,ID,...]",
                        help="Package IDs to remove with --debloat (or 'all'). "
                             "E.g.: facebook,linkedin")
    parser.add_argument("--packages",       metavar="PKG[,PKG,...]",
                        help="App package names for --app-backup. "
                             "E.g.: com.example.app,com.other.app")
    parser.add_argument("--apk",            metavar="PATH",
                        help="Path to APK file or split-APK directory for --sideload")
    parser.add_argument("--downgrade",      action="store_true",
                        help="Allow version downgrade when sideloading (adb install -d)")
    parser.add_argument("--watch",          action="store_true",
                        help="Refresh thermal display every 2 seconds (use with --thermal)")
    parser.add_argument("--glyph-enable",   metavar="on|off",
                        help="Toggle Glyph interface (use with --glyph)")
    parser.add_argument("--top-n",          metavar="N", type=int, default=20,
                        help="Number of results for --storage-report (default: 20)")
    parser.add_argument("--include-system", action="store_true",
                        help="Include system apps in --apk-extract")
    parser.add_argument("--package",        metavar="PKG",
                        help="Package name filter for --logcat")
    parser.add_argument("--tag",            metavar="TAG",
                        help="Log tag filter for --logcat")
    parser.add_argument("--level",          metavar="V|D|I|W|E",
                        help="Minimum log level for --logcat (default: all)")
    parser.add_argument("--lines",          metavar="N", type=int, default=500,
                        help="Max log lines for --logcat (default: 500)")
    parser.add_argument("--target",         metavar="system|bootloader|recovery|safe|download|sideload",
                        help="Reboot target (use with --reboot)")
    parser.add_argument("--duration",       metavar="N", type=int, default=30,
                        help="Screen record duration in seconds (use with --screenrecord, max 180)")
    parser.add_argument("--key",            metavar="PROP",
                        help="Property key for --prop-get / --prop-set")
    parser.add_argument("--value",          metavar="VAL",
                        help="Property value for --prop-set")
    parser.add_argument("--profile",        metavar="performance|balanced|powersave",
                        help="CPU governor profile (use with --performance)")
    parser.add_argument("--encrypt",        action="store_true",
                        help="Encrypt partition backup with a password (use with --backup)")
    parser.add_argument("--module-id",      metavar="MODULE",
                        help="Module directory name or ID for --modules-toggle")
    parser.add_argument("--enable",         action="store_true",
                        help="Enable a disabled module (use with --modules-toggle; "
                             "omit to disable)")
    parser.add_argument("--pattern",        metavar="PATTERN",
                        help="Glyph pattern name for --glyph-pattern "
                             "(test, off, pulse, blink, wave)")

    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--backup",         action="store_true",
                      help="Dump all critical partitions from device to local storage (requires root)")
    mode.add_argument("--restore",        action="store_true",
                      help="Flash partitions from a backup back to device (safe set by default)")
    mode.add_argument("--install-magisk", action="store_true",
                      help="Download and install (or update) the latest Magisk APK on device")
    mode.add_argument("--flash-firmware", action="store_true",
                      help="Flash all boot partitions from nothing_archive (replaces OTA)")
    mode.add_argument("--ota-update",     action="store_true",
                      help="Download firmware, auto-patch init_boot with Magisk CLI, and flash "
                           "(one-shot root-preserving update; falls back to manual if no root)")
    mode.add_argument("--unroot",         action="store_true",
                      help="Flash stock patch target to both slots (prepare for OTA)")
    mode.add_argument("--push-for-patch", action="store_true",
                      help="Push stock patch target to /sdcard/Download/ for Magisk patching")
    mode.add_argument("--flash-patched",  action="store_true",
                      help="Pull magisk_patched*.img from device and flash to both slots")
    mode.add_argument("--modules",        action="store_true",
                      help="List recommended Magisk modules with install status "
                           "(use --install to install)")
    mode.add_argument("--debloat",        action="store_true",
                      help="List pre-installed NothingOS bloatware with status "
                           "(use --remove to disable packages; reversible)")
    mode.add_argument("--wifi-adb",       action="store_true",
                      help="Switch to wireless ADB mode and connect automatically "
                           "(USB cable can be unplugged after)")
    mode.add_argument("--verify-backup",  action="store_true",
                      help="Compare live partition hashes against a stored backup "
                           "(use --restore-dir to specify backup)")
    mode.add_argument("--glyph",         action="store_true",
                      help="Show Glyph interface status and zone map "
                           "(use --glyph-enable on/off to toggle)")
    mode.add_argument("--thermal",       action="store_true",
                      help="Show thermal zone temperatures (use --watch for live mode)")
    mode.add_argument("--sideload",      action="store_true",
                      help="Install APK or split-APK directory (use --apk to specify path)")
    mode.add_argument("--app-backup",    action="store_true",
                      help="Backup APK + app data for packages (use --packages to specify)")
    mode.add_argument("--app-restore",   action="store_true",
                      help="Restore apps from a backup created by --app-backup "
                           "(use --restore-dir to specify backup directory)")
    mode.add_argument("--history",        action="store_true",
                      help="Display the flash operation history log")
    mode.add_argument("--storage-report", action="store_true",
                      help="Show top-N largest directories in /data/data/ and /sdcard/ "
                           "(use --top-n to set count)")
    mode.add_argument("--apk-extract",    action="store_true",
                      help="Pull APKs for all user-installed apps "
                           "(use --include-system for system apps too)")
    mode.add_argument("--logcat",         action="store_true",
                      help="Dump logcat buffer to local file "
                           "(use --package, --tag, --level, --lines to filter)")
    mode.add_argument("--bugreport",      action="store_true",
                      help="Generate full adb bugreport ZIP (takes 30-90 seconds)")
    mode.add_argument("--anr-dump",       action="store_true",
                      help="Pull ANR traces and tombstones from /data/anr/ + /data/tombstones/ "
                           "(requires root)")
    mode.add_argument("--battery",        action="store_true",
                      help="Show battery health report: level, status, temperature, cycle count")
    mode.add_argument("--info",           action="store_true",
                      help="Show full device dashboard: Android version, SoC, RAM, storage, IMEI")
    mode.add_argument("--reboot",         action="store_true",
                      help="Reboot to a target (use --target; interactive menu if omitted)")
    mode.add_argument("--screenshot",     action="store_true",
                      help="Capture a screenshot and save it locally")
    mode.add_argument("--screenrecord",   action="store_true",
                      help="Record the screen (use --duration for length, default 30s)")
    mode.add_argument("--permissions",    action="store_true",
                      help="Audit dangerous permissions granted to apps "
                           "(use --package for single-app detail)")
    mode.add_argument("--prop-get",       action="store_true",
                      help="Read system property (use --key for a specific prop; "
                           "all properties if omitted)")
    mode.add_argument("--prop-set",       action="store_true",
                      help="Write system property (requires root; use --key and --value)")
    mode.add_argument("--performance",    action="store_true",
                      help="Set CPU governor profile (use --profile; interactive menu if omitted)")
    mode.add_argument("--adb-pair",       action="store_true",
                      help="Pair a new device via wireless ADB pairing code (Android 11+)")
    mode.add_argument("--modules-status", action="store_true",
                      help="List all installed Magisk modules with enabled/disabled state")
    mode.add_argument("--modules-toggle", action="store_true",
                      help="Enable or disable a Magisk module (use --module-id and --enable)")
    mode.add_argument("--glyph-pattern",  action="store_true",
                      help="Run a Glyph light pattern (use --pattern: test, off, pulse, blink, wave)")
    mode.add_argument("--fix-biometric",  action="store_true",
                      help="Force PIN/password auth instead of fingerprint (workaround for broken sensors)")

    args     = parser.parse_args()
    base_dir = Path(args.base_dir)

    # Reconfigure stdout/stderr to UTF-8 so Unicode in JSON descriptions prints cleanly
    # on Windows terminals that default to cp1252.
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")

    try:
        # Commands that don't require a connected device
        if args.history:
            action_history(base_dir)
            return
        if args.adb_pair:
            action_adb_pair()
            return

        print("Detecting device...")
        device     = detect_device(args.serial)
        device_dir = base_dir / device.codename
        print(f"  Device  : Nothing {device.model}  (slot: {device.current_slot or 'N/A'})")
        print(f"  Storage : {device_dir}")

        if args.backup:
            password = None
            if args.encrypt:
                import getpass
                password = getpass.getpass("Backup encryption password: ")
            action_backup(device, device_dir, password=password)
        elif args.restore:
            password = None
            if args.encrypt:
                import getpass
                password = getpass.getpass("Backup decryption password: ")
            action_restore(device, device_dir, args.restore_dir, args.restore_full,
                           password=password)
        elif args.verify_backup:
            action_verify_backup(device, device_dir, args.restore_dir)
        elif args.debloat:
            action_debloat(device, args.remove)
        elif args.wifi_adb:
            action_wifi_adb(device)
        elif args.glyph:
            action_glyph(device, args.glyph_enable)
        elif args.glyph_pattern:
            action_glyph_pattern(device, args.pattern)
        elif args.thermal:
            action_thermal(device, watch=args.watch)
        elif args.sideload:
            action_sideload(device, args.apk, allow_downgrade=args.downgrade)
        elif args.app_backup:
            action_app_backup(device, device_dir, args.packages)
        elif args.app_restore:
            action_app_restore(device, args.restore_dir)
        elif args.modules:
            action_modules(device, base_dir, args.install)
        elif args.modules_status:
            action_modules_status(device)
        elif args.modules_toggle:
            if not args.module_id:
                raise AdbError("--modules-toggle requires --module-id <MODULE>")
            action_modules_toggle(device, args.module_id, enable=args.enable)
        elif args.storage_report:
            action_storage_report(device, top_n=args.top_n)
        elif args.apk_extract:
            action_apk_extract(device, device_dir, include_system=args.include_system)
        elif args.logcat:
            action_logcat(device, device_dir, package=args.package, tag=args.tag,
                          level=args.level, lines=args.lines)
        elif args.bugreport:
            action_bugreport(device, device_dir)
        elif args.anr_dump:
            action_anr_dump(device, device_dir)
        elif args.battery:
            action_battery(device)
        elif args.info:
            action_info(device)
        elif args.reboot:
            action_reboot(device, args.target)
        elif args.screenshot:
            action_screenshot(device, device_dir)
        elif args.screenrecord:
            action_screenrecord(device, device_dir, duration=args.duration)
        elif args.permissions:
            action_permissions(device, package=args.package)
        elif args.prop_get:
            action_prop_get(device, args.key)
        elif args.prop_set:
            if not args.key or not args.value:
                raise AdbError("--prop-set requires --key <PROP> and --value <VAL>")
            action_prop_set(device, args.key, args.value)
        elif args.performance:
            action_performance(device, args.profile)
        elif args.fix_biometric:
            r = run(["adb", "-s", device.serial, "shell",
                     "locksettings require-strong-auth STRONG_AUTH_REQUIRED_AFTER_USER_LOCKDOWN"])
            if r.returncode == 0:
                print("[OK] Strong auth enforced — PIN/password will be used instead of fingerprint.")
                print("     Effect lasts until next reboot. Run again if needed after restart.")
            else:
                raise AdbError(f"locksettings failed: {r.stderr.strip()}")
        elif args.install_magisk:
            print("Checking Magisk status...")
            ms = check_magisk(device.serial)
            print_magisk_status(ms)
            action_install_magisk(device, base_dir, ms)
        else:
            print("Checking Magisk status...")
            ms = check_magisk(device.serial)

            fw = resolve_firmware(device, base_dir, args.force_download)

            if args.flash_firmware:
                action_flash_firmware(device, fw, base_dir, skip_backup=args.no_backup)
            elif args.ota_update:
                action_ota_update(device, fw, base_dir, skip_backup=args.no_backup)
            elif args.unroot:
                action_unroot(device, fw)
            elif args.push_for_patch:
                action_push_for_patch(device, fw)
            elif args.flash_patched:
                action_flash_patched(device, fw, base_dir, skip_backup=args.no_backup)
            else:
                action_check(device, fw, ms)

    except (FirmwareError, AdbError, FlashError, FastbootTimeoutError, MagiskError) as e:
        print(f"\nERROR: {e}", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        print("\nInterrupted.")
        sys.exit(0)
