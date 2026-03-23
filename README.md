# nothing-firmware

A Python CLI tool for managing firmware, Magisk root, and device backups on Nothing phones.

## Supported devices

| Device | Codename | SoC |
|--------|----------|-----|
| Nothing Phone (1) | Spacewar | Snapdragon 778G+ |
| Nothing Phone (2) | Pong | Snapdragon 8+ Gen 1 |
| Nothing Phone (2a) | Pacman | Dimensity 7200 Pro |
| Nothing Phone (3a) | Galaxian | Snapdragon 7s Gen 3 |
| Nothing Phone (3a) Lite | Galaxian | Dimensity 7300 Pro |
| CMF Phone 1 | — | Dimensity 7300 |

---

## Requirements

- Python 3.10+
- [ADB](https://developer.android.com/tools/releases/platform-tools) in `PATH`
- [Fastboot](https://developer.android.com/tools/releases/platform-tools) in `PATH`
- USB debugging enabled on device
- [Magisk](https://github.com/topjohnwu/Magisk) with **Superuser access → Apps and ADB** for root operations

No third-party Python packages required — stdlib only.

---

## Installation

```bash
git clone https://github.com/Limplom/nothingctl
cd nothing-firmware
python nothingctl.py
```

---

## Claude Code integration (optional)

This repo includes a `SKILL.md` — a ready-to-use skill for [Claude Code](https://claude.ai/claude-code) (Anthropic's AI CLI).

If you use Claude Code, you can control your Nothing phone through natural language without typing flags manually:

```
> check firmware and magisk status on my nothing phone
> backup all partitions
> debloat my nothing phone (show me what can be removed)
> install lsposed and shamiko
```

Claude will call the correct `nothingctl.py` commands automatically.

**Setup:** just point Claude Code at this directory — the `SKILL.md` is picked up automatically.

---

## Quick start

```bash
# Check firmware version + Magisk status (always run this first)
python nothingctl.py
```

Prints: current firmware vs. latest available, Magisk version, active slot, and recommended next steps.

---

## All modes

### Firmware & Root

| Flag | What it does | Requires |
|------|-------------|----------|
| *(default)* | Check firmware + Magisk status, download if newer | ADB |
| `--backup` | Dump 31 partitions via root dd → local storage + checksums | ADB root |
| `--restore` | Flash backed-up partitions back to device | Fastboot |
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
| `--app-backup` | Backup APK + app data for specified packages | ADB root |
| `--app-restore` | Restore APK + data from a previous `--app-backup` | ADB root |

### Storage & Logs

| Flag | What it does | Requires |
|------|-------------|----------|
| `--storage-report` | Top-N largest dirs in /data/data/, /sdcard/Android/data/, /sdcard/ | ADB (root for /data/data/) |
| `--apk-extract` | Pull APKs for all user-installed apps | ADB |
| `--logcat` | Dump logcat buffer to local file | ADB |
| `--bugreport` | Full `adb bugreport` ZIP (30–90 s) | ADB |
| `--anr-dump` | Pull ANR traces + tombstones from /data/anr/ + /data/tombstones/ | ADB root |

### Device Diagnostics

| Flag | What it does | Requires |
|------|-------------|----------|
| `--wifi-adb` | Switch to wireless ADB and connect automatically | ADB (USB) |
| `--glyph` | Show Glyph package, service state, settings, zone map | ADB |
| `--glyph --glyph-enable on\|off` | Toggle Glyph interface | ADB root |
| `--thermal` | Show all thermal zone temperatures with ASCII bars | ADB root |
| `--thermal --watch` | Refresh thermal display every 2 s (live mode) | ADB root |
| `--history` | Display flash operation history (no device needed) | — |

---

## Modifier flags

| Flag | Effect |
|------|--------|
| `--serial <s>` | Target a specific device serial |
| `--base-dir <p>` | Override storage root (default: `~/tools/Nothing`) |
| `--force-download` | Re-download firmware even if cached |
| `--no-backup` | Skip auto-backup before flash operations |
| `--restore-dir <p>` | Skip backup picker, use this directory |
| `--restore-full` | Include risky partitions in restore (preloader, tee, nvram) |
| `--packages <p>` | Comma-separated package names for `--app-backup` |
| `--apk <path>` | APK or split-APK directory for `--sideload` |
| `--downgrade` | Allow version downgrade when sideloading |
| `--install <ids>` | Module IDs for `--modules` (comma-sep or `all`) |
| `--remove <ids>` | Package IDs for `--debloat` (comma-sep or `all`) |
| `--watch` | Live refresh for `--thermal` |
| `--glyph-enable on\|off` | Toggle argument for `--glyph` |
| `--top-n <N>` | Result count for `--storage-report` (default: 20) |
| `--include-system` | Include system apps in `--apk-extract` |
| `--package <pkg>` | Package filter for `--logcat` |
| `--tag <tag>` | Log tag filter for `--logcat` |
| `--level V\|D\|I\|W\|E` | Minimum log level for `--logcat` |
| `--lines <N>` | Max log lines for `--logcat` (default: 500) |

---

## Common workflows

### OTA update with root preserved (one command)

```bash
python nothingctl.py --ota-update
# Downloads firmware → patches init_boot with Magisk CLI → flashes both slots
```

### Manual firmware update

```bash
python nothingctl.py                   # check + download
python nothingctl.py --backup          # safety backup
python nothingctl.py --flash-firmware  # flash boot partitions
python nothingctl.py --push-for-patch  # push image to device
# patch in Magisk app
python nothingctl.py --flash-patched   # flash patched image
```

### App backup

```bash
# Specific packages:
python nothingctl.py --app-backup --packages com.whatsapp,org.telegram.messenger

# All user apps:
python nothingctl.py --app-backup --packages \
  "$(adb shell pm list packages -3 | sed 's/package://g' | tr '\n' ',')"
```

### Logcat with filters

```bash
python nothingctl.py --logcat --package com.discord --level W
python nothingctl.py --logcat --tag AudioFlinger --level E --lines 200
```

---

## Storage layout

All data is stored under `~/tools/Nothing/<Codename>/`:

```
~/tools/Nothing/
  Galaxian/                        ← per-device directory
    Backups/
      partition-backup/
        backup_<timestamp>/        ← 31 × .img + checksums.sha256
      apk_extract/                 ← APKs (shared by --apk-extract and --app-backup)
      app_backups/
        <timestamp>/               ← *_data.tar.gz per package (no APK duplication)
    logs/                          ← logcat dumps
    bugreports/                    ← bugreport ZIPs
    diagnostics/
      <timestamp>/anr/             ← ANR traces
      <timestamp>/tombstones/      ← crash tombstones
    Galaxian_B4.0-<tag>/           ← firmware archive
  modules/                         ← Magisk modules (shared across devices)
  flash_history.json               ← log of all flash operations
```

---

## ARB (Anti-Rollback Protection)

Checked automatically before `--flash-firmware` and `--ota-update`. Reads the `rollback_index` from `vbmeta.img` and compares it against the device's current ARB index. **A firmware with a lower ARB index than the device will cause a permanent boot loop** — the tool blocks this automatically.

---

## Safety rules

1. Always run the default mode first to confirm state before any flash.
2. `--backup` before every flash — auto-backup runs if root is available.
3. Never use `--restore-full` unless restoring to the exact same device unit.
4. Use `--serial` when multiple devices are connected.
5. GKI 2.0 devices (Phone 2, 2a, 3a, 3a Lite): patch target is `init_boot.img`. Phone (1): `boot.img`.
6. ARB is fuse-burned and permanent — the tool enforces this automatically.

---

## Device-specific notes

### Nothing Phone (3a) Lite — MediaTek

- Glyph package: `com.nothing.hearthstone` (not `ly.nothing.glyph.service`)
- Glyph toggle: via `am stopservice/startservice com.nothing.thirdparty/.GlyphService` (root)
- Glyph settings namespace: `global` (`glyph_long_torch_enable`, `glyph_pocket_mode_state`, `glyph_screen_upward_state`)
- Thermal: MediaTek zone names (`soc_max`, `cpu-big-core*`, `apu`, `shell_*`); unpopulated sensors (`-274000` millidegrees) are filtered automatically

### Phone (1) / (2) — Legacy Glyph

- Glyph package: `ly.nothing.glyph.service`
- Glyph toggle: `settings put secure glyph_interface_enable 0|1`

---

## License

MIT
