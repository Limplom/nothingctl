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

Wraps `nothingctl.py` — a Python CLI tool for complete Nothing phone management.

**Invocation:** `python <skill-dir>/nothingctl.py [--serial <serial>] <mode> [modifiers]`

**Always detect device first** (no args) — confirms firmware + Magisk state.

---

## Modes

### Firmware & Root

| Flag | What it does | Req. |
|------|-------------|------|
| *(default)* | Check firmware + Magisk status, download if newer | ADB |
| `--backup` | Dump 31 partitions → local storage + checksums.sha256 | root |
| `--restore` | Flash backed-up partitions via fastboot | Fastboot |
| `--verify-backup` | Compare live partition hashes against backup | root |
| `--install-magisk` | Download + install/update Magisk APK | ADB |
| `--flash-firmware` | Flash boot partitions from nothing_archive + ARB check | Fastboot |
| `--ota-update` | Download + Magisk CLI patch + flash (root preserved) | root + FB |
| `--unroot` | Flash stock boot to both slots (removes root) | Fastboot |
| `--push-for-patch` | Push stock boot image to /sdcard/Download/ | ADB |
| `--flash-patched` | Pull magisk_patched*.img and flash both slots | Fastboot |
| `--fix-biometric` | Force PIN auth instead of fingerprint (workaround for broken sensor blocking Magisk grants; effect lasts until reboot) | ADB |

### Magisk Modules & App Management

| Flag | What it does | Req. |
|------|-------------|------|
| `--modules` | List recommended Magisk modules + install status | root |
| `--modules-status` | All installed Magisk modules with enabled/disabled state | root |
| `--modules-toggle` | Enable/disable module (use `--module-id`, `--enable`) | root |
| `--debloat` | List/disable NothingOS bloatware (use `--remove ids`) | ADB |
| `--sideload` | Install APK or split-APK directory (use `--apk`) | ADB |
| `--app-backup` | Backup APK + /data/data for packages (use `--packages`) | root |
| `--app-restore` | Restore from --app-backup (use `--restore-dir`) | root |
| `--app-info` | Version, SDK, install date, APK path (use `--package`) | ADB |
| `--kill-app` | Force-stop app (use `--package`; `--clear-cache` wipes data) | ADB |
| `--launch-app` | Launch app or deep link (use `--package` or `--intent`) | ADB |
| `--package-list` | Export all apps as text/csv/json (use `--format`, `--output`) | ADB |
| `--permissions` | Audit dangerous permissions (use `--package` for single app) | ADB |

### Device Info & Battery

| Flag | What it does | Req. |
|------|-------------|------|
| `--info` | Dashboard: Android version, SoC, RAM, storage, IMEI | ADB |
| `--battery` | Level, status, temperature, voltage, cycle count | ADB |
| `--battery-stats` | Per-app wakelock drain since last charge + sysfs cycle count | ADB |
| `--charging-control` | Read/set charge limit via sysfs (use `--limit N`; custom kernel only) | root |
| `--reboot` | Reboot to target (use `--target`; interactive if omitted) | ADB |
| `--screenshot` | Capture screenshot and save locally | ADB |
| `--screenrecord` | Record screen (use `--duration`, default 30 s) | ADB |

### System Monitoring

| Flag | What it does | Req. |
|------|-------------|------|
| `--thermal` | All thermal zone temperatures with ASCII bars (`--watch` for live) | ADB |
| `--memory` | RAM usage + top apps by RSS (`--package` for detail, `--watch` live) | ADB |
| `--cpu-usage` | CPU core frequencies + top processes (`--top-n`, `--watch` live) | ADB |
| `--process-tree` | Full process list UID/PID/state (`--package` to filter) | ADB |
| `--doze-status` | Doze state + whitelist (`--whitelist-add/--whitelist-remove`) | ADB |

### Network & Connectivity

| Flag | What it does | Req. |
|------|-------------|------|
| `--network-info` | WiFi SSID, signal, IP, DNS, mobile operator | ADB |
| `--dns-set` | Show/set Private DNS (use `--provider`: off/cloudflare/adguard/google/quad9/hostname) | ADB |
| `--port-forward` | List/add/remove ADB port forwards (`--local/--remote`, `--clear`) | ADB |
| `--wifi-scan` | Nearby WiFi networks sorted by signal | ADB |
| `--wifi-profiles` | List saved networks; forget with `--forget <SSID\|ID>` | ADB |
| `--wifi-adb` | Switch to wireless ADB and connect automatically | USB |
| `--adb-pair` | Pair device via wireless ADB code (Android 11+) | — |

### Display & Audio

| Flag | What it does | Req. |
|------|-------------|------|
| `--display` | Show/set display settings (use `--key/--value`; keys: brightness/brightness_auto/dpi/timeout/rotation/rotation_auto/font_scale) | ADB |
| `--color-profile` | Show/set color mode + night light (use `--mode`: natural/vivid/custom) | ADB |
| `--audio` | Stream volumes with ASCII bars; set with `--stream` + `--volume` | ADB |
| `--audio-route` | Active audio output path + connected Bluetooth devices | ADB |

### Storage & Logs

| Flag | What it does | Req. |
|------|-------------|------|
| `--storage-report` | Top-N largest dirs in /data/data/, /sdcard/ (use `--top-n`) | ADB |
| `--apk-extract` | Pull APKs for all user apps → base_dir/apk_extract/ (`--include-system`) | ADB |
| `--cache-clear` | Clear caches system-wide via pm trim-caches (`--package` for single) | ADB |
| `--logcat` | Dump logcat to file (use `--package`, `--tag`, `--level`, `--lines`) | ADB |
| `--bugreport` | Full adb bugreport ZIP (30–90 s) | ADB |
| `--anr-dump` | Pull /data/anr/ + /data/tombstones/ | root |
| `--history` | Flash operation history log (no device needed) | — |

### Input & Control

| Flag | What it does | Req. |
|------|-------------|------|
| `--input` | Send input: `--tap X,Y` / `--swipe X1,Y1,X2,Y2[,ms]` / `--text STR` / `--keyevent CODE` | ADB |
| `--locale` | Show/set language/timezone/time format (`--lang`, `--timezone`, `--hour24`) | ADB |
| `--location` | Show/set location mode (`--mode`: off/gps/battery/on) + last position + app perms | ADB |
| `--notifications` | List active notifications (`--package` to filter) | ADB |
| `--clipboard` | Read clipboard; set with `--clip-text` (read blocked Android 10+ by OS) | ADB |
| `--prop-get` | Read system property (`--key`; all if omitted) | ADB |
| `--prop-set` | Write system property (`--key` and `--value`) | root |
| `--performance` | Set CPU governor (`--profile`: performance/balanced/powersave) | root |
| `--dev-options` | Developer Options menu; set with `--key/--value` | ADB |
| `--screen-always-on` | Stay-awake while charging (`--screen-on on\|off`) | ADB |

### Nothing-specific

| Flag | What it does | Req. |
|------|-------------|------|
| `--glyph` | Glyph package, service state, settings, zone map (`--glyph-enable on\|off`) | ADB |
| `--glyph-pattern` | Run Glyph light pattern (use `--pattern`: test/off/pulse/blink/wave) | ADB |
| `--glyph-notify` | Glyph notification config + Hearthstone services | ADB |
| `--nothing-settings` | Read/write Nothing settings (`--ns-key namespace:key`, `--ns-value`) | ADB |
| `--essential-space` | Show/toggle Essential Space — Phone (2+) only (`--essential-enable`) | ADB |

---

## Key modifier flags

| Flag | Used with |
|------|-----------|
| `--serial <s>` | All — target specific device |
| `--package <pkg>` | `--logcat`, `--app-info`, `--kill-app`, `--launch-app`, `--permissions`, `--memory`, `--notifications`, `--cache-clear`, `--process-tree` |
| `--watch` | `--thermal`, `--memory`, `--cpu-usage` |
| `--top-n <N>` | `--storage-report`, `--cpu-usage` (default: 20) |
| `--include-system` | `--apk-extract`, `--package-list` |
| `--format text\|csv\|json` + `--output <path>` | `--package-list` |
| `--restore-dir <p>` | `--restore`, `--verify-backup`, `--app-restore` |
| `--restore-full` | `--restore` (includes preloader/tee/nvram — same device only) |
| `--no-backup` | `--flash-firmware`, `--ota-update`, `--flash-patched` |
| `--duration <N>` | `--screenrecord` (default: 30 s, max: 180 s) |
| `--target <t>` | `--reboot` (system/bootloader/recovery/safe/download/sideload) |
| `--key` / `--value` | `--prop-get/set`, `--dev-options`, `--display` |
| `--stream <name\|N>` + `--volume <N>` | `--audio` (streams: voice/system/ring/media/alarm/notification) |
| `--mode <m>` | `--color-profile` (natural/vivid/custom), `--location` (off/gps/battery/on) |
| `--provider <p>` | `--dns-set` (off/cloudflare/adguard/google/quad9/hostname) |
| `--forget <SSID\|ID>` | `--wifi-profiles` |
| `--lang` / `--timezone` / `--hour24` | `--locale` |
| `--whitelist-add/--whitelist-remove` | `--doze-status` |
| `--clip-text <text>` | `--clipboard` |
| `--limit <N>` | `--charging-control` (20–100%; custom kernel only) |
| `--module-id <id>` + `--enable` | `--modules-toggle` |
| `--pattern <name>` | `--glyph-pattern` |
| `--ns-key <ns:key>` + `--ns-value <val>` | `--nothing-settings` |
| `--install <ids>` | `--modules` (comma-sep IDs or `all`) |
| `--remove <ids>` | `--debloat` (comma-sep IDs or `all`) |
| `--packages <p>` | `--app-backup` (comma-sep package names) |
| `--apk <path>` | `--sideload` |
| `--downgrade` | `--sideload` |
| `--encrypt` | `--backup` |
| `--clear-cache` | `--kill-app` (also wipes app data) |
| `--intent <uri>` | `--launch-app` |
| `--screen-on on\|off` | `--screen-always-on` |
| `--tap/--swipe/--text/--keyevent` | `--input` |
| `--essential-enable/--no-essential-enable` | `--essential-space` |
| `--glyph-enable on\|off` | `--glyph` |
| `--base-dir <p>` | Override `~/tools/Nothing` storage root |
| `--force-download` | Re-download firmware even if cached |

---

## Known behaviors & gotchas

- **`--screenrecord` on Phone (1)**: auto-scales to 720p (1080x2400 causes MediaCodec encoder error -38 on stock kernel)
- **`--fix-biometric`**: effect is per-reboot only — re-run after restart
- **`--charging-control`**: stock kernel does not expose sysfs charge limit node; shows clear "not supported" message on both Nothing Phone (1) and (3a) Lite
- **`--color-profile` color mode**: returns n/a on stock Nothing OS (uses proprietary display keys not mapped to `display_color_mode`)
- **`--clipboard` read**: blocked by Android 10+ OS policy; `--clip-text` to write works
- **`--battery-stats` wakelock data**: only available after device has been in use since last charge (empty immediately after boot/charge)
- **`--wifi-scan` RSSI format**: Android 12+ returns `-87(0:-93/1:-89)` — tool extracts the leading value
- **Multi-SIM operator** (Phone 1 with no SIM): `getprop` returns comma-separated values; tool picks first non-empty entry
- **`--doze-status` whitelist**: Android format is `system-excidle,pkg.name,uid` — deduplicated automatically

---

## Core workflows

### OTA update with root preserved

```bash
python <skill-dir>/nothingctl.py --ota-update
# Downloads + Magisk CLI patches init_boot + flashes both A/B slots
```

### Manual firmware update (no root)

```bash
python <skill-dir>/nothingctl.py --push-for-patch  # push stock boot to device
# user patches in Magisk app
python <skill-dir>/nothingctl.py --flash-patched   # flash patched image
```

### Broken fingerprint blocking Magisk

```bash
python <skill-dir>/nothingctl.py --fix-biometric   # force PIN auth (temporary)
```

---

## Device-specific notes

### Nothing Phone (3a) Lite (Galaxian / mt6878)
- **SoC**: MediaTek Dimensity 7300 Pro — NOT Snapdragon
- **Glyph package**: `com.nothing.hearthstone`
- **Glyph toggle**: `am stopservice/startservice com.nothing.thirdparty/.GlyphService` (root)
- **Glyph settings namespace**: `global` (not `secure`)
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

Checked before `--flash-firmware` and `--ota-update`. Reads `rollback_index` from
`vbmeta.img` (offset 112, big-endian uint64). Blocks flash if firmware ARB index
< device ARB index — would cause permanent boot loop.

---

## Storage layout

```
~/tools/Nothing/
  <Codename>/
    <Codename>_<Tag>/         ← firmware archive (init_boot.img / boot.img / vbmeta.img)
    Backups/
      partition-backup/backup_<ts>/   ← 31 × .img + checksums.sha256
      apk_extract/                    ← --apk-extract and --app-backup APKs
      app_backups/<ts>/               ← *_data.tar.gz per package
    logs/                             ← --logcat
    bugreports/                       ← --bugreport
    diagnostics/<ts>/anr|tombstones/  ← --anr-dump
  modules/                            ← Magisk modules (shared)
  flash_history.json                  ← --history log
```

---

## Safety rules

1. Run default mode first — confirm firmware + Magisk state before any flash.
2. `--backup` before every flash — auto-runs if root available.
3. Never `--restore-full` unless on the exact same device unit.
4. Use `--serial` when multiple devices are connected.
5. Phone (2+): patch target is `init_boot.img`. Phone (1): `boot.img`.
6. ARB is fuse-burned — the tool enforces this automatically.
