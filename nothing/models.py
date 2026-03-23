"""Dataclasses shared across modules."""

from dataclasses import dataclass
from pathlib import Path


@dataclass
class MagiskStatus:
    app_installed:       bool
    root_active:         bool        # /data/adb/magisk present + su works
    installed_version:   int | None  # daemon version code (e.g. 30700)
    latest_version:      int | None  # from GitHub (e.g. 30700)
    latest_version_str:  str | None  # human-readable (e.g. "30.7")
    latest_apk_url:      str | None

    @property
    def is_outdated(self) -> bool:
        if not self.app_installed or self.latest_version is None:
            return False
        current = self.installed_version
        return current is not None and current < self.latest_version

    @property
    def state_label(self) -> str:
        if not self.app_installed:
            return "NOT INSTALLED"
        if not self.root_active:
            return "APP ONLY (boot not patched)"
        if self.is_outdated:
            return f"ACTIVE but OUTDATED (v{self.installed_version} < v{self.latest_version})"
        return f"ACTIVE  v{self.installed_version}"


@dataclass
class BootTarget:
    filename:       str   # "init_boot.img" or "boot.img"
    partition_base: str   # "init_boot" or "boot"
    is_gki2:        bool  # True = GKI 2.0 device (Nothing Phone 2+)


@dataclass
class DeviceInfo:
    serial:       str
    model:        str
    codename:     str   # e.g. "Galaxian", "Spacewar", "Pong"
    current_slot: str   # "_a" or "_b" or ""


@dataclass
class FirmwareState:
    extracted_dir: Path
    version:       str
    is_newer:      bool
    boot_target:   BootTarget
