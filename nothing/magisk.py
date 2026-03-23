"""Magisk status detection and install/update."""

import json
import re
from pathlib import Path
from urllib.request import Request, urlopen

from .device import confirm, run
from .exceptions import MagiskError
from .firmware import download_file, USER_AGENT
from .models import DeviceInfo, MagiskStatus

MAGISK_API = "https://api.github.com/repos/topjohnwu/Magisk/releases/latest"


# ---------------------------------------------------------------------------
# Version helpers
# ---------------------------------------------------------------------------

def _magisk_tag_to_code(tag: str) -> int | None:
    """Convert GitHub release tag like 'v30.7' to version code 30700."""
    m = re.match(r"v?(\d+)\.(\d+)", tag)
    if not m:
        return None
    return int(m.group(1)) * 1000 + int(m.group(2)) * 100


# ---------------------------------------------------------------------------
# Status probe
# ---------------------------------------------------------------------------

def check_magisk(serial: str) -> MagiskStatus:
    """Probe device and GitHub to build a complete Magisk status picture."""
    # 1 — APK presence + version code
    r = run(["adb", "-s", serial, "shell", "pm list packages --show-versioncode"])
    magisk_line = next(
        (l for l in r.stdout.splitlines() if "com.topjohnwu.magisk" in l), None
    )
    app_installed = magisk_line is not None
    installed_vc: int | None = None
    if magisk_line:
        m = re.search(r"versionCode:(\d+)", magisk_line)
        if m:
            installed_vc = int(m.group(1))

    # 2 — Root active: daemon version via su (authoritative over APK versionCode)
    root_active = False
    r = run(["adb", "-s", serial, "shell", "su -c 'magisk -V 2>/dev/null'"])
    if r.returncode == 0 and r.stdout.strip().isdigit():
        installed_vc = int(r.stdout.strip())
        root_active  = True

    # 3 — Latest from GitHub (graceful failure if offline)
    latest_vc: int | None = None
    latest_str: str | None = None
    latest_url: str | None = None
    try:
        req  = Request(MAGISK_API, headers={"User-Agent": USER_AGENT})
        data = json.loads(urlopen(req, timeout=10).read())
        tag  = data.get("tag_name", "")
        latest_str = tag.lstrip("v")
        latest_vc  = _magisk_tag_to_code(tag)
        apk = next(
            (a for a in data.get("assets", [])
             if a["name"].startswith("Magisk-v") and a["name"].endswith(".apk")),
            None,
        )
        if apk:
            latest_url = apk["browser_download_url"]
    except Exception:
        pass  # offline — still report local state

    return MagiskStatus(
        app_installed=app_installed,
        root_active=root_active,
        installed_version=installed_vc,
        latest_version=latest_vc,
        latest_version_str=latest_str,
        latest_apk_url=latest_url,
    )


# ---------------------------------------------------------------------------
# Status display
# ---------------------------------------------------------------------------

def print_magisk_status(ms: MagiskStatus) -> None:
    """Print Magisk status and feature-availability table."""
    print(f"\n  Magisk : {ms.state_label}")
    if ms.latest_version_str:
        print(f"  Latest : v{ms.latest_version_str}"
              + ("  [UPDATE AVAILABLE]" if ms.is_outdated else "  [up to date]"))

    root = ms.root_active

    features = [
        ("Firmware check + download",           True, "always available"),
        ("--flash-firmware / --restore",         True, "fastboot — no root needed"),
        ("--push-for-patch / --flash-patched",   True, "fastboot — no root needed"),
        ("--backup (partition dump)",            root, "requires root + ADB su"),
        ("Auto-backup before flash",             root, "requires root + ADB su"),
        ("Performance tweaks (su)",              root, "requires root + ADB su"),
        ("System cert install",                  root, "requires root + ADB su"),
        ("App private data access",              root, "requires root + ADB su"),
    ]

    if not root:
        print("\n  Feature availability without active root:")
        for name, avail, note in features:
            mark = "[OK]  " if avail else "[N/A] "
            print(f"    {mark} {name}")
            if not avail:
                print(f"           -> {note}")

    if not ms.app_installed:
        print("\n  Run --install-magisk to install Magisk and enable root features.")
    elif ms.is_outdated:
        print("\n  Run --install-magisk to update Magisk.")


# ---------------------------------------------------------------------------
# Install / update
# ---------------------------------------------------------------------------

def action_install_magisk(device: DeviceInfo, base_dir: Path,
                          ms: MagiskStatus) -> None:
    """Download latest Magisk APK and install/update on device."""
    if not ms.latest_apk_url:
        raise MagiskError(
            "Could not fetch latest Magisk release from GitHub.\n"
            "Check internet connection or install manually from "
            "https://github.com/topjohnwu/Magisk/releases"
        )

    action = "Update" if ms.app_installed else "Install"
    print(f"\n{action} Magisk v{ms.latest_version_str}")

    if not ms.app_installed:
        print("\nFeatures enabled after installation + patching boot image:")
        print("  --backup, auto-backup, performance tweaks, system cert install, app data access")

    confirm(f"{action} Magisk APK on device?")

    apk_name = ms.latest_apk_url.split("/")[-1]
    apk_path = base_dir / apk_name
    if not apk_path.exists():
        print(f"Downloading {apk_name}...")
        download_file(ms.latest_apk_url, apk_path)

    print(f"Installing {apk_name}...")
    r = run(["adb", "-s", device.serial, "install", "-r", str(apk_path)])
    if r.returncode != 0:
        raise MagiskError(f"adb install failed: {r.stderr.strip()}")

    print(f"[OK] Magisk {action}d.")

    if not ms.root_active:
        print("\nNext steps to activate root:")
        print("  1. python nothingctl.py --push-for-patch   (push boot image to device)")
        print("  2. Open Magisk app -> Install -> Patch an Image -> select the file")
        print("  3. python nothingctl.py --flash-patched    (flash patched image)")
    else:
        print("\nOpen Magisk and tap 'Update' if prompted to update the daemon.")
