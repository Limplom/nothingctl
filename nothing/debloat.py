"""NothingOS bloatware removal via pm uninstall --user 0 (reversible)."""

import json
from dataclasses import dataclass
from pathlib import Path

from .device import confirm, run
from .exceptions import AdbError
from .models import DeviceInfo

_DEBLOAT_JSON = Path(__file__).parent.parent / "debloat.json"


@dataclass
class PackageEntry:
    id:       str
    package:  str
    name:     str
    category: str
    notes:    str = ""


def load_packages(json_path: Path = _DEBLOAT_JSON) -> list[PackageEntry]:
    with open(json_path, encoding="utf-8") as f:
        data = json.load(f)
    return [PackageEntry(**p) for p in data["packages"]]


def _is_installed(package: str, serial: str) -> bool:
    r = run(["adb", "-s", serial, "shell", f"pm list packages {package}"])
    return f"package:{package}" in r.stdout


def action_debloat(device: DeviceInfo, remove: str | None) -> None:
    packages = load_packages()

    if not remove:
        # ── List mode ────────────────────────────────────────────────────────
        print(f"\n{'ID':<22} {'Name':<28} {'Status':<14} Notes")
        print("─" * 90)
        for p in packages:
            status = "INSTALLED" if _is_installed(p.package, device.serial) else "not installed"
            marker = "  " if status == "not installed" else "->"
            print(f" {marker} {p.id:<20} {p.name:<28} {status:<14} {p.notes}")
        print()
        print("Remove:  python nothingctl.py --debloat --remove <id,id,...|all>")
        print("Restore: adb shell pm install-existing --user 0 <package>")
        return

    # ── Remove mode ──────────────────────────────────────────────────────────
    if remove == "all":
        targets = packages
    else:
        ids     = {x.strip() for x in remove.split(",")}
        targets = [p for p in packages if p.id in ids]
        missing = ids - {p.id for p in targets}
        if missing:
            raise AdbError(f"Unknown package ID(s): {', '.join(sorted(missing))}\n"
                           f"Run --debloat without --remove to see available IDs.")

    installed = [p for p in targets if _is_installed(p.package, device.serial)]
    if not installed:
        print("\nAll selected packages are already removed.")
        return

    print(f"\nWill disable {len(installed)} package(s) for user 0 (fully reversible):")
    for p in installed:
        print(f"  {p.name:<28} {p.package}")

    confirm("\nProceed?")

    failed = []
    for p in installed:
        print(f"  Removing {p.name}...", end=" ", flush=True)
        r = run(["adb", "-s", device.serial, "shell",
                 f"pm uninstall --user 0 {p.package}"])
        if "Success" in r.stdout or r.returncode == 0:
            print("OK")
        else:
            print(f"FAILED ({r.stdout.strip() or r.stderr.strip()})")
            failed.append(p)

    print()
    if failed:
        print(f"WARNING: {len(failed)} package(s) could not be removed: "
              f"{', '.join(p.name for p in failed)}")
    ok = len(installed) - len(failed)
    print(f"[OK] {ok}/{len(installed)} packages removed.")
    print("\nTo restore any package:")
    for p in installed:
        print(f"  adb shell pm install-existing --user 0 {p.package}")
