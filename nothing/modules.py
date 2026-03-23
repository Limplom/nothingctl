"""Recommended Magisk module list, download, and install helpers.

Module definitions live in modules.json (skill root directory).
Edit that file to add, remove, or update modules — no Python changes needed.
"""

import json
import os
import re
from dataclasses import dataclass
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

from .device import adb_push, confirm, run
from .exceptions import AdbError, MagiskError
from .firmware import USER_AGENT, download_file
from .models import DeviceInfo

GITHUB_API_BASE  = "https://api.github.com/repos"
# modules.json sits next to nothingctl.py (two levels up from this file)
_MODULES_JSON    = Path(__file__).parent.parent / "modules.json"


# ---------------------------------------------------------------------------
# ModuleInfo dataclass — populated at runtime from modules.json
# ---------------------------------------------------------------------------

@dataclass
class ModuleInfo:
    id:              str
    name:            str
    description:     str
    category:        str         # framework / privacy / utility / apps
    source:          str         # "github" | "ksu_store"
    install_type:    str         # "zip" | "apk"
    repo:            str | None  # "owner/repo" for GitHub
    asset_pattern:   str         # regex to match release asset filename
    requires_zygisk: bool = False
    use_prerelease:  bool = False
    notes:           str = ""


def load_modules(json_path: Path = _MODULES_JSON) -> list[ModuleInfo]:
    """Load module definitions from modules.json."""
    if not json_path.exists():
        raise FileNotFoundError(
            f"modules.json not found at {json_path}\n"
            "Expected alongside nothingctl.py in the skill directory."
        )
    with open(json_path, encoding="utf-8") as f:
        data = json.load(f)
    return [ModuleInfo(**entry) for entry in data["modules"]]


# ---------------------------------------------------------------------------
# GitHub helper — generic, any repo
# ---------------------------------------------------------------------------

def _github_headers() -> dict:
    """Build request headers, adding Authorization if GITHUB_TOKEN is set."""
    headers = {"User-Agent": USER_AGENT}
    token = os.environ.get("GITHUB_TOKEN")
    if token:
        headers["Authorization"] = f"Bearer {token}"
    return headers


def _github_latest(repo: str, use_prerelease: bool = False) -> tuple[str, list[dict]]:
    """Return (tag_name, assets) for the latest release of any GitHub repo.

    When use_prerelease=True, fetches the full releases list and returns the
    first entry (most recent), which may be a pre-release. Needed for repos
    like PlayIntegrityFix that publish only pre-releases.
    """
    if use_prerelease:
        url = f"{GITHUB_API_BASE}/{repo}/releases?per_page=1"
    else:
        url = f"{GITHUB_API_BASE}/{repo}/releases/latest"
    req = Request(url, headers=_github_headers())
    with urlopen(req, timeout=20) as resp:
        data = json.loads(resp.read())
    if use_prerelease:
        data = data[0]   # list → take first (most recent)
    return data["tag_name"], data.get("assets", [])


def _find_asset(assets: list[dict], pattern: str) -> dict | None:
    """Return first matching asset, preferring arm64 builds for Nothing phones."""
    rx      = re.compile(pattern, re.IGNORECASE)
    matches = [a for a in assets if rx.search(a["name"])]
    if not matches:
        return None
    # Nothing phones are all ARM64 — prefer arm64 when multiple builds exist
    arm64 = [a for a in matches if "arm64" in a["name"].lower()]
    return arm64[0] if arm64 else matches[0]


# ---------------------------------------------------------------------------
# Installed module detection
# ---------------------------------------------------------------------------

def get_installed_modules(serial: str) -> set[str]:
    """Return directory names under /data/adb/modules/ (requires root)."""
    r = run(["adb", "-s", serial, "shell",
             "su -c 'ls /data/adb/modules/ 2>/dev/null'"])
    if r.returncode != 0 or not r.stdout.strip():
        return set()
    return set(r.stdout.strip().splitlines())


def get_installed_versions(serial: str) -> dict[str, str]:
    """
    Read version= from every /data/adb/modules/*/module.prop in one ADB call.
    Returns {dir_name: version_string}.
    """
    r = run(["adb", "-s", serial, "shell",
             "su -c 'for d in /data/adb/modules/*/; do "
             "  name=$(basename $d); "
             "  ver=$(grep -m1 \"^version=\" $d/module.prop 2>/dev/null | cut -d= -f2); "
             "  echo \"$name|$ver\"; "
             "done'"])
    result: dict[str, str] = {}
    for line in r.stdout.strip().splitlines():
        parts = line.split("|", 1)
        if len(parts) == 2 and parts[0]:
            result[parts[0].strip()] = parts[1].strip()
    return result


def _is_installed(module: ModuleInfo, installed_dirs: set[str]) -> bool:
    """Fuzzy-match module id against installed Magisk module directory names."""
    key = module.id.lower().replace("-", "").replace("_", "")
    for d in installed_dirs:
        if key in d.lower().replace("_", "").replace("-", ""):
            return True
    return False


# ---------------------------------------------------------------------------
# Display
# ---------------------------------------------------------------------------

def _installed_version(module: ModuleInfo, installed_dirs: set[str],
                       versions: dict[str, str]) -> str | None:
    """Find installed version string by fuzzy-matching module id to dir names."""
    key = module.id.lower().replace("-", "").replace("_", "")
    for d in installed_dirs:
        if key in d.lower().replace("_", "").replace("-", ""):
            return versions.get(d)
    return None


def print_module_list(
    modules: list[ModuleInfo],
    installed_dirs: set[str],
    installed_versions: dict[str, str],
    latest_tags: dict[str, str | None],
    root_available: bool,
) -> None:
    CATEGORIES = ["framework", "privacy", "utility", "apps"]
    print()
    for cat in CATEGORIES:
        group = [m for m in modules if m.category == cat]
        if not group:
            continue
        print(f"[{cat}]")
        for m in group:
            tag         = latest_tags.get(m.id)
            latest_str  = tag or "?"

            if m.source == "ksu_store":
                status = "[manual install]"
                ver_str = ""
            elif not root_available:
                status  = "[unknown]"
                ver_str = ""
            elif _is_installed(m, installed_dirs):
                inst_ver = _installed_version(m, installed_dirs, installed_versions)
                if inst_ver and tag:
                    # Normalize for comparison: strip leading 'v'
                    iv = inst_ver.lstrip("v")
                    tv = tag.lstrip("v")
                    if iv == tv:
                        status  = "[INSTALLED]"
                        ver_str = f"  {inst_ver} — up to date"
                    else:
                        status  = "[UPDATE]"
                        ver_str = f"  {inst_ver} -> {tag}"
                else:
                    status  = "[INSTALLED]"
                    ver_str = f"  {inst_ver or '?'}"
            else:
                status  = "[not installed]"
                ver_str = f"  latest: {latest_str}"

            zygisk_str = "  [needs Zygisk]" if m.requires_zygisk else ""
            print(f"  {m.id:<24} {status:<16}{ver_str}{zygisk_str}")
            print(f"  {'':24} {m.description}")
            if m.notes:
                print(f"  {'':24} -> {m.notes}")
            print()

    print("Usage:")
    print("  List modules  :  python nothingctl.py --modules")
    print("  Install one   :  python nothingctl.py --modules --install lsposed")
    print("  Install set   :  python nothingctl.py --modules --install lsposed,shamiko,play-integrity-fix")
    print("  Install all   :  python nothingctl.py --modules --install all")


# ---------------------------------------------------------------------------
# Download + install
# ---------------------------------------------------------------------------

def _download_module(module: ModuleInfo, base_dir: Path) -> Path:
    if module.source != "github" or module.repo is None:
        raise MagiskError(
            f"'{module.id}' requires manual install.\n"
            f"  → {module.notes}"
        )
    tag, assets = _github_latest(module.repo, use_prerelease=module.use_prerelease)
    asset = _find_asset(assets, module.asset_pattern)
    if not asset:
        available = [a["name"] for a in assets]
        raise MagiskError(
            f"No matching asset for '{module.id}' in {module.repo} @ {tag}.\n"
            f"Pattern: {module.asset_pattern}\n"
            f"Available: {available}"
        )
    dest = base_dir / "modules" / module.id / tag / asset["name"]
    if dest.exists():
        print(f"  Cached  : {dest.name}")
        return dest
    mb = asset["size"] // 1024 // 1024
    print(f"  Download: {asset['name']} ({mb} MB)...")
    return download_file(asset["browser_download_url"], dest)


def _install_zip(local: Path, device: DeviceInfo) -> None:
    remote = f"/data/local/tmp/{local.name}"
    print(f"  Pushing {local.name}...")
    adb_push(local, remote, device.serial)
    print("  Installing via Magisk...")
    r = run(["adb", "-s", device.serial, "shell",
             f"su -c 'magisk --install-module {remote} && rm -f {remote}'"])
    if r.returncode != 0:
        raise MagiskError(
            f"Module install failed: {(r.stderr or r.stdout).strip()}"
        )


def _install_apk(local: Path, device: DeviceInfo, module: ModuleInfo) -> None:
    if module.notes:
        print(f"  NOTE: {module.notes}")
    print(f"  Installing APK {local.name}...")
    r = run(["adb", "-s", device.serial, "install", "-r", str(local)])
    if r.returncode != 0:
        raise AdbError(f"APK install failed: {r.stderr.strip()}")


def install_module(module: ModuleInfo, device: DeviceInfo, base_dir: Path) -> None:
    """Download and install a single module. Skips ksu_store modules with instructions."""
    if module.source == "ksu_store":
        print(f"\n[SKIP] {module.name} — manual install required.")
        print(f"       → {module.notes}")
        return

    print(f"\nInstalling {module.name}...")
    local = _download_module(module, base_dir)

    if module.install_type == "zip":
        _install_zip(local, device)
    else:
        _install_apk(local, device, module)

    print(f"[OK] {module.name} installed.")
    if module.notes:
        print(f"     → {module.notes}")


# ---------------------------------------------------------------------------
# Main action
# ---------------------------------------------------------------------------

def action_modules(
    device: DeviceInfo,
    base_dir: Path,
    install_ids: str | None,
) -> None:
    """
    List recommended modules with install status, or install specified ones.

    install_ids: None = list only | "all" = all GitHub modules | "id,id,..." = specific ones
    """
    from .backup import check_adb_root  # local import to avoid circular dependency

    modules       = load_modules()
    root_ok           = check_adb_root(device.serial)
    installed_dirs    = get_installed_modules(device.serial) if root_ok else set()
    installed_versions = get_installed_versions(device.serial) if root_ok else {}

    if not root_ok and install_ids:
        raise AdbError(
            "Root required to install Magisk modules.\n"
            "Enable in Magisk: Settings → Superuser access → Apps and ADB."
        )

    # Fetch latest release tags for display
    print("Fetching release info", end="", flush=True)
    latest_tags: dict[str, str | None] = {}
    for m in modules:
        if m.source == "github" and m.repo:
            try:
                tag, _ = _github_latest(m.repo, use_prerelease=m.use_prerelease)
                latest_tags[m.id] = tag
            except Exception:
                latest_tags[m.id] = None
        else:
            latest_tags[m.id] = None
        print(".", end="", flush=True)
    print()

    if not install_ids:
        print_module_list(modules, installed_dirs, installed_versions, latest_tags, root_ok)
        return

    # ── Install mode ──────────────────────────────────────────────────────
    if install_ids.lower() == "all":
        targets = [m for m in modules if m.source == "github"]
    else:
        ids      = {s.strip() for s in install_ids.split(",")}
        targets  = [m for m in modules if m.id in ids]
        missing  = ids - {m.id for m in targets}
        if missing:
            valid = [m.id for m in modules]
            print(f"WARNING: Unknown IDs: {', '.join(missing)}")
            print(f"Valid IDs: {', '.join(valid)}")

    if not targets:
        print("No modules to install.")
        return

    print(f"\nWill install: {', '.join(m.name for m in targets)}")
    confirm("Proceed?")

    failed = []
    for m in targets:
        try:
            install_module(m, device, base_dir)
        except (MagiskError, AdbError) as e:
            print(f"[FAIL] {m.name}: {e}")
            failed.append(m.id)

    if failed:
        print(f"\nFailed: {', '.join(failed)}")
    ok = len(targets) - len(failed)
    print(f"\n[OK] {ok}/{len(targets)} modules installed.")
    if ok:
        print("     Reboot device to activate modules.")
