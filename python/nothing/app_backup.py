"""Per-app backup and restore: APK + /data/data via root tar."""

import datetime
from pathlib import Path

from .device import adb_pull, adb_push, confirm, run
from .exceptions import AdbError
from .models import DeviceInfo

_TEMP = "/data/local/tmp"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _list_user_packages(serial: str) -> list[str]:
    r = run(["adb", "-s", serial, "shell", "pm list packages -3"])
    return [l.split("package:", 1)[1].strip()
            for l in r.stdout.splitlines() if l.startswith("package:")]


def _apk_path(package: str, serial: str) -> str | None:
    r = run(["adb", "-s", serial, "shell", f"pm path {package}"])
    for line in r.stdout.splitlines():
        if line.startswith("package:"):
            return line.split("package:", 1)[1].strip()
    return None


def _app_uid(package: str, serial: str) -> str | None:
    r = run(["adb", "-s", serial, "shell",
             f"dumpsys package {package} | grep -m1 userId="])
    for line in r.stdout.splitlines():
        if "userId=" in line:
            for tok in line.split():
                if tok.startswith("userId="):
                    return tok.split("=", 1)[1].strip()
    return None


# ---------------------------------------------------------------------------
# Backup
# ---------------------------------------------------------------------------

def action_app_backup(device: DeviceInfo, base_dir: Path,
                      packages: str | None) -> None:
    """
    Backup APK + data directory for specified packages (requires root for data).
    packages: comma-separated package names, or None to show list and prompt.
    """
    if not packages:
        # Interactive selection
        print("\nFetching user-installed packages...")
        pkgs = _list_user_packages(device.serial)
        if not pkgs:
            raise AdbError("No user-installed packages found.")
        print(f"\n{'#':<4} Package")
        print("─" * 60)
        for i, p in enumerate(pkgs):
            print(f"  {i:<3} {p}")
        raw = input("\nEnter package numbers or names (comma-separated): ").strip()
        selected = []
        for tok in raw.split(","):
            tok = tok.strip()
            if tok.isdigit():
                idx = int(tok)
                if 0 <= idx < len(pkgs):
                    selected.append(pkgs[idx])
            else:
                selected.append(tok)
        if not selected:
            print("Nothing selected.")
            return
    else:
        selected = [p.strip() for p in packages.split(",")]

    timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    apk_dir  = base_dir / "Backups" / "apk_extract"
    data_dir = base_dir / "Backups" / "app_backups" / timestamp
    apk_dir.mkdir(parents=True, exist_ok=True)
    data_dir.mkdir(parents=True, exist_ok=True)

    print(f"\nBacking up {len(selected)} app(s)")
    print(f"  APKs → {apk_dir}")
    print(f"  Data → {data_dir}")

    for pkg in selected:
        print(f"\n  [{pkg}]")

        # ── APK → shared apk_extract/ ───────────────────────────────────────
        apk_remote = _apk_path(pkg, device.serial)
        if apk_remote:
            local_apk = apk_dir / f"{pkg}.apk"
            print(f"    APK  : pulling {apk_remote.split('/')[-1]}...")
            adb_pull(apk_remote, local_apk, device.serial)
            print(f"           saved → {local_apk.name}")
        else:
            print(f"    APK  : not found (system app?)")

        # ── Data → timestamped app_backups/<ts>/ ────────────────────────────
        remote_tar = f"{_TEMP}/{pkg}_data.tar.gz"
        r = run(["adb", "-s", device.serial, "shell",
                 f"su -c 'test -d /data/data/{pkg} && "
                 f"tar czf {remote_tar} -C /data/data {pkg} 2>/dev/null && echo __OK__'"])
        if "__OK__" in r.stdout:
            local_tar = data_dir / f"{pkg}_data.tar.gz"
            print(f"    Data : pulling tar archive...")
            adb_pull(remote_tar, local_tar, device.serial)
            size_mb = local_tar.stat().st_size / 1024 / 1024
            print(f"           saved → {local_tar.name}  ({size_mb:.1f} MB)")
            run(["adb", "-s", device.serial, "shell", f"rm -f {remote_tar}"])
        else:
            print(f"    Data : skipped (no root or /data/data/{pkg} not found)")

    print(f"\n[OK] App backup complete")
    print(f"     APKs  → {apk_dir}")
    print(f"     Data  → {data_dir}")


# ---------------------------------------------------------------------------
# Restore
# ---------------------------------------------------------------------------

def action_app_restore(device: DeviceInfo, restore_dir: str | None) -> None:
    """
    Restore apps from a backup directory created by --app-backup.
    Re-installs APK then extracts data tar with correct ownership.
    """
    if not restore_dir:
        raise AdbError("Specify the backup directory with --restore-dir <path>")

    src = Path(restore_dir)
    if not src.exists():
        raise AdbError(f"Restore directory not found: {src}")

    # Data .tar.gz files are in the timestamped directory
    datas = sorted(src.glob("*_data.tar.gz"))

    # APKs live in the shared Backups/apk_extract/ sibling folder
    apk_dir = src.parent.parent / "apk_extract"
    apks = sorted(apk_dir.glob("*.apk")) if apk_dir.exists() else []

    if not apks and not datas:
        raise AdbError(f"No .apk or _data.tar.gz files found in {src} or {apk_dir}")

    print(f"\nRestore data from : {src}")
    print(f"APKs from         : {apk_dir}")
    print(f"  APKs  : {len(apks)}")
    print(f"  Data  : {len(datas)}")
    confirm("Proceed?")

    # ── Reinstall APKs ─────────────────────────────────────────────────────
    for apk in apks:
        pkg = apk.stem
        print(f"\n  Installing {apk.name}...")
        r = run(["adb", "-s", device.serial, "install", "-r", str(apk)])
        if "Success" in (r.stdout + r.stderr):
            print(f"    [OK]")
        else:
            print(f"    [WARN] {(r.stdout + r.stderr).strip()}")

    # ── Restore data ───────────────────────────────────────────────────────
    for tar in datas:
        pkg = tar.name.replace("_data.tar.gz", "")
        print(f"\n  Restoring data for {pkg}...")
        remote_tar = f"{_TEMP}/{tar.name}"
        adb_push(tar, remote_tar, device.serial)

        uid = _app_uid(pkg, device.serial) or "1000"
        r = run(["adb", "-s", device.serial, "shell",
                 f"su -c 'tar xzf {remote_tar} -C /data/data 2>/dev/null && "
                 f"chown -R {uid}:{uid} /data/data/{pkg} && "
                 f"rm -f {remote_tar} && echo __OK__'"])
        if "__OK__" in r.stdout:
            print(f"    [OK] data restored (uid={uid})")
        else:
            print(f"    [WARN] data restore may have failed: {r.stdout.strip()}")
            run(["adb", "-s", device.serial, "shell", f"rm -f {remote_tar}"])

    print(f"\n[OK] Restore complete. You may need to relaunch restored apps.")
