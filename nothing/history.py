"""Flash history — automatic logging of flash operations and display."""

import json
import datetime
from pathlib import Path

_HISTORY_FILENAME = "flash_history.json"


def log_flash(base_dir: Path, entry: dict) -> None:
    """Append a flash event to flash_history.json."""
    history_file = base_dir / _HISTORY_FILENAME
    records: list = []
    if history_file.exists():
        try:
            records = json.loads(history_file.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            records = []
    entry.setdefault("timestamp", datetime.datetime.now().isoformat(timespec="seconds"))
    records.append(entry)
    history_file.write_text(json.dumps(records, indent=2), encoding="utf-8")


def action_history(base_dir: Path) -> None:
    """Display the flash history log."""
    history_file = base_dir / _HISTORY_FILENAME
    if not history_file.exists():
        print("\nNo flash history yet.")
        print("History is recorded automatically after each --flash-firmware or --ota-update.")
        return

    try:
        records = json.loads(history_file.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        print(f"\nCould not read flash history: {e}")
        return

    if not records:
        print("\nFlash history is empty.")
        return

    print(f"\nFlash history ({len(records)} entries)  —  {history_file}\n")
    print(f"  {'#':<4} {'Timestamp':<22} {'Operation':<18} {'Version':<36} {'ARB':<5} Serial")
    print("  " + "─" * 100)

    for i, r in enumerate(reversed(records)):
        ts        = r.get("timestamp", "?")[:19].replace("T", " ")
        op        = r.get("operation", "?")
        version   = r.get("version",   "?")
        arb       = str(r.get("arb_index", "?"))
        serial    = r.get("serial",    "?")
        print(f"  {i:<4} {ts:<22} {op:<18} {version:<36} {arb:<5} {serial}")

    print()
