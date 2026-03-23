---
name: nothingctl
description: >
  Manage firmware, root (Magisk), partitions, and all aspects of Nothing phones
  via ADB. Use this skill when the user wants to: check or flash firmware, root
  or unroot a Nothing device, backup/restore partitions, install or update
  Magisk, maintain root across OTA updates, manage Magisk modules, debloat
  NothingOS, monitor thermals/CPU/RAM, control Glyph interface, sideload APKs,
  backup/restore app data, manage network/DNS/ports, control display/audio/WiFi
  settings, inspect notifications/processes/location, or manage developer options.
  Triggers on: "nothing phone", "nothing firmware", "NothingOS", "spacewar",
  "pong", "pacman", "galaxian", "flash nothing", "root nothing", "magisk",
  "init_boot patch", "nothing backup", "nothing restore", "glyph", "thermal",
  "debloat", "magisk module", "nothing settings", "essential space".
allowed-tools: Bash
---

# nothingctl Skill

This skill wraps `nothingctl.py`, a Python CLI tool that automates firmware
management, Magisk root maintenance, and full device control for Nothing phones.

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
| `--fix-biometric` | Force PIN/password auth (bypasses broken fingerprint sensor blocking root grants) | ADB |

### Modules & Apps

| Flag | What it does | Requires |
|------|-------------|----------|
| `--modules` | List recommended Magisk modules with install status + version diff | ADB root |
| `--modules --install <ids>` | Download + install modules (comma-sep IDs or `all`) | ADB root |
| `--modules-status` | List all installed Magisk modules with enabled/disabled state | ADB root |
| `--modules-toggle --module-id <id>` | Enable or disable a Magisk module (add `--enable` to enable) | ADB root |
| `--debloat` | List pre-installed NothingOS bloatware with status | ADB |
| `--debloat --remove <ids>` | Disable packages via `pm uninstall --user 0` (reversible) | ADB |
| `--sideload --apk <path>` | Install APK or split-APK directory via ADB | ADB |
| `--app-backup` | Backup APK + /data/data via root tar (interactive or --packages) | ADB root |
| `--app-restore` | Restore APK + data from --app-backup directory | ADB root |
| `--app-info --package <pkg>` | Show version, SDK, install date, APK path for an app | ADB |
| `--kill-app --package <pkg>` | Force-stop an app (add `--clear-cache` to also wipe data) | ADB |
| `--launch-app --package <pkg>` | Launch an app (or `--intent <uri>` for deep links) | ADB |
| `--package-list` | Export all installed apps as text/csv/json (use `--format`, `--output`, `--include-system`) | ADB |
| `--permissions` | Audit dangerous permissions granted to apps (use `--package` for single-app detail) | ADB |

### Device Info & Diagnostics

| Flag | What it does | Requires |
|------|-------------|----------|
| `--info` | Full device dashboard: Android version, SoC, RAM, storage, IMEI | ADB |
| `--battery` | Battery health: level, status, temperature, cycle count | ADB |
| `--battery-stats` | Per-app wakelock drain since last charge + cycle count via sysfs | ADB |
| `--reboot` | Reboot to a target (use `--target`; interactive menu if omitted) | ADB |
| `--screenshot` | Capture screenshot and save locally | ADB |
| `--screenrecord` | Record screen (use `--duration`, default 30 s) | ADB |
| `--wifi-adb` | Switch to wireless ADB mode and connect automatically | ADB (USB) |
| `--adb-pair` | Pair a new device via wireless ADB pairing code (Android 11+) | — |
| `--thermal` | Show all thermal zone temperatures with ASCII bars | ADB |
| `--thermal --watch` | Refresh thermal display every 2 s (live mode) | ADB |
| `--history` | Display flash operation history log (no device needed) | — |

### System Monitoring

| Flag | What it does | Requires |
|------|-------------|----------|
| `--memory` | RAM usage: /proc/meminfo + top apps by RSS (use `--package` for detail, `--watch` for live) | ADB |
| `--cpu-usage` | CPU core frequencies + top processes (use `--top-n`, `--watch` for live) | ADB |
| `--process-tree` | Full process list with UID/PID/state (use `--package` to filter by name) | ADB |
| `--doze-status` | Doze mode state + battery optimization whitelist (use `--whitelist-add/--whitelist-remove`) | ADB |

### Storage & Logs

| Flag | What it does | Requires |
|------|-------------|----------|
| `--storage-report` | Top-N largest dirs in /data/data/, /sdcard/ (use `--top-n`) | ADB (root for /data/data/) |
| `--apk-extract` | Pull APKs for all user-installed apps → base_dir/apk_extract/ | ADB |
| `--apk-extract --include-system` | Include system apps in the extract | ADB |
| `--logcat` | Dump logcat buffer to base_dir/logs/ | ADB |
| `--logcat --package <pkg>` | Filter by app (resolved to PID via pidof) | ADB |
| `--logcat --tag <tag> --level <V\|D\|I\|W\|E>` | Filter by log tag and/or minimum level | ADB |
| `--logcat --lines <N>` | Max lines to capture (default: 500) | ADB |
| `--bugreport` | Full adb bugreport ZIP → base_dir/bugreports/ (30–90 s) | ADB |
| `--anr-dump` | Pull /data/anr/ + /data/tombstones/ → base_dir/diagnostics/ | ADB root |
| `--cache-clear` | Clear app caches system-wide via `pm trim-caches` (use `--package` for single app) | ADB |

### Network & Connectivity

| Flag | What it does | Requires |
|------|-------------|----------|
| `--network-info` | Show WiFi SSID, signal, IP, DNS, mobile operator | ADB |
| `--dns-set` | Show or set Private DNS provider (use `--provider`: off/cloudflare/adguard/google/quad9/hostname) | ADB |
| `--port-forward` | List ADB port forwards; add with `--local/--remote`; remove all with `--clear` | ADB |
| `--wifi-scan` | Scan and list nearby WiFi networks sorted by signal strength | ADB |
| `--wifi-profiles` | List saved WiFi networks; forget one with `--forget <SSID\|ID>` | ADB |

### Display & Audio

| Flag | What it does | Requires |
|------|-------------|----------|
| `--display` | Show or set display settings (use `--key/--value`; keys: brightness/dpi/timeout/rotation/font_scale) | ADB |
| `--color-profile` | Show or set display color mode + night light (use `--mode`: natural/vivid/custom) | ADB |
| `--audio` | Show all stream volumes with ASCII bars; set with `--stream` + `--volume` | ADB |
| `--audio-route` | Show active audio output path and connected Bluetooth devices | ADB |

### Input & Control

| Flag | What it does | Requires |
|------|-------------|----------|
| `--input` | Send input to device: `--tap X,Y`, `--swipe X1,Y1,X2,Y2[,ms]`, `--text STRING`, `--keyevent CODE` | ADB |
| `--prop-get` | Read system property (use `--key`; all properties if omitted) | ADB |
| `--prop-set` | Write system property (use `--key` and `--value`) | ADB root |
| `--performance` | Set CPU governor profile (use `--profile`: performance/balanced/powersave; menu if omitted) | ADB root |
| `--locale` | Show or set language/timezone/time format (use `--lang`, `--timezone`, `--hour24`/`--no-hour24`) | ADB |
| `--location` | Show or set location mode (use `--mode`: off/gps/battery/on) + last known position + app permissions | ADB |
| `--clipboard` | Read clipboard content; set with `--clip-text <text>` (read blocked on Android 10+ by OS) | ADB |
| `--notifications` | List active notifications (use `--package` to filter by app) | ADB |

### Developer Options

| Flag | What it does | Requires |
|------|-------------|----------|
| `--dev-options` | Interactive Developer Options menu with current values; set directly with `--key/--value` | ADB |
| `--screen-always-on` | Show or control stay-awake while charging (use `--screen-on on\|off`) | ADB |
| `--charging-control` | Read or set sysfs charge limit threshold (use `--limit N`; requires custom kernel) | ADB root |

### Nothing-specific

| Flag | What it does | Requires |
|------|-------------|----------|
| `--glyph` | Show Glyph package, service state, feature settings, zone map | ADB |
| `--glyph --glyph-enable on\|off` | Toggle Glyph interface | ADB root |
| `--glyph-pattern --pattern <name>` | Run a Glyph light pattern (test/off/pulse/blink/wave) | ADB |
| `--glyph-notify` | Show Glyph notification configuration and active Hearthstone services | ADB |
| `--nothing-settings` | Read/write Nothing-specific settings (use `--ns-key namespace:key` and `--ns-value`) | ADB |
| `--essential-space` | Show or toggle Essential Space — Phone (2+) only (use `--essential-enable`/`--no-essential-enable`) | ADB |

---

## Modifier flags

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
| `--watch` | Live refresh mode for `--thermal`, `--memory`, `--cpu-usage` |
| `--glyph-enable on\|off` | Toggle argument for `--glyph` |
| `--top-n <N>` | Result count for `--storage-report` / `--cpu-usage` (default: 20) |
| `--include-system` | Include system apps in `--apk-extract` / `--package-list` |
| `--package <pkg>` | Package filter for `--logcat`, `--app-info`, `--kill-app`, `--launch-app`, `--permissions`, `--memory`, `--notifications`, `--cache-clear`, `--process-tree` |
| `--tag <tag>` | Log tag filter for `--logcat` |
| `--level <V\|D\|I\|W\|E>` | Minimum log level for `--logcat` |
| `--lines <N>` | Max log lines for `--logcat` (default: 500) |
| `--target <target>` | Reboot target for `--reboot` (system/bootloader/recovery/safe/download/sideload) |
| `--duration <N>` | Screen record duration in seconds for `--screenrecord` (default: 30, max: 180) |
| `--key <prop>` | Property key for `--prop-get/--prop-set`; setting key for `--dev-options`; display setting key for `--display` |
| `--value <val>` | Property value for `--prop-set`; setting value for `--dev-options`; display setting value for `--display` |
| `--profile <p>` | CPU governor profile for `--performance` (performance/balanced/powersave) |
| `--encrypt` | Encrypt partition backup with a password (use with `--backup`) |
| `--module-id <id>` | Module directory name for `--modules-toggle` |
| `--enable` | Enable a module with `--modules-toggle` (omit to disable) |
| `--pattern <name>` | Glyph pattern name for `--glyph-pattern` (test/off/pulse/blink/wave) |
| `--provider <p>` | DNS provider for `--dns-set` (off/cloudflare/adguard/google/quad9/hostname) |
| `--local <port>` | Local port for `--port-forward` |
| `--remote <port>` | Remote (device) port for `--port-forward` |
| `--clear` | Remove all port forwards (use with `--port-forward`) |
| `--clear-cache` | Also wipe app data when killing (use with `--kill-app`) |
| `--intent <uri>` | Deep-link URI for `--launch-app` |
| `--format text\|csv\|json` | Output format for `--package-list` (default: text) |
| `--output <path>` | Save output to file (use with `--package-list`) |
| `--tap <X,Y>` | Tap coordinates for `--input` |
| `--swipe <X1,Y1,X2,Y2[,ms]>` | Swipe coordinates for `--input` |
| `--text <string>` | Text to type on device (use with `--input`) |
| `--keyevent <code>` | Keycode name or number for `--input` (e.g. KEYCODE_HOME) |
| `--ns-key <ns:key>` | Nothing setting in `namespace:key` format (use with `--nothing-settings`) |
| `--ns-value <val>` | Value to write (use with `--nothing-settings` and `--ns-key`) |
| `--essential-enable` / `--no-essential-enable` | Enable/disable Essential Space (use with `--essential-space`) |
| `--screen-on on\|off` | Set screen always-on (use with `--screen-always-on`) |
| `--limit <N>` | Charge limit percentage 20–100 (use with `--charging-control`; requires custom kernel) |
| `--stream <name\|N>` | Audio stream for `--audio` (voice/system/ring/media/alarm/notification or 0–5) |
| `--volume <N>` | Volume level to set (use with `--audio` and `--stream`) |
| `--forget <SSID\|ID>` | Network SSID or ID to forget (use with `--wifi-profiles`) |
| `--lang <locale>` | Locale to set, e.g. `de-DE` (use with `--locale`) |
| `--timezone <tz>` | Timezone to set, e.g. `Europe/Berlin` (use with `--locale`) |
| `--hour24` / `--no-hour24` | Enable/disable 24h time format (use with `--locale`) |
| `--whitelist-add <pkg>` | Add package to Doze whitelist (use with `--doze-status`) |
| `--whitelist-remove <pkg>` | Remove package from Doze whitelist (use with `--doze-status`) |
| `--mode <mode>` | Location mode (off/gps/battery/on) for `--location`; color profile for `--color-profile` |
| `--clip-text <text>` | Text to write to clipboard (use with `--clipboard`) |

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

### Fix broken fingerprint blocking Magisk root

```bash
python <skill-dir>/nothingctl.py --fix-biometric
# Forces PIN/password auth — effect lasts until next reboot
```

### App backup

```bash
# Specific packages:
python <skill-dir>/nothingctl.py --app-backup --packages com.whatsapp,org.telegram.messenger
# All user apps:
python <skill-dir>/nothingctl.py --app-backup --packages \
  "$(adb shell pm list packages -3 | sed 's/package://g' | tr '\n' ',')"
```

### Network diagnostics

```bash
python <skill-dir>/nothingctl.py --network-info           # full network status
python <skill-dir>/nothingctl.py --dns-set --provider adguard  # set Private DNS
python <skill-dir>/nothingctl.py --wifi-scan              # show nearby networks
```

### Live device monitoring

```bash
python <skill-dir>/nothingctl.py --thermal --watch        # live thermal
python <skill-dir>/nothingctl.py --memory --watch         # live RAM
python <skill-dir>/nothingctl.py --cpu-usage --watch      # live CPU
```

---

## Device-specific notes

### Nothing Phone (3a) Lite (A001T / Galaxian)
- **SoC**: MediaTek Dimensity 7300 Pro (mt6878) — NOT Snapdragon
- **Glyph package**: `com.nothing.hearthstone` (not `ly.nothing.glyph.service`)
- **Glyph toggle**: via `am stopservice/startservice com.nothing.thirdparty/.GlyphService` (root)
- **Glyph settings**: in `global` namespace (`glyph_long_torch_enable`, `glyph_pocket_mode_state`, `glyph_screen_upward_state`)
- **Thermal**: MediaTek zone names (`soc_max`, `cpu-big-core*`, `apu`, `shell_*`); unpopulated sensors report `-274000` millidegrees and are filtered automatically
- **Essential Space**: not available (Phone 2+ only)

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
      apk_extract/                     ← flat APK store
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
  nothingctl.py         ← entry point
  modules.json          ← Magisk module definitions
  debloat.json          ← bloatware package list
  nothing/
    cli.py              — argparse + all dispatch logic
    device.py           — ADB/fastboot wrappers, detect_device(), run()
    models.py           — MagiskStatus, DeviceInfo, FirmwareState, BootTarget
    exceptions.py       — AdbError, MagiskError, FirmwareError, FlashError, FastbootTimeoutError
    firmware.py         — GitHub API, download, extraction, firmware resolution
    backup.py           — partition backup + restore + verify
    magisk.py           — Magisk status check, install/update
    arb.py              — Anti-Rollback Protection check
    modules.py          — Magisk module list, download, install/toggle
    debloat.py          — NothingOS bloatware list + disable/restore
    wifi_adb.py         — Wireless ADB setup + pairing
    glyph.py            — Glyph interface diagnostics + toggle + patterns
    thermal.py          — Thermal zone monitor (Snapdragon + MediaTek)
    sideload.py         — APK / split-APK sideload
    app_backup.py       — Per-app APK + data backup and restore
    storage.py          — Storage report + APK extraction
    diagnostics.py      — Logcat dump, bugreport, ANR/tombstone collection
    history.py          — Flash history log
    info.py             — Device info dashboard
    battery.py          — Battery health report
    batteryplus.py      — Per-app battery stats, charging control
    capture.py          — Screenshot + screenrecord (auto-scaled for encoder compat)
    permissions.py      — Dangerous permission audit
    prop.py             — System property read/write
    performance.py      — CPU governor profile control
    reboot.py           — Reboot to target
    network.py          — WiFi info, Private DNS, port forwarding
    wifimanager.py      — WiFi scan, saved network management
    appmanager.py       — App info, kill, launch, package list
    sysmon.py           — RAM + CPU monitoring with watch mode
    inputctl.py         — Tap/swipe/text/keyevent input
    devoptions.py       — Developer Options menu + screen always-on
    nothingsettings.py  — Nothing-specific settings (glyph_*, nt_*, essential_*)
    glyphnotify.py      — Glyph notification config + Hearthstone services
    display.py          — Display settings + color profile
    audio.py            — Audio stream volumes + routing
    maintenance.py      — Cache clear + locale management
    notifclip.py        — Active notifications + clipboard
    procmon.py          — Process tree + Doze status + location
```

---

## Safety rules

1. Always run default mode first to confirm firmware + Magisk state before any flash.
2. `--backup` before any flash operation (auto-backup runs if root is available).
3. Never use `--restore-full` unless restoring to the exact same device unit.
4. Check `--serial` when multiple devices are connected.
5. GKI 2.0 devices (Phone 2, 2a, 3a, 3a Lite): patch target is `init_boot.img`. Phone 1: `boot.img`.
6. ARB is permanent (eFuse) — a firmware with lower ARB index than the device will cause a boot loop. The tool blocks this automatically.
