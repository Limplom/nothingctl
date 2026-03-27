"""GitHub API, firmware download, extraction, and resolution."""

import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

from .device import run
from .exceptions import FirmwareError
from .models import BootTarget, DeviceInfo, FirmwareState

GITHUB_API            = "https://api.github.com/repos/spike0en/nothing_archive"
USER_AGENT            = "nothing-firmware-manager/2.0"
SDCARD_DOWNLOAD       = "/sdcard/Download"
SAFE_FLASH_PARTITIONS = ["boot", "dtbo", "vendor_boot"]  # init_boot added dynamically


# ---------------------------------------------------------------------------
# GitHub API helpers
# ---------------------------------------------------------------------------

def gh_get(path: str):
    req = Request(f"{GITHUB_API}{path}", headers={"User-Agent": USER_AGENT})
    with urlopen(req, timeout=20) as resp:
        return json.loads(resp.read())


def fetch_releases(codename: str) -> list:
    releases = gh_get("/releases?per_page=50")
    prefix = codename.lower() + "_"
    return [r for r in releases if r["tag_name"].lower().startswith(prefix)]


def latest_release(releases: list) -> dict:
    def sort_key(r):
        m = re.search(r"-(\d{6})-", r["tag_name"])
        return m.group(1) if m else "000000"
    return max(releases, key=sort_key)


def find_asset(release: dict, suffix: str) -> dict | None:
    return next((a for a in release.get("assets", []) if a["name"].endswith(suffix)), None)


# ---------------------------------------------------------------------------
# Download + extraction
# ---------------------------------------------------------------------------

def download_file(url: str, dest: Path) -> Path:
    dest.parent.mkdir(parents=True, exist_ok=True)
    req = Request(url, headers={"User-Agent": USER_AGENT})
    with urlopen(req, timeout=60) as resp:
        total = int(resp.headers.get("Content-Length", 0))
        done = 0
        with open(dest, "wb") as f:
            while chunk := resp.read(131072):
                f.write(chunk)
                done += len(chunk)
                if total:
                    pct = done * 100 // total
                    print(f"\r  {pct}%  {done/1024/1024:.1f} / {total/1024/1024:.1f} MB",
                          end="", flush=True)
    print()
    return dest


def extract_7z(archive: Path, dest_dir: Path) -> bool:
    candidates = ["7z", "7zz", "7za",
                  r"C:\Program Files\7-Zip\7z.exe",
                  "/c/Program Files/7-Zip/7z.exe"]
    for z in candidates:
        try:
            r = subprocess.run([z, "e", str(archive), f"-o{dest_dir}", "-y"],
                               capture_output=True, text=True)
            if r.returncode == 0:
                return True
        except FileNotFoundError:
            continue
    try:
        import py7zr
        with py7zr.SevenZipFile(archive, mode="r") as z:
            z.extractall(path=dest_dir)
        return True
    except ImportError:
        pass
    return False


# ---------------------------------------------------------------------------
# Boot target + partition list
# ---------------------------------------------------------------------------

def detect_boot_target(extracted_dir: Path) -> BootTarget:
    if (extracted_dir / "init_boot.img").exists():
        return BootTarget("init_boot.img", "init_boot", is_gki2=True)
    if (extracted_dir / "boot.img").exists():
        return BootTarget("boot.img", "boot", is_gki2=False)
    raise FirmwareError(
        f"Neither init_boot.img nor boot.img found in {extracted_dir}. "
        "The firmware package may be incomplete."
    )


def build_partition_list(boot_target: BootTarget) -> list[str]:
    """Return base partition names to flash (without _a/_b suffix)."""
    partitions = list(SAFE_FLASH_PARTITIONS)
    if boot_target.is_gki2:
        partitions = ["init_boot"] + partitions
    return partitions


# ---------------------------------------------------------------------------
# Firmware resolution (check + optional download)
# ---------------------------------------------------------------------------

def resolve_firmware(device: DeviceInfo, base_dir: Path, force: bool) -> FirmwareState:
    codename        = device.codename
    current_version = run(["adb", "-s", device.serial, "shell",
                           "getprop ro.build.display.id"]).stdout.strip()

    print(f"  Codename: {codename}")
    print(f"  Current : {current_version or 'unknown'}")

    print("\nChecking nothing_archive...")
    try:
        releases = fetch_releases(codename)
    except URLError as e:
        raise FirmwareError(f"Cannot reach GitHub API: {e}")

    if not releases:
        raise FirmwareError(f"No releases found for codename '{codename}'.")

    latest     = latest_release(releases)
    latest_tag = latest["tag_name"]
    print(f"  Latest  : {latest_tag}")

    current_tag = f"{codename.capitalize()}_{current_version}" if current_version else None
    is_newer    = current_tag != latest_tag

    print(f"  Status  : {'UPDATE AVAILABLE' if is_newer else 'up to date'}")

    dest_dir       = base_dir / codename / latest_tag
    init_boot_path = dest_dir / "init_boot.img"
    boot_path      = dest_dir / "boot.img"

    if (init_boot_path.exists() or boot_path.exists()) and not force:
        print(f"  Cached  : {dest_dir}")
    else:
        asset = find_asset(latest, "-image-boot.7z")
        if not asset:
            raise FirmwareError("No image-boot.7z asset in release.")
        print(f"\nDownloading {asset['name']} ({asset['size']//1024//1024} MB)...")
        archive = download_file(asset["browser_download_url"], dest_dir / asset["name"])
        print("Extracting...")
        if not extract_7z(archive, dest_dir):
            raise FirmwareError("Extraction failed. Ensure 7-Zip is installed.")
        archive.unlink(missing_ok=True)

    boot_target = detect_boot_target(dest_dir)
    return FirmwareState(
        extracted_dir=dest_dir,
        version=latest_tag,
        is_newer=is_newer,
        boot_target=boot_target,
    )


# ---------------------------------------------------------------------------
# Patched image finder
# ---------------------------------------------------------------------------

def find_magisk_patched(serial: str) -> str | None:
    r = run(["adb", "-s", serial, "shell", f"ls -t {SDCARD_DOWNLOAD}/magisk_patched*.img"])
    first_line = r.stdout.strip().splitlines()[0] if r.stdout.strip() else ""
    return first_line if first_line and "No such" not in first_line else None
