"""System monitor: RAM and CPU usage for Nothing phones."""

import re
import time

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo


# ── helpers ──────────────────────────────────────────────────────────────────

def _ascii_bar(value: float, total: float, width: int = 24) -> str:
    """Return a filled ASCII progress bar representing value/total."""
    if total <= 0:
        return " " * width
    pct = min(1.0, value / total)
    filled = int(pct * width)
    return "█" * filled + "░" * (width - filled)


def _mib(kb: int) -> str:
    """Convert kilobytes to a human-readable MiB/GiB string."""
    mib = kb / 1024
    if mib >= 1024:
        return f"{mib / 1024:.2f} GiB"
    return f"{mib:.0f} MiB"


def _mhz(hz: int) -> str:
    """Convert Hz (from sysfs, already in kHz) to a GHz/MHz string."""
    if hz >= 1_000_000:
        return f"{hz / 1_000_000:.2f} GHz"
    return f"{hz / 1_000:.0f} MHz"


# ── /proc/meminfo parsing ─────────────────────────────────────────────────────

def _read_proc_meminfo(serial: str) -> dict[str, int]:
    """Return a dict of meminfo key → value in kB."""
    r = run(["adb", "-s", serial, "shell", "cat /proc/meminfo"])
    result: dict[str, int] = {}
    for line in r.stdout.splitlines():
        m = re.match(r"^(\w[\w()]+):\s+(\d+)", line)
        if m:
            try:
                result[m.group(1)] = int(m.group(2))
            except ValueError:
                pass
    return result


# ── dumpsys meminfo parsing ───────────────────────────────────────────────────

def _parse_rss_summary(output: str) -> list[tuple[int, str]]:
    """
    Parse 'Total RSS by process' section from 'dumpsys meminfo'.
    Returns list of (rss_kb, name) sorted descending.
    """
    entries: list[tuple[int, str]] = []
    in_section = False
    for line in output.splitlines():
        if "Total RSS by process" in line:
            in_section = True
            continue
        if in_section:
            # Section ends at blank line or next section header
            stripped = line.strip()
            if not stripped:
                break
            # Format: "    539,556K: system (pid 1562)"
            m = re.match(r"^\s*([\d,]+)K:\s+(.+)$", stripped)
            if m:
                try:
                    kb = int(m.group(1).replace(",", ""))
                    name_raw = m.group(2)
                    # strip trailing " (pid NNNN)" or " (pid NNNN / activities)"
                    name = re.sub(r"\s*\(pid\s+\d+.*?\)\s*$", "", name_raw).strip()
                    entries.append((kb, name))
                except ValueError:
                    pass
    return entries


def _parse_app_meminfo(output: str) -> list[tuple[str, int]]:
    """
    Parse 'dumpsys meminfo <package>' detailed output.
    Returns list of (label, kb) for interesting rows.
    """
    # We look for lines like:
    #   "   Java Heap:    25732    26748    12048        0      232     2396      0"
    # The first numeric column is the PSS total; we grab (label, pss_total).
    rows = []
    for line in output.splitlines():
        m = re.match(r"^\s{0,10}([\w ]+?):\s{1,10}(\d+)", line)
        if m:
            label = m.group(1).strip()
            if label in (
                "Java Heap", "Native Heap", "Code", "Stack",
                "Graphics", "Private Other", "System", "Unknown",
                "TOTAL PSS", "TOTAL RSS", "TOTAL SWAP",
            ):
                try:
                    rows.append((label, int(m.group(2))))
                except ValueError:
                    pass
    return rows


# ── memory action ─────────────────────────────────────────────────────────────

def action_memory(device: DeviceInfo, package: str | None, watch: bool) -> None:
    """
    Display RAM usage.

    Without --package: /proc/meminfo summary + top-10 RSS processes.
    With    --package: detailed per-app breakdown from 'dumpsys meminfo <pkg>'.
    With --watch     : refresh every 2 seconds.
    """

    def _snapshot_system() -> None:
        mem = _read_proc_meminfo(device.serial)
        total  = mem.get("MemTotal", 0)
        free   = mem.get("MemFree", 0)
        avail  = mem.get("MemAvailable", 0)
        bufs   = mem.get("Buffers", 0)
        cached = mem.get("Cached", 0)
        used   = total - avail

        print(f"\n  RAM Summary — {device.model}\n")
        print(f"  {'Total':<16}: {_mib(total):>10}")
        print(f"  {'Used (est.)':<16}: {_mib(used):>10}  {_ascii_bar(used, total)}")
        print(f"  {'Available':<16}: {_mib(avail):>10}  {_ascii_bar(avail, total)}")
        print(f"  {'Free':<16}: {_mib(free):>10}")
        print(f"  {'Buffers':<16}: {_mib(bufs):>10}")
        print(f"  {'Cached':<16}: {_mib(cached):>10}")

        # Swap
        swap_total = mem.get("SwapTotal", 0)
        swap_free  = mem.get("SwapFree", 0)
        if swap_total:
            swap_used = swap_total - swap_free
            print(f"\n  {'Swap total':<16}: {_mib(swap_total):>10}")
            print(f"  {'Swap used':<16}: {_mib(swap_used):>10}  {_ascii_bar(swap_used, swap_total)}")

        # Top processes — grab enough lines to cover the RSS section
        r2 = run(["adb", "-s", device.serial, "shell",
                  "dumpsys meminfo 2>/dev/null | head -120"])
        entries = _parse_rss_summary(r2.stdout)
        if entries:
            print(f"\n  Top processes by RSS:\n")
            print(f"  {'Process':<48} {'RSS':>10}")
            print("  " + "─" * 62)
            for kb, name in entries[:10]:
                bar = _ascii_bar(kb, entries[0][0], 12)
                print(f"  {name:<48} {_mib(kb):>10}  {bar}")

        # LowMemoryKiller
        r3 = run(["adb", "-s", device.serial, "shell",
                  "cat /sys/module/lowmemorykiller/parameters/minfree 2>/dev/null"])
        if r3.returncode == 0 and r3.stdout.strip() and r3.stdout.strip() != "N/A":
            pages = r3.stdout.strip().split(",")
            adj_levels = ["foreground", "visible", "secondary_server",
                          "hidden", "content_provider", "empty"]
            print(f"\n  LowMemoryKiller minfree thresholds (pages × 4 kB):")
            for i, p in enumerate(pages):
                try:
                    kb = int(p.strip()) * 4
                    label = adj_levels[i] if i < len(adj_levels) else f"level{i}"
                    print(f"    {label:<22}: {_mib(kb)}")
                except ValueError:
                    pass
        print()

    def _snapshot_package(pkg: str) -> None:
        r = run(["adb", "-s", device.serial, "shell",
                 f"dumpsys meminfo {pkg} 2>/dev/null"])
        output = r.stdout.strip()
        if not output or "No process found" in output or "No services found" in output:
            print(f"\n  Package '{pkg}' not found or not running on {device.model}.\n")
            return

        rows = _parse_app_meminfo(output)

        print(f"\n  Memory detail — {pkg}\n  Device: {device.model}\n")
        if rows:
            print(f"  {'Category':<20} {'PSS total':>10}  {'bar'}")
            print("  " + "─" * 52)
            # Find TOTAL PSS for bar scaling
            total_pss = next((v for k, v in rows if k == "TOTAL PSS"), 0)
            scale = total_pss if total_pss else max(v for _, v in rows)
            for label, kb in rows:
                bar = _ascii_bar(kb, scale) if scale else ""
                print(f"  {label:<20} {_mib(kb):>10}  {bar}")
        else:
            # Fallback: print raw output
            for line in output.splitlines()[:40]:
                print(f"  {line}")
        print()

    if watch:
        label = f"live memory — {package}" if package else "live memory"
        print(f"  Memory monitor (Ctrl-C to stop, refresh every 2s)\n")
        try:
            while True:
                print("\033[2J\033[H", end="")
                print(f"  Nothing {device.model}  —  {label}\n")
                if package:
                    _snapshot_package(package)
                else:
                    _snapshot_system()
                time.sleep(2)
        except KeyboardInterrupt:
            print("\nStopped.")
    else:
        if package:
            _snapshot_package(package)
        else:
            _snapshot_system()


# ── CPU frequency helpers ─────────────────────────────────────────────────────

def _read_cpu_freqs(serial: str) -> list[dict]:
    """
    Read per-core frequency, max-frequency, and online status via a single
    adb shell call. Returns a list of dicts with keys:
        core, cur_hz, max_hz, online
    """
    # One shell invocation for all 8 possible cores (avoid seq, not always present)
    script = (
        "for i in 0 1 2 3 4 5 6 7; do "
        "  p=/sys/devices/system/cpu/cpu$i; "
        "  [ -d $p ] || continue; "
        "  cur=$(cat $p/cpufreq/scaling_cur_freq 2>/dev/null || echo 0); "
        "  mx=$(cat $p/cpufreq/cpuinfo_max_freq 2>/dev/null || echo 0); "
        "  onl=$(cat $p/online 2>/dev/null || echo 1); "
        "  echo \"$i|$cur|$mx|$onl\"; "
        "done"
    )
    r = run(["adb", "-s", serial, "shell", script])
    cores = []
    for line in r.stdout.strip().splitlines():
        parts = line.split("|")
        if len(parts) == 4:
            try:
                core   = int(parts[0])
                cur_hz = int(parts[1]) if parts[1] else 0
                max_hz = int(parts[2]) if parts[2] else 0
                online = parts[3].strip() not in ("0",)
                cores.append({"core": core, "cur_hz": cur_hz,
                               "max_hz": max_hz, "online": online})
            except ValueError:
                pass
    return cores


def _classify_snapdragon_cluster(core: int, max_hz: int) -> str:
    """
    Classify a Snapdragon lahaina (8 Gen 1) core by index/max-freq.
    cpu0-3  → Silver (efficiency)   ~1804 MHz max
    cpu4-6  → Gold   (performance)  ~2400 MHz max
    cpu7    → Prime  (prime)        ~3187/2515 MHz max
    """
    if core <= 3:
        return "Silver"
    if core == 7:
        return "Prime"
    return "Gold"


def _classify_mtk_cluster(core: int, max_hz: int) -> str:
    """
    Classify a MediaTek mt6878 core by max frequency.
    mt6878 (Dimensity 7300): cpu0-3 small ~2000 MHz, cpu4-7 big ~2500 MHz
    """
    if max_hz <= 2_100_000:
        return "Efficiency"
    return "Performance"


def _detect_soc(serial: str) -> str:
    """Return 'snapdragon', 'mediatek', or 'unknown'."""
    r = run(["adb", "-s", serial, "shell", "getprop ro.board.platform"])
    platform = r.stdout.strip().lower()
    if platform.startswith("mt") or platform.startswith("dimensity"):
        return "mediatek"
    # Snapdragon platform strings: lahaina, taro, kalama, pineapple, etc.
    if platform and platform not in ("", "unknown"):
        return "snapdragon"
    return "unknown"


def _classify_cluster(soc: str, core: int, max_hz: int) -> str:
    if soc == "mediatek":
        return _classify_mtk_cluster(core, max_hz)
    return _classify_snapdragon_cluster(core, max_hz)


# ── top-process parsing ───────────────────────────────────────────────────────

def _read_top_processes(serial: str, top_n: int) -> list[tuple[float, int, str]]:
    """
    Run 'top -b -n 1' and return top_n processes sorted by %CPU descending.
    Returns list of (cpu_pct, pid, name).
    """
    r = run(["adb", "-s", serial, "shell",
             "top -b -n 1 -o PID,USER,%CPU,%MEM,ARGS 2>/dev/null"])
    procs: list[tuple[float, int, str]] = []
    header_seen = False
    for line in r.stdout.splitlines():
        # Header line starts with PID
        if re.match(r"^\s*PID\s+USER", line):
            header_seen = True
            continue
        if not header_seen:
            continue
        parts = line.split()
        if len(parts) < 5:
            continue
        try:
            pid     = int(parts[0])
            cpu_pct = float(parts[2])
            name    = parts[4]
            procs.append((cpu_pct, pid, name))
        except (ValueError, IndexError):
            pass

    procs.sort(key=lambda x: x[0], reverse=True)
    return procs[:top_n]


# ── CPU usage action ──────────────────────────────────────────────────────────

def action_cpu_usage(device: DeviceInfo, top_n: int, watch: bool) -> None:
    """
    Display CPU frequency per core and top-N processes by CPU usage.
    With --watch: refresh every 2 seconds.
    """
    soc = _detect_soc(device.serial)

    def _snapshot() -> None:
        cores = _read_cpu_freqs(device.serial)

        if not cores:
            print("  Could not read CPU frequency data.")
            return

        print(f"\n  CPU Cores — {device.model}  (SoC: {soc})\n")
        print(f"  {'Core':<8} {'Cluster':<13} {'Status':<8} "
              f"{'Current':>10}  {'Max':>10}  {'Load'}")
        print("  " + "─" * 72)

        for c in cores:
            core   = c["core"]
            cur    = c["cur_hz"]
            maxf   = c["max_hz"]
            online = c["online"]
            cluster = _classify_cluster(soc, core, maxf)
            status  = "online " if online else "offline"
            cur_str = _mhz(cur) if (online and cur > 0) else "—"
            max_str = _mhz(maxf) if maxf > 0 else "—"
            bar = _ascii_bar(cur, maxf, 20) if (online and cur > 0 and maxf > 0) else " " * 20
            pct_str = f"({cur / maxf * 100:4.0f}%)" if (online and cur > 0 and maxf > 0) else "      "
            print(f"  cpu{core:<5} {cluster:<13} {status:<8} "
                  f"{cur_str:>10}  {max_str:>10}  {bar} {pct_str}")

        # Overall CPU utilisation summary from top header
        r_top = run(["adb", "-s", device.serial, "shell",
                     "top -b -n 1 2>/dev/null | grep -E '%cpu'"])
        if r_top.returncode == 0 and r_top.stdout.strip():
            # Keep only the first matching line (the cpu% summary)
            cpu_line = r_top.stdout.strip().splitlines()[0].strip()
            print(f"\n  Overall: {cpu_line}")

        # Top-N processes
        procs = _read_top_processes(device.serial, top_n)
        if procs:
            print(f"\n  Top {top_n} processes by CPU:\n")
            print(f"  {'%CPU':>6}  {'PID':>7}  {'Name'}")
            print("  " + "─" * 50)
            for cpu_pct, pid, name in procs:
                bar = _ascii_bar(cpu_pct, 100, 14)
                print(f"  {cpu_pct:>5.1f}%  {pid:>7}  {name:<30}  {bar}")
        print()

    if watch:
        print(f"  CPU monitor (Ctrl-C to stop, refresh every 2s)\n")
        try:
            while True:
                print("\033[2J\033[H", end="")
                print(f"  Nothing {device.model}  —  live CPU\n")
                _snapshot()
                time.sleep(2)
        except KeyboardInterrupt:
            print("\nStopped.")
    else:
        _snapshot()
