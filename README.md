# nothingctl

A Python CLI tool for full control of Nothing phones — firmware management, Magisk root, partition backups, and deep device diagnostics via ADB.

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
- [ADB + Fastboot](https://developer.android.com/tools/releases/platform-tools) in `PATH`
- USB debugging enabled on device
- [Magisk](https://github.com/topjohnwu/Magisk) with **Superuser access → Apps and ADB** for root operations

No third-party Python packages required — stdlib only.

---

## Installation

```bash
git clone https://github.com/Limplom/nothingctl
cd nothingctl
python nothingctl.py
```

---

## Claude Code integration (optional)

This repo includes a `SKILL.md` — a ready-to-use skill for [Claude Code](https://claude.ai/claude-code).

Control your Nothing phone through natural language:

```
> check firmware and magisk status on my nothing phone
> backup all partitions
> show me active notifications filtered by com.whatsapp
> scan nearby wifi networks
> set Private DNS to AdGuard
```

Claude will call the correct `nothingctl.py` commands automatically. Just point Claude Code at this directory — the `SKILL.md` is picked up automatically.

---

## Quick start

```bash
# Check firmware version + Magisk status (always run this first)
python nothingctl.py
```

Prints: current firmware vs. latest available, Magisk version, active slot, and recommended next steps.

---

## All features

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
| `--fix-biometric` | Force PIN/password auth instead of fingerprint (workaround for broken sensor blocking root grants) | ADB |

### Magisk Modules & Apps

| Flag | What it does | Requires |
|------|-------------|----------|
| `--modules` | List recommended Magisk modules with install status | ADB root |
| `--modules --install <ids>` | Download + install modules (comma-sep IDs or `all`) | ADB root |
| `--modules-status` | List all installed Magisk modules with enabled/disabled state | ADB root |
| `--modules-toggle --module-id <id>` | Enable or disable a Magisk module (`--enable` to enable) | ADB root |
| `--debloat` | List pre-installed NothingOS bloatware with status | ADB |
| `--debloat --remove <ids>` | Disable packages via `pm uninstall --user 0` (reversible) | ADB |
| `--sideload --apk <path>` | Install APK or split-APK directory | ADB |
| `--app-backup` | Backup APK + app data for packages (use `--packages`) | ADB root |
| `--app-restore` | Restore from a previous `--app-backup` (use `--restore-dir`) | ADB root |
| `--app-info --package <pkg>` | Show version, SDK, install date, APK size for an app | ADB |
| `--kill-app --package <pkg>` | Force-stop an app (`--clear-cache` to also wipe data) | ADB |
| `--launch-app --package <pkg>` | Launch an app or deep link (`--intent <uri>`) | ADB |
| `--package-list` | Export all installed apps as text/csv/json | ADB |
| `--permissions` | Audit dangerous permissions granted to apps | ADB |

### Device Info & Battery

| Flag | What it does | Requires |
|------|-------------|----------|
| `--info` | Full device dashboard: Android version, SoC, RAM, storage, IMEI | ADB |
| `--battery` | Battery health: level, status, temperature, voltage, cycle count | ADB |
| `--battery-stats` | Per-app wakelock drain since last charge + sysfs cycle count | ADB |
| `--charging-control` | Read or set charge limit threshold via sysfs (use `--limit N`; requires custom kernel) | ADB root |
| `--reboot` | Reboot to a target (`--target`; interactive menu if omitted) | ADB |

### System Monitoring

| Flag | What it does | Requires |
|------|-------------|----------|
| `--memory` | RAM usage by app + LMK stats (`--package` for detail, `--watch` for live) | ADB |
| `--cpu-usage` | CPU core frequencies + top processes (`--top-n`, `--watch` for live) | ADB |
| `--thermal` | All thermal zone temperatures with ASCII bars (`--watch` for live) | ADB |
| `--process-tree` | Full process list with UID/PID/state (`--package` to filter) | ADB |
| `--doze-status` | Doze mode state + battery optimization whitelist (`--whitelist-add/--whitelist-remove`) | ADB |

### Display & Audio

| Flag | What it does | Requires |
|------|-------------|----------|
| `--display` | Show or set display settings (`--key/--value`; keys: brightness/dpi/timeout/rotation/font_scale) | ADB |
| `--color-profile` | Show or set display color mode + night light (`--mode`: natural/vivid/custom) | ADB |
| `--audio` | Show all stream volumes with ASCII bars; set with `--stream` + `--volume` | ADB |
| `--audio-route` | Show active audio output path and connected Bluetooth devices | ADB |

### Network & Connectivity

| Flag | What it does | Requires |
|------|-------------|----------|
| `--network-info` | Show WiFi SSID, signal, IP, DNS, mobile operator | ADB |
| `--dns-set` | Show or set Private DNS (`--provider`: off/cloudflare/adguard/google/quad9/hostname) | ADB |
| `--port-forward` | List/add/remove ADB port forwards (`--local/--remote`, `--clear` for all) | ADB |
| `--wifi-scan` | Scan and list nearby WiFi networks sorted by signal strength | ADB |
| `--wifi-profiles` | List saved WiFi networks; forget one with `--forget <SSID\|ID>` | ADB |
| `--wifi-adb` | Switch to wireless ADB and connect automatically | ADB (USB) |
| `--adb-pair` | Pair a new device via wireless ADB pairing code (Android 11+) | — |

### Input & Control

| Flag | What it does | Requires |
|------|-------------|----------|
| `--input` | Send input: `--tap X,Y` / `--swipe X1,Y1,X2,Y2[,ms]` / `--text STRING` / `--keyevent CODE` | ADB |
| `--screenshot` | Capture screenshot and save locally | ADB |
| `--screenrecord` | Record screen (`--duration`, default 30 s; auto-scaled for encoder compatibility) | ADB |
| `--locale` | Show or set language/timezone/time format (`--lang`, `--timezone`, `--hour24`) | ADB |
| `--location` | Show or set location mode + last known position + app permissions | ADB |
| `--notifications` | List active notifications (`--package` to filter by app) | ADB |
| `--clipboard` | Read clipboard (`--clip-text <text>` to write; read blocked on Android 10+ by OS) | ADB |
| `--prop-get` | Read system property (`--key`; all if omitted) | ADB |
| `--prop-set` | Write system property (`--key` and `--value`) | ADB root |
| `--performance` | Set CPU governor profile (`--profile`: performance/balanced/powersave) | ADB root |

### Developer Options

| Flag | What it does | Requires |
|------|-------------|----------|
| `--dev-options` | Interactive Developer Options menu; set directly with `--key/--value` | ADB |
| `--screen-always-on` | Show or control stay-awake while charging (`--screen-on on\|off`) | ADB |

### Storage & Logs

| Flag | What it does | Requires |
|------|-------------|----------|
| `--storage-report` | Top-N largest dirs in /data/data/, /sdcard/ (`--top-n`) | ADB |
| `--apk-extract` | Pull APKs for all user-installed apps (`--include-system`) | ADB |
| `--cache-clear` | Clear app caches system-wide (`--package` for single app) | ADB |
| `--logcat` | Dump logcat buffer to local file (`--package`, `--tag`, `--level`, `--lines`) | ADB |
| `--bugreport` | Full `adb bugreport` ZIP (30–90 s) | ADB |
| `--anr-dump` | Pull ANR traces + tombstones from /data/anr/ + /data/tombstones/ | ADB root |
| `--history` | Display flash operation history log (no device needed) | — |

### Nothing-specific

| Flag | What it does | Requires |
|------|-------------|----------|
| `--glyph` | Show Glyph package, service state, feature settings, zone map | ADB |
| `--glyph --glyph-enable on\|off` | Toggle Glyph interface | ADB root |
| `--glyph-pattern --pattern <name>` | Run a Glyph light pattern (test/off/pulse/blink/wave) | ADB |
| `--glyph-notify` | Show Glyph notification config and active Hearthstone services | ADB |
| `--nothing-settings` | Read/write Nothing-specific settings (`--ns-key namespace:key` and `--ns-value`) | ADB |
| `--essential-space` | Show or toggle Essential Space — Phone (2+) only | ADB |

---

## Common workflows

### OTA update with root preserved

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

### Fix broken fingerprint blocking Magisk

```bash
python nothingctl.py --fix-biometric
# Forces PIN/password auth instead of fingerprint — re-run after each reboot
```

### Network diagnostics

```bash
python nothingctl.py --network-info                      # full network status
python nothingctl.py --dns-set --provider adguard        # set AdGuard Private DNS
python nothingctl.py --wifi-scan                         # nearby networks
python nothingctl.py --port-forward --local 8080 --remote 8080  # add forward
```

### Live monitoring

```bash
python nothingctl.py --thermal --watch      # thermal zones live
python nothingctl.py --memory --watch       # RAM live
python nothingctl.py --cpu-usage --watch    # CPU live
```

### Display & audio control

```bash
python nothingctl.py --display                           # show all settings
python nothingctl.py --display --key brightness --value 128
python nothingctl.py --audio                             # show all volumes
python nothingctl.py --audio --stream media --volume 12  # set media volume
```

### App management

```bash
python nothingctl.py --app-info --package com.whatsapp
python nothingctl.py --kill-app --package com.whatsapp --clear-cache
python nothingctl.py --package-list --format csv --output apps.csv
python nothingctl.py --process-tree --package com.google
```

### App backup

```bash
# Specific packages:
python nothingctl.py --app-backup --packages com.whatsapp,org.telegram.messenger

# All user apps:
python nothingctl.py --app-backup --packages \
  "$(adb shell pm list packages -3 | sed 's/package://g' | tr '\n' ',')"
```

---

## Storage layout

```
~/tools/Nothing/
  <Codename>/                        ← per-device directory
    Backups/
      partition-backup/
        backup_<timestamp>/          ← 31 × .img + checksums.sha256
      apk_extract/                   ← APKs (--apk-extract, --app-backup)
      app_backups/
        <timestamp>/                 ← *_data.tar.gz per package
    logs/                            ← logcat dumps
    bugreports/                      ← bugreport ZIPs
    diagnostics/
      <timestamp>/anr/               ← ANR traces
      <timestamp>/tombstones/        ← crash tombstones
    <Codename>_<Tag>/                ← firmware archive
  modules/                           ← Magisk modules (shared)
  flash_history.json                 ← log of all flash operations
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
- Glyph toggle: via `am stopservice/startservice` (root)
- Glyph settings in `global` namespace: `glyph_long_torch_enable`, `glyph_pocket_mode_state`, `glyph_screen_upward_state`
- Thermal: MediaTek zone names; unpopulated sensors (`-274000` millidegrees) filtered automatically
- `--charging-control` and `--essential-space` not available (kernel/hardware limitation)

### Phone (1) / (2) — Legacy Glyph

- Glyph package: `ly.nothing.glyph.service`
- Glyph toggle: `settings put secure glyph_interface_enable 0|1`

### Glyph zone counts

| Model | Zones |
|-------|-------|
| Phone (1) | 5 |
| Phone (2) | 7 |
| Phone (2a) | 3 |
| Phone (3a) | 4 |
| Phone (3a) Lite | 2 |
| CMF Phone 1 | 2 |

---

## License

MIT
