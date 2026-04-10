# nothingctl

CLI tool for full control of Nothing phones — firmware management, Magisk root, partition backups, and deep device diagnostics via ADB.

Available as a **compiled Go binary** (recommended) or as a Python script.

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

## Installation

### Go binary (recommended)

Download the latest release for your platform from the [Releases](https://github.com/Limplom/nothingctl/releases) page:

| Platform | File |
|---|---|
| Windows x86-64 | `nothingctl-windows-amd64.exe` |
| Linux x86-64 | `nothingctl-linux-amd64` |
| Linux ARM64 | `nothingctl-linux-arm64` |
| macOS Intel | `nothingctl-darwin-amd64` |
| macOS Apple Silicon | `nothingctl-darwin-arm64` |

No installer, no runtime — just download and run.

```bash
# Linux / macOS
chmod +x nothingctl-linux-amd64
./nothingctl-linux-amd64 --help
```

### Python (legacy)

```bash
git clone https://github.com/Limplom/nothingctl
cd nothingctl
python python/nothingctl.py
```

Requires Python 3.10+. No third-party packages — stdlib only.

---

## Requirements

- [ADB + Fastboot](https://developer.android.com/tools/releases/platform-tools) in `PATH`
- [7-Zip](https://www.7-zip.org/) (`7z`) in `PATH` — for firmware extraction only
- USB debugging enabled on device
- [Magisk](https://github.com/topjohnwu/Magisk) with **Superuser access → Apps and ADB** for root operations

---

## Quick start

```bash
# Show device dashboard
nothingctl info

# Check Magisk / root manager status
nothingctl root-status

# Firmware check → backup → OTA with root preserved
nothingctl root-status
nothingctl backup
nothingctl ota-update
```

---

## All commands

### Firmware & Root

| Command | What it does | Requires |
|---------|-------------|----------|
| `check-update` | Check nothing_archive for a firmware update (no download) | ADB |
| `root-status` | Detect active root manager (Magisk / KernelSU / APatch) | ADB |
| `backup` | Dump 31 partitions via root dd → local storage + checksums | ADB root |
| `restore` | Flash backed-up partitions back to device | Fastboot |
| `verify-backup` | Compare local backup checksums for integrity | — |
| `verify-backup --live` | Compare backup checksums against **live device** partition hashes | ADB root |
| `install-magisk` | Download + install/update Magisk APK on device | ADB |
| `update-magisk` | Update Magisk to latest version | ADB |
| `flash-firmware` | Flash all boot partitions from nothing_archive + ARB check | Fastboot |
| `ota-update` | One-shot: download + Magisk CLI patch + flash (root preserved) | ADB root + Fastboot |
| `unroot` | Flash stock boot to both slots (removes root) | Fastboot |
| `push-for-patch` | Push stock boot image to /sdcard/Download/ | ADB |
| `flash-patched` | Pull magisk_patched*.img and flash both slots | Fastboot |
| `fix-biometric` | Force PIN/password auth (workaround for fingerprint blocking root grants) | ADB |
| `history` | Display flash operation history log | — |

### Magisk Modules & Apps

| Command | What it does | Requires |
|---------|-------------|----------|
| `modules` | List recommended Magisk modules with install status | ADB root |
| `modules --install <ids>` | Download + install modules (comma-sep IDs or `all`) | ADB root |
| `modules-status` | List installed Magisk modules with enabled/disabled state | ADB root |
| `modules-toggle --modules <ids>` | Enable or disable a Magisk module | ADB root |
| `modules-update-all` | Check all installed modules for updates and install them (`--force` skips prompt) | ADB root |
| `debloat` | List pre-installed NothingOS bloatware with status | ADB |
| `debloat --remove <ids>` | Disable packages via `pm uninstall --user 0` (reversible) | ADB |
| `debloat --profile <name>` | Disable all packages in a predefined tier: `minimal` / `recommended` / `aggressive` | ADB |
| `sideload --apk <path>` | Install APK or split-APK directory | ADB |
| `app-backup` | Backup APK + app data (`--packages` to filter) | ADB root |
| `app-restore` | Restore from a previous `app-backup` | ADB root |
| `app-info --package <pkg>` | Show version, SDK, install date, APK size | ADB |
| `kill-app --package <pkg>` | Force-stop an app | ADB |
| `launch-app --package <pkg>` | Launch an app or deep link | ADB |
| `package-list` | Export all installed apps as text/csv/json | ADB |
| `permissions` | Audit dangerous permissions granted to apps | ADB |

### Device Info & Battery

| Command | What it does | Requires |
|---------|-------------|----------|
| `info` | Full device dashboard: Android version, SoC, RAM, storage, IMEI | ADB |
| `battery` | Battery health: level, status, temperature, voltage, cycle count | ADB |
| `battery-stats` | Per-app wakelock drain since last charge | ADB |
| `charging-control --limit N` | Set charge limit threshold via sysfs | ADB root |
| `reboot` | Reboot to a target (`--target`; interactive menu if omitted) | ADB |

### System Monitoring

| Command | What it does | Requires |
|---------|-------------|----------|
| `memory` | RAM usage by app + LMK stats (`--watch` for live) | ADB |
| `cpu-usage` | CPU core frequencies + top processes (`--watch` for live) | ADB |
| `thermal` | All thermal zone temperatures with ASCII bars (`--watch` for live) | ADB |
| `process-tree` | Full process list with UID/PID/state (`--package` to filter) | ADB |
| `doze-status` | Doze mode state + battery optimization whitelist | ADB |
| `logcat` | Dump logcat buffer to file (`--package`, `--tag`, `--level`, `--lines`) | ADB |
| `bugreport` | Full `adb bugreport` ZIP | ADB |
| `anr-dump` | Pull ANR traces + tombstones | ADB root |

### Display & Audio

| Command | What it does | Requires |
|---------|-------------|----------|
| `display` | Show or set display settings (`--set brightness/dpi/timeout/rotation/font-scale`) | ADB |
| `color-profile --profile <name>` | Set display color mode (natural/vivid/srgb) | ADB |
| `audio` | Show all stream volumes with ASCII bars; set with `--set` + `--volume` | ADB |
| `audio-route` | Show active audio output path and connected Bluetooth devices | ADB |
| `screenshot` | Capture screenshot and save locally | ADB |
| `screenrecord` | Record screen (`--duration`, default 30 s) | ADB |

### Network & Connectivity

| Command | What it does | Requires |
|---------|-------------|----------|
| `network-info` | Show WiFi SSID, signal, IP, DNS, mobile operator | ADB |
| `dns-set --provider <name>` | Set Private DNS (off/cloudflare/adguard/google/quad9/hostname) | ADB |
| `port-forward` | List/add/remove ADB port forwards (`--local/--remote`, `--clear`) | ADB |
| `wifi-scan` | Scan and list nearby WiFi networks sorted by signal strength | ADB |
| `wifi-profiles` | List saved WiFi networks; forget one with `--ssid` | ADB |
| `wifi-adb` | Switch to wireless ADB and connect automatically | ADB (USB) |
| `adb-pair --port <port>` | Pair device via wireless ADB pairing code (Android 11+) | — |

### Input & Control

| Command | What it does | Requires |
|---------|-------------|----------|
| `input` | Send input: `--tap X,Y` / `--swipe X1,Y1,X2,Y2` / `--text STR` / `--keyevent CODE` | ADB |
| `locale` | Show or set language/timezone/time format | ADB |
| `location` | Show or set location mode | ADB |
| `notifications` | List active notifications (`--package` to filter) | ADB |
| `clipboard` | Read or write clipboard (`--text` to write) | ADB |
| `prop-get` | Read system property (`--key`; all if omitted) | ADB |
| `prop-set --key K --value V` | Write system property | ADB root |
| `performance` | Set CPU governor profile (`--profile`: performance/balanced/powersave) | ADB root |
| `storage-report` | Top-N largest dirs in /data/data/, /sdcard/ | ADB |
| `apk-extract` | Pull APKs for all user-installed apps | ADB |
| `cache-clear` | Clear app caches system-wide (`--package` for single app) | ADB |

### Developer Options

| Command | What it does | Requires |
|---------|-------------|----------|
| `dev-options` | Show or set Developer Options (`--set/--value`) | ADB |
| `screen-always-on` | Control stay-awake while charging (`--enable true\|false`) | ADB |

### Utility

| Command | What it does | Requires |
|---------|-------------|----------|
| `self-update` | Check GitHub for a newer nothingctl release and replace the running binary (`--dry-run` to preview) | — |

### Nothing-specific

| Command | What it does | Requires |
|---------|-------------|----------|
| `glyph` | Show Glyph package, service state, feature settings, zone map | ADB |
| `glyph --enable on\|off` | Toggle Glyph interface | ADB |
| `glyph-pattern --pattern <name>` | Run a Glyph light pattern (pulse/blink/wave) | ADB |
| `glyph-notify` | Show Glyph notification config and active Hearthstone services | ADB |
| `nothing-settings` | Read/write Nothing-specific settings (`--set/--value`) | ADB |
| `essential-space` | Toggle Essential Space — Phone (2+) only (`--enable true\|false`) | ADB |

---

## Common workflows

### OTA update with root preserved

```bash
nothingctl ota-update
# Downloads firmware → patches init_boot with Magisk CLI → flashes both slots
```

### Manual firmware update

```bash
nothingctl root-status        # check state
nothingctl backup             # safety backup
nothingctl flash-firmware     # flash boot partitions
nothingctl push-for-patch     # push image to device
# patch in Magisk app on device
nothingctl flash-patched      # flash patched image
```

### Live monitoring

```bash
nothingctl thermal --watch
nothingctl memory --watch
nothingctl cpu-usage --watch
```

### Debloat

```bash
nothingctl debloat                                               # list status
nothingctl debloat --profile minimal                             # remove Facebook/tracking packages
nothingctl debloat --profile recommended                         # minimal + Nothing optional apps
nothingctl debloat --remove facebook-services,facebook-appmanager # manual selection
```

### Network diagnostics

```bash
nothingctl network-info
nothingctl dns-set --provider adguard
nothingctl wifi-scan
nothingctl port-forward --local 8080 --remote 8080
```

### Target a specific device

```bash
nothingctl -s <serial> info
nothingctl -s <serial> backup
```

### Run across all connected devices

```bash
nothingctl --serial all info
nothingctl --serial all battery
nothingctl --serial all root-status
nothingctl --serial all check-update
```

### Update nothingctl itself

```bash
nothingctl self-update            # download and replace binary
nothingctl self-update --dry-run  # preview only
```

---

## Global flags

| Flag | Description |
|------|-------------|
| `-s, --serial` | Target a specific device by ADB serial — use `all` to run on every connected device |
| `--base-dir` | Override default storage root (`~/tools/Nothing`) |
| `--force-download` | Re-download firmware even if already cached |
| `--no-backup` | Skip automatic backup before flashing |

---

## Storage layout

```
~/tools/Nothing/
  <Codename>/
    Backups/
      partition-backup/backup_<timestamp>/   ← 31 × .img + checksums.sha256
      apk_extract/
      app_backups/<timestamp>/               ← *_data.tar.gz per package
    logs/                                    ← logcat dumps
    bugreports/
    diagnostics/<timestamp>/anr/
    diagnostics/<timestamp>/tombstones/
    <Codename>_<Tag>/                        ← extracted firmware
  modules/                                   ← Magisk modules
  flash_history.json
```

---

## Building from source

Requires [Go 1.22+](https://go.dev/dl/) and `adb`/`fastboot` in PATH.

```bash
git clone https://github.com/Limplom/nothingctl
cd nothingctl/go

# Build for your current platform
go run ./cmd/nothingctl/ --help          # run without installing
go build -o ../nothingctl ./cmd/nothingctl/   # build binary

# Or use make (Linux / macOS / Git Bash on Windows)
make build        # → ../nothingctl
make dist         # → ../dist/ for all 5 platforms
```

### Manual cross-compilation

```bash
# Windows (from Linux/macOS)
GOOS=windows GOARCH=amd64 go build -o nothingctl.exe ./cmd/nothingctl/

# Linux ARM64 (e.g. Raspberry Pi, Apple Silicon Linux)
GOOS=linux GOARCH=arm64 go build -o nothingctl ./cmd/nothingctl/

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o nothingctl ./cmd/nothingctl/
```

All commands must be run from the `go/` directory. The `debloat.json` and `modules.json` data files are embedded at compile time — the resulting binary has no runtime file dependencies.

---

## ARB (Anti-Rollback Protection)

Checked automatically before `flash-firmware` and `ota-update`. Reads the `rollback_index` from `vbmeta.img` and compares it against the device's current ARB index. **A firmware with a lower ARB index than the device will cause a permanent boot loop** — the tool blocks this automatically.

---

## Safety rules

1. Run `root-status` and `info` first to confirm device state.
2. Run `backup` before every flash — auto-backup runs if root is available.
3. Never restore to a different device unit than the one backed up from.
4. Use `--serial` when multiple devices are connected.
5. GKI 2.0 devices (Phone 2, 2a, 3a, 3a Lite): patch target is `init_boot.img`. Phone (1): `boot.img`.
6. ARB is fuse-burned and permanent — the tool enforces this automatically.

---

## Device-specific notes

### Nothing Phone (3a) Lite — MediaTek

- Glyph toggle via `am stopservice/startservice` (root)
- Thermal: MediaTek zone names; unpopulated sensors filtered automatically
- `charging-control` and `essential-space` not available (hardware limitation)

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

## Claude Code integration (optional)

This repo includes a `SKILL.md` — a ready-to-use skill for [Claude Code](https://claude.ai/claude-code).

Control your Nothing phone through natural language:

```
> check firmware and magisk status on my nothing phone
> backup all partitions
> show thermal zones live
> set Private DNS to AdGuard
```

---

## License

MIT
