---
name: nothingctl
description: >
  Manage firmware, root (Magisk), and partition backups on Nothing devices
  (Nothing Phone 1, 2, 2a, 3a, 3a Lite, CMF Phone 1, etc.). Use this skill when
  the user wants to check or flash firmware, root or unroot a Nothing device,
  backup/restore partitions, install or update Magisk, maintain root across OTA
  updates, manage Magisk modules, debloat NothingOS, monitor thermals, control
  Glyph interface, sideload APKs, or backup/restore app data.
  Triggers on: "nothing phone", "nothing firmware", "nothing archive",
  "NothingOS", "spacewar", "pong", "pacman", "galaxian", "flash nothing",
  "root nothing", "magisk patched", "init_boot patch", "nothing backup",
  "nothing restore", "glyph", "thermal", "debloat", "magisk module".
allowed-tools: Bash
---

# nothingctl Skill

This skill wraps `nothingctl.py`, a Python CLI tool that automates
firmware management, Magisk root maintenance, and device diagnostics for
Nothing phones.

**Skill directory:** the directory containing this SKILL.md file.

---

## Quick-start — always run this first

```bash
# Detect device and check firmware + Magisk status
python <skill-dir>/nothingctl.py [--serial <serial>]
```

Prints: current firmware vs. latest, Magisk status, recommended next steps.

---

## All modes

### Firmware & Root

| Flag | What it does | Requires |
|------|-------------|----------|
| *(default)* | Check firmware + Magisk status, download if newer | ADB |
| `--backup` | Dump 31 partitions via root dd → local storage + checksums.sha256 | ADB root |
| `--restore` | Flash backed-up partitions via fastboot | Fastboot |
| `--verify-backup` | Compare live partition hashes against backup checksums | ADB root |
| `--install-magisk` | Download + install/update Magisk APK on device | ADB |
| `--flash-firmware` | Flash all boot partitions from nothing_archive + ARB check | Fastboot |
| `--ota-update` | One-shot: download + Magisk CLI patch + flash (root preserved) | ADB root + Fastboot |
| `--unroot` | Flash stock boot to both slots (removes root) | Fastboot |
| `--push-for-patch` | Push stock boot image to /sdcard/Download/ | ADB |
| `--flash-patched` | Pull magisk_patched*.img and flash both slots | Fastboot |

### Modules & Apps

| Flag | What it does | Requires |
|------|-------------|----------|
| `--modules` | List recommended Magisk modules with install status + version diff | ADB root |
| `--modules --install <ids>` | Download + install modules (comma-sep IDs or `all`) | ADB root |
| `--debloat` | List pre-installed NothingOS bloatware with status | ADB |
| `--debloat --remove <ids>` | Disable packages via `pm uninstall --user 0` (reversible) | ADB |
| `--sideload --apk <path>` | Install APK or split-APK directory via ADB | ADB |
| `--app-backup` | Backup APK + /data/data via root tar (interactive or --packages) | ADB root |
| `--app-restore` | Restore APK + data from --app-backup directory | ADB root |

### Device Diagnostics

| Flag | What it does | Requires |
|------|-------------|----------|
| `--wifi-adb` | Switch to wireless ADB mode and connect automatically | ADB (USB) |
| `--glyph` | Show Glyph package, service state, feature settings, zone map | ADB |
| `--glyph --glyph-enable on\|off` | Toggle Glyph interface (GlyphService stop/start on 3a/Lite) | ADB root |
| `--thermal` | Show all thermal zone temperatures with ASCII bars | ADB root |
| `--thermal --watch` | Refresh thermal display every 2s (live mode) | ADB root |
| `--history` | Display flash operation history log (no device needed) | — |

### Storage & Logs

| Flag | What it does | Requires |
|------|-------------|----------|
| `--storage-report` | Top-N largest dirs in /data/data/, /sdcard/Android/data/, /sdcard/ | ADB (root for /data/data/) |
| `--storage-report --top-n <N>` | Change result count (default: 20) | ADB |
| `--apk-extract` | Pull APKs for all user-installed apps → base_dir/apk_extract/ | ADB |
| `--apk-extract --include-system` | Include system apps in the extract | ADB |
| `--logcat` | Dump logcat buffer to base_dir/logs/ | ADB |
| `--logcat --package <pkg>` | Filter by app (resolved to PID via pidof) | ADB |
| `--logcat --tag <tag> --level <V\|D\|I\|W\|E>` | Filter by log tag and/or minimum level | ADB |
| `--logcat --lines <N>` | Max lines to capture (default: 500) | ADB |
| `--bugreport` | Full adb bugreport ZIP → base_dir/bugreports/ (30–90 s) | ADB |
| `--anr-dump` | Pull /data/anr/ + /data/tombstones/ → base_dir/diagnostics/ | ADB root |

### Modifier flags

| Flag | Effect |
|------|--------|
| `--serial <s>` | Target a specific device serial |
| `--base-dir <p>` | Override `~/tools/Nothing` storage root |
| `--force-download` | Re-download firmware even if cached |
| `--no-backup` | Skip auto-backup before flash operations |
| `--restore-dir <p>` | Skip backup/restore picker, use this directory |
| `--restore-full` | Include risky partitions (preloader, tee, nvram) in restore |
| `--packages <p>` | Comma-separated package names for `--app-backup` |
| `--apk <path>` | APK file or split-APK directory for `--sideload` |
| `--downgrade` | Allow version downgrade when sideloading (`adb install -d`) |
| `--watch` | Live refresh mode for `--thermal` |
| `--glyph-enable on\|off` | Toggle argument for `--glyph` |
| `--top-n <N>` | Result count for `--storage-report` (default: 20) |
| `--include-system` | Include system apps in `--apk-extract` |
| `--package <pkg>` | Package filter for `--logcat` |
| `--tag <tag>` | Log tag filter for `--logcat` |
| `--level <V\|D\|I\|W\|E>` | Minimum log level for `--logcat` |
| `--lines <N>` | Max log lines for `--logcat` (default: 500) |

---

## Standard workflows

### One-shot OTA update (root preserved)

```bash
python <skill-dir>/nothingctl.py --ota-update
# Downloads firmware → Magisk CLI patches init_boot → flashes both slots
# Falls back to manual --push-for-patch flow if no root
```

### Manual firmware update

```bash
python <skill-dir>/nothingctl.py                   # check + download
python <skill-dir>/nothingctl.py --backup          # safety backup
python <skill-dir>/nothingctl.py --flash-firmware  # flash boot partitions
python <skill-dir>/nothingctl.py --push-for-patch  # push image to device
# user patches in Magisk app
python <skill-dir>/nothingctl.py --flash-patched   # flash patched image
```

### OTA method (device updates itself)

```bash
python <skill-dir>/nothingctl.py --unroot          # remove root for OTA
# user applies OTA on device
python <skill-dir>/nothingctl.py --push-for-patch
# user patches in Magisk app
python <skill-dir>/nothingctl.py --flash-patched
```

### App backup

```bash
# Interactive selection:
python <skill-dir>/nothingctl.py --app-backup
# Specific packages:
python <skill-dir>/nothingctl.py --app-backup --packages com.whatsapp,org.telegram.messenger
# All user apps:
python <skill-dir>/nothingctl.py --app-backup --packages "$(adb shell pm list packages -3 | sed 's/package://g' | tr '\n' ',')"
```

---

## Device-specific notes

### Nothing Phone (3a) Lite (A001T / Galaxian)
- **SoC**: MediaTek Dimensity 7300 Pro (mt6878) — NOT Snapdragon
- **Glyph package**: `com.nothing.hearthstone` (not `ly.nothing.glyph.service`)
- **Glyph toggle**: via `am stopservice/startservice com.nothing.thirdparty/.GlyphService` (root)
- **Glyph settings**: in `global` namespace (`glyph_long_torch_enable`, `glyph_pocket_mode_state`, `glyph_screen_upward_state`)
- **Thermal**: MediaTek zone names (`soc_max`, `cpu-big-core*`, `apu`, `shell_*`); unpopulated sensors report `-274000` millidegrees and are filtered automatically

### Phone (1) / (2) — Legacy Glyph
- **Glyph package**: `ly.nothing.glyph.service`
- **Glyph toggle**: `settings put secure glyph_interface_enable 0|1`

### Glyph zone counts by model
| Model | Zones |
|-------|-------|
| Phone (1) | 5 (Camera, Diagonal, Battery dot, Battery bar, USB) |
| Phone (2) | 7 (Camera top/bottom, Diagonal, Battery left/right, USB, Notification) |
| Phone (2a) | 3 (Camera, Battery, Bottom strip) |
| Phone (3a) | 4 (Camera top/bottom, Battery, Bottom strip) |
| Phone (3a) Lite | 2 (Camera, Bottom strip) |
| CMF Phone 1 | 2 (Ring, Dot) |

---

## ARB (Anti-Rollback Protection)

Automatically checked before `--flash-firmware` and `--ota-update`.
Reads `rollback_index` from vbmeta.img at byte offset 112 (big-endian uint64).
Raises `FirmwareError` if firmware ARB index < device ARB index (would brick).

---

## Flash history

Every successful `--flash-firmware` and `--ota-update` is logged to
`~/tools/Nothing/flash_history.json`. View with:

```bash
python <skill-dir>/nothingctl.py --history
```

---

## Firmware storage layout

```
~/tools/Nothing/
  <Codename>/                          ← per-device directory (e.g. Galaxian, Pong)
    <Codename>_<Tag>/                  ← firmware archive (e.g. Galaxian_B4.0-240901)
      init_boot.img                    ← Magisk patch target (GKI 2.0 — Phone 2+)
      boot.img                         ← Magisk patch target (legacy — Phone 1)
      vbmeta.img                       ← ARB index source
      ...
    Backups/
      partition-backup/
        backup_<timestamp>/            ← 31 × .img + checksums.sha256
      apk_extract/                     ← flat APK store (shared by --apk-extract + --app-backup)
        <pkg>.apk
      app_backups/
        <timestamp>/                   ← data-only tarballs per app
          <pkg>_data.tar.gz
    logs/
      logcat_<label>_<ts>.txt          ← logcat dumps (--logcat)
    bugreports/
      bugreport_<serial>_<ts>.zip      ← bugreport ZIPs (--bugreport)
    diagnostics/
      <timestamp>/anr/                 ← ANR traces (--anr-dump)
      <timestamp>/tombstones/          ← crash tombstones (--anr-dump)
  modules/                             ← Magisk modules, shared across devices
    <module-id>/<tag>/
  flash_history.json                   ← flash operation log
```

---

## Package structure

```
nothingctl/
  nothingctl.py   ← entry point
  modules.json          ← Magisk module definitions (edit to add/remove)
  debloat.json          ← bloatware package list (edit to add/remove)
  nothing/
    __init__.py
    exceptions.py       — AdbError, MagiskError, FirmwareError, FlashError, FastbootTimeoutError
    models.py           — MagiskStatus, BootTarget, DeviceInfo, FirmwareState
    device.py           — ADB/fastboot wrappers, detect_device(), run()
    firmware.py         — GitHub API, download, extraction, firmware resolution
    backup.py           — partition backup + restore + verify + checksums
    magisk.py           — Magisk status check, install/update
    arb.py              — Anti-Rollback Protection check (vbmeta parsing)
    modules.py          — Magisk module list, GitHub download, install
    debloat.py          — NothingOS bloatware list + disable/restore
    wifi_adb.py         — Wireless ADB setup
    glyph.py            — Glyph interface diagnostics + toggle (multi-model)
    thermal.py          — Thermal zone monitor (Snapdragon + MediaTek)
    sideload.py         — APK / split-APK sideload
    app_backup.py       — Per-app APK + data backup and restore
    storage.py          — Storage report + APK extraction
    diagnostics.py      — Logcat dump, bugreport, ANR/tombstone collection
    history.py          — Flash history log
    cli.py              — all action_* functions + argparse main()
```

---

## Safety rules

1. Always run default mode first to confirm firmware + Magisk state before any flash.
2. `--backup` before any flash operation (auto-backup runs if root is available).
3. Never use `--restore-full` unless restoring to the exact same device unit.
4. Check `--serial` when multiple devices are connected.
5. GKI 2.0 devices (Phone 2, 2a, 3a, 3a Lite): patch target is `init_boot.img`. Phone 1: `boot.img`.
6. ARB is permanent (eFuse) — a firmware with lower ARB index than the device will cause a boot loop. The tool blocks this automatically.
