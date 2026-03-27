---
name: nothingctl
description: >
  Full control of Nothing phones via ADB: firmware, Magisk root, partitions,
  display, audio, WiFi, network, battery, system monitoring, Glyph interface,
  app management, developer options, notifications, location, and more.
  Use when the user mentions: Nothing Phone (any model), NothingOS, firmware
  flash, Magisk, root, init_boot, Glyph, glyph pattern, thermal, debloat,
  network info, DNS, port forward, wifi scan, audio volume, display settings,
  color profile, battery stats, cache clear, locale, notifications, clipboard,
  process tree, Doze, Essential Space, nothing settings, sideload APK,
  app backup, logcat, bugreport, screenshot, screenrecord.
  Device codenames: spacewar (Phone 1), pong (Phone 2), pacman (Phone 2a),
  galaxian (Phone 3a / 3a Lite), CMF Phone 1.
allowed-tools: Bash
---

# nothingctl Skill

Wraps the `nothingctl` Go binary — a compiled CLI tool for complete Nothing phone management.

**Invocation:** `<skill-dir>/nothingctl [-s <serial>] <command> [flags]`

**Always detect device first** — run `nothingctl root-status` and `nothingctl info` to confirm state.

---

## Commands

### Firmware & Root

| Command | What it does | Req. |
|---------|-------------|------|
| `check-update` | Check nothing_archive for a firmware update — no download | ADB |
| `root-status` | Detect active root manager (Magisk / KernelSU / APatch) | ADB |
| `backup` | Dump 31 partitions → local storage + checksums.sha256 | root |
| `restore` | Flash backed-up partitions via fastboot | Fastboot |
| `verify-backup` | Compare live partition hashes against backup | root |
| `install-magisk` | Download + install/update Magisk APK | ADB |
| `update-magisk` | Update Magisk to latest version | ADB |
| `flash-firmware` | Flash boot partitions from nothing_archive + ARB check | Fastboot |
| `ota-update` | Download + Magisk CLI patch + flash (root preserved) | root + FB |
| `unroot` | Flash stock boot to both slots (removes root) | Fastboot |
| `push-for-patch` | Push stock boot image to /sdcard/Download/ | ADB |
| `flash-patched` | Pull magisk_patched*.img and flash both slots | Fastboot |
| `fix-biometric` | Force PIN auth instead of fingerprint (effect lasts until reboot) | ADB |
| `history` | Flash operation history log (no device needed) | — |

### Magisk Modules & App Management

| Command | What it does | Req. |
|---------|-------------|------|
| `modules` | List recommended Magisk modules + install status | root |
| `modules --install <ids>` | Download + install modules (comma-sep IDs or `all`) | root |
| `modules-status` | All installed Magisk modules with enabled/disabled state | root |
| `modules-toggle --modules <ids> --enable true\|false` | Enable/disable modules | root |
| `debloat` | List/disable NothingOS bloatware | ADB |
| `debloat --remove <ids>` | Disable packages via pm uninstall --user 0 (reversible) | ADB |
| `sideload --apk <path>` | Install APK or split-APK directory | ADB |
| `app-backup` | Backup APK + /data/data for packages (`--packages`) | root |
| `app-restore` | Restore from app-backup | root |
| `app-info --package <pkg>` | Version, SDK, install date, APK path | ADB |
| `kill-app --package <pkg>` | Force-stop app | ADB |
| `launch-app --package <pkg>` | Launch app or deep link (`--deep-link <uri>`) | ADB |
| `package-list` | Export all apps as text/csv/json (`--format`) | ADB |
| `permissions` | Audit dangerous permissions (`--package` for single app) | ADB |

### Device Info & Battery

| Command | What it does | Req. |
|---------|-------------|------|
| `info` | Dashboard: Android version, SoC, RAM, storage, IMEI | ADB |
| `battery` | Level, status, temperature, voltage, cycle count | ADB |
| `battery-stats` | Per-app wakelock drain since last charge | ADB |
| `charging-control --limit N` | Read/set charge limit via sysfs (custom kernel only) | root |
| `reboot` | Reboot to target (`--target`; interactive if omitted) | ADB |
| `screenshot` | Capture screenshot and save locally | ADB |
| `screenrecord` | Record screen (`--duration`, default 30 s) | ADB |

### System Monitoring

| Command | What it does | Req. |
|---------|-------------|------|
| `thermal` | All thermal zone temperatures with ASCII bars (`--watch` for live) | ADB |
| `memory` | RAM usage + top apps by RSS (`--package` for detail, `--watch` live) | ADB |
| `cpu-usage` | CPU core frequencies + top processes (`--top N`, `--watch` live) | ADB |
| `process-tree` | Full process list UID/PID/state (`--package` to filter) | ADB |
| `doze-status` | Doze state + whitelist (`--whitelist-add/--whitelist-remove`) | ADB |
| `logcat` | Dump logcat to file (`--package`, `--tag`, `--level`, `--lines`) | ADB |
| `bugreport` | Full adb bugreport ZIP (30–90 s) | ADB |
| `anr-dump` | Pull /data/anr/ + /data/tombstones/ | root |

### Network & Connectivity

| Command | What it does | Req. |
|---------|-------------|------|
| `network-info` | WiFi SSID, signal, IP, DNS, mobile operator | ADB |
| `dns-set --provider <name>` | Set Private DNS (off/cloudflare/adguard/google/quad9/hostname) | ADB |
| `port-forward` | List/add/remove ADB port forwards (`--local/--remote`, `--clear`) | ADB |
| `wifi-scan` | Nearby WiFi networks sorted by signal | ADB |
| `wifi-profiles` | List saved networks; forget with `--ssid <SSID>` | ADB |
| `wifi-adb` | Switch to wireless ADB and connect automatically | USB |
| `adb-pair --port <port>` | Pair device via wireless ADB code (Android 11+) | — |

### Display & Audio

| Command | What it does | Req. |
|---------|-------------|------|
| `display` | Show/set display settings (`--set brightness/dpi/timeout/rotation/font-scale --value N`) | ADB |
| `color-profile --profile <name>` | Set color mode (natural/vivid/srgb) | ADB |
| `audio` | Stream volumes with ASCII bars; set with `--set <stream> --volume N` | ADB |
| `audio-route` | Active audio output path + connected Bluetooth devices | ADB |

### Storage

| Command | What it does | Req. |
|---------|-------------|------|
| `storage-report` | Top-N largest dirs in /data/data/, /sdcard/ (`--top N`) | ADB |
| `apk-extract` | Pull APKs for all user apps (`--include-system`) | ADB |
| `cache-clear` | Clear caches system-wide (`--package` for single app) | ADB |

### Input & Control

| Command | What it does | Req. |
|---------|-------------|------|
| `input` | Send input: `--tap X,Y` / `--swipe X1,Y1,X2,Y2` / `--text STR` / `--keyevent CODE` | ADB |
| `locale` | Show/set language/timezone/time format (`--lang`, `--timezone`, `--24h`) | ADB |
| `location` | Show/set location mode (`--mode`: off/device/battery/high) | ADB |
| `notifications` | List active notifications (`--package` to filter) | ADB |
| `clipboard` | Read clipboard; write with `--text <text>` | ADB |
| `prop-get` | Read system property (`--key`; all if omitted) | ADB |
| `prop-set --key K --value V` | Write system property | root |
| `performance` | Set CPU governor (`--profile`: performance/balanced/powersave) | root |
| `dev-options` | Developer Options menu; set with `--set/--value` | ADB |
| `screen-always-on` | Stay-awake while charging (`--enable true\|false`) | ADB |

### Nothing-specific

| Command | What it does | Req. |
|---------|-------------|------|
| `glyph` | Glyph package, service state, settings, zone map | ADB |
| `glyph --enable on\|off` | Toggle Glyph interface | ADB |
| `glyph-pattern --pattern <name>` | Run Glyph light pattern (pulse/blink/wave) | ADB |
| `glyph-notify` | Glyph notification config + Hearthstone services | ADB |
| `nothing-settings` | Read/write Nothing settings (`--set/--value`) | ADB |
| `essential-space --enable true\|false` | Toggle Essential Space — Phone (2+) only | ADB |

---

## Global flags

| Flag | Description |
|------|-------------|
| `-s, --serial <s>` | Target specific device by ADB serial |
| `--base-dir <p>` | Override `~/tools/Nothing` storage root |
| `--no-backup` | Skip automatic backup before flash |
| `--force-download` | Re-download firmware even if cached |

---

## Known behaviors & gotchas

- **`screenrecord` on Phone (1)**: auto-scales to 720p (encoder error -38 on stock kernel)
- **`fix-biometric`**: effect is per-reboot only — re-run after restart
- **`charging-control`**: stock kernel does not expose sysfs charge limit node
- **`clipboard` read**: blocked by Android 10+ OS policy; `--text` to write works
- **`battery-stats` wakelock data**: empty immediately after boot/charge
- **`wifi-scan` RSSI format**: Android 12+ returns `-87(0:-93/1:-89)` — leading value extracted

---

## Core workflows

### OTA update with root preserved

```bash
nothingctl ota-update
# Downloads firmware → Magisk CLI patches init_boot → flashes both A/B slots
```

### Manual firmware update

```bash
nothingctl push-for-patch   # push stock boot to device
# user patches in Magisk app on device
nothingctl flash-patched    # flash patched image
```

### Safety backup before flash

```bash
nothingctl backup
nothingctl flash-firmware
```

---

## Device-specific notes

### Nothing Phone (3a) Lite (Galaxian / mt6878)
- **SoC**: MediaTek Dimensity 7300 Pro — NOT Snapdragon
- **Glyph package**: `com.nothing.hearthstone`
- **Thermal**: MediaTek zone names; `-274000` millidegree sensors filtered automatically
- **Essential Space / charging-control**: not available

### Phone (1) / (2) — Legacy Glyph
- **Glyph package**: `ly.nothing.glyph.service`
- **Glyph toggle**: `settings put secure glyph_interface_enable 0|1`

### Glyph zone counts
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

Checked before `flash-firmware` and `ota-update`. Reads `rollback_index` from
`vbmeta.img` (offset 112, big-endian uint64). Blocks flash if firmware ARB index
< device ARB index — would cause permanent boot loop.

---

## Storage layout

```
~/tools/Nothing/
  <Codename>/
    <Codename>_<Tag>/         ← firmware archive
    Backups/
      partition-backup/backup_<ts>/   ← 31 × .img + checksums.sha256
      apk_extract/
      app_backups/<ts>/               ← *_data.tar.gz per package
    logs/
    bugreports/
    diagnostics/<ts>/anr|tombstones/
  modules/                            ← Magisk modules (shared)
  flash_history.json
```

---

## Safety rules

1. Run `root-status` + `info` first — confirm state before any flash.
2. `backup` before every flash — auto-runs if root available.
3. Never `restore` to a different device unit than the one backed up from.
4. Use `--serial` when multiple devices are connected.
5. Phone (2+): patch target is `init_boot.img`. Phone (1): `boot.img`.
6. ARB is fuse-burned — the tool enforces this automatically.
