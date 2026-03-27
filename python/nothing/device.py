"""Low-level ADB/fastboot wrappers and device detection."""

import os
import re
import subprocess
import sys
import time

from .exceptions import AdbError, FastbootTimeoutError, FlashError, FirmwareError
from .models import DeviceInfo

FASTBOOT_POLL_INTERVAL = 2    # seconds between fastboot device polls
FASTBOOT_POLL_TIMEOUT  = 40   # seconds max wait for fastboot


# ---------------------------------------------------------------------------
# Subprocess helpers
# ---------------------------------------------------------------------------

def run(args: list, check=True, capture=True) -> subprocess.CompletedProcess:
    return subprocess.run(
        args,
        capture_output=capture,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )


def adb_shell(cmd: str, serial: str) -> str:
    r = run(["adb", "-s", serial, "shell", cmd])
    if r.returncode != 0 and r.stderr:
        raise AdbError(f"adb shell '{cmd}' failed: {r.stderr.strip()}")
    return r.stdout.strip()


def adb_push(local, remote: str, serial: str) -> None:
    env = {**os.environ, "MSYS_NO_PATHCONV": "1"}
    r = subprocess.run(
        ["adb", "-s", serial, "push", str(local), remote],
        capture_output=True, text=True, env=env,
    )
    if r.returncode != 0:
        raise AdbError(f"adb push failed: {r.stderr.strip()}")


def adb_pull(remote: str, local, serial: str) -> None:
    env = {**os.environ, "MSYS_NO_PATHCONV": "1"}
    r = subprocess.run(
        ["adb", "-s", serial, "pull", remote, str(local)],
        capture_output=True, text=True, env=env,
    )
    if r.returncode != 0:
        raise AdbError(f"adb pull '{remote}' failed: {r.stderr.strip()}")


# ---------------------------------------------------------------------------
# Fastboot helpers
# ---------------------------------------------------------------------------

def fastboot_run(*args, serial: str) -> subprocess.CompletedProcess:
    r = run(["fastboot", "-s", serial, *args])
    if r.returncode != 0:
        raise FlashError(f"fastboot {' '.join(args)} failed: {r.stderr.strip()}")
    return r


def fastboot_flash(partition: str, image, serial: str) -> None:
    print(f"  Flashing {partition:<20} <- {image.name}")
    fastboot_run("flash", partition, str(image), serial=serial)


def fastboot_flash_ab(partition_base: str, image, serial: str) -> None:
    fastboot_flash(f"{partition_base}_a", image, serial)
    fastboot_flash(f"{partition_base}_b", image, serial)


def reboot_to_bootloader(serial: str) -> None:
    print("Rebooting to bootloader...")
    run(["adb", "-s", serial, "reboot", "bootloader"])
    wait_for_fastboot(serial)


def wait_for_fastboot(serial: str) -> None:
    print("Waiting for fastboot device", end="", flush=True)
    deadline = time.time() + FASTBOOT_POLL_TIMEOUT
    while time.time() < deadline:
        r = run(["fastboot", "devices"], check=False)
        if serial in r.stdout or (r.stdout.strip() and "fastboot" in r.stdout):
            print(" OK")
            return
        print(".", end="", flush=True)
        time.sleep(FASTBOOT_POLL_INTERVAL)
    print()
    raise FastbootTimeoutError(
        "Fastboot device not found after timeout.\n"
        "Check: USB cable and fastboot driver.\n"
        "  Windows : install WinUSB driver via Zadig (zadig.akeo.ie)\n"
        "  macOS   : brew install android-platform-tools\n"
        "  Linux   : add udev rules or run as root"
    )


def query_current_slot(serial: str) -> str:
    r = run(["fastboot", "-s", serial, "getvar", "current-slot"])
    out = r.stdout + r.stderr
    m = re.search(r"current-slot:\s*([ab])", out)
    return f"_{m.group(1)}" if m else "unknown"


# ---------------------------------------------------------------------------
# User interaction
# ---------------------------------------------------------------------------

def confirm(prompt: str) -> None:
    try:
        ans = input(f"{prompt} [y/N]: ").strip().lower()
    except (EOFError, KeyboardInterrupt):
        ans = ""
    if ans != "y":
        print("Aborted.")
        sys.exit(0)


# ---------------------------------------------------------------------------
# Device detection
# ---------------------------------------------------------------------------

def detect_device(serial: str | None) -> DeviceInfo:
    r = run(["adb", "devices", "-l"])
    lines = [l for l in r.stdout.splitlines()
             if " device" in l and not l.startswith("List")]

    if not lines:
        raise AdbError("No ADB device found. Check cable and USB debugging.")
    if len(lines) > 1 and not serial:
        serials = [l.split()[0] for l in lines]
        raise AdbError(f"Multiple devices found: {serials}. Use --serial to specify one.")

    detected_serial = serial or lines[0].split()[0]

    # Prefer the human-friendly brand name (e.g. "Nothing Phone (1)") over
    # the raw model code (e.g. "A063") which Nothing uses for EEA variants.
    brand_name   = run(["adb", "-s", detected_serial, "shell",
                        "getprop ro.product.brand_device_name"]).stdout.strip()
    model_code   = run(["adb", "-s", detected_serial, "shell",
                        "getprop ro.product.model"]).stdout.strip()
    # brand_device_name includes the manufacturer prefix ("Nothing Phone (1)").
    # Strip it so cli.py can prepend "Nothing " uniformly without duplication.
    if brand_name.lower().startswith("nothing "):
        brand_name = brand_name[8:]
    model        = brand_name or model_code
    manufacturer = run(["adb", "-s", detected_serial, "shell",
                        "getprop ro.product.manufacturer"]).stdout.strip()
    codename     = run(["adb", "-s", detected_serial, "shell",
                        "getprop ro.product.device"]).stdout.strip().capitalize()
    slot         = run(["adb", "-s", detected_serial, "shell",
                        "getprop ro.boot.slot_suffix"]).stdout.strip()

    if "nothing" not in manufacturer.lower():
        raise FirmwareError(
            f"Not a Nothing device (manufacturer: {manufacturer}). "
            "This tool only supports Nothing devices."
        )

    return DeviceInfo(serial=detected_serial, model=model, codename=codename, current_slot=slot)
