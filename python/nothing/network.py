"""Network information and DNS management for Nothing phones."""

import re

from .device import run
from .exceptions import AdbError
from .models import DeviceInfo

# ---------------------------------------------------------------------------
# Known Private DNS aliases → real hostnames
# ---------------------------------------------------------------------------

_DNS_ALIASES: dict[str, str] = {
    "cloudflare": "one.one.one.one",
    "1.1.1.1":    "one.one.one.one",
    "adguard":    "dns.adguard.com",
    "google":     "dns.google",
    "quad9":      "dns.quad9.net",
}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _shell(serial: str, cmd: str) -> str:
    """Run adb shell command; return stdout stripped, empty string on failure."""
    r = run(["adb", "-s", serial, "shell", cmd])
    return r.stdout.strip() if r.returncode == 0 else ""


def _setting(serial: str, namespace: str, key: str) -> str:
    """Read an Android setting via 'adb shell settings get'."""
    r = run(["adb", "-s", serial, "shell", "settings", "get", namespace, key])
    val = r.stdout.strip() if r.returncode == 0 else ""
    return "" if val in ("null", "null\n") else val


# ---------------------------------------------------------------------------
# 1. Network Info
# ---------------------------------------------------------------------------

def action_network_info(device: DeviceInfo) -> None:
    """Display network information for the connected Nothing phone."""
    s = device.serial

    # ── WiFi — use 'cmd wifi status' (stable across Android versions) ───────
    wifi_raw = _shell(s, "cmd wifi status")

    ssid       = ""
    bssid      = ""
    rssi       = ""
    link_speed = ""
    freq       = ""
    ip_wifi    = ""

    # Find the WifiInfo: line which contains all data in one line
    wifi_info_line = ""
    for line in wifi_raw.splitlines():
        if "WifiInfo:" in line:
            wifi_info_line = line
            break

    if wifi_info_line:
        m = re.search(r'SSID:\s*"([^"]*)"', wifi_info_line)
        if m:
            ssid = m.group(1)
        m = re.search(r'BSSID:\s*([0-9a-fA-F:]{17})', wifi_info_line)
        if m:
            bssid = m.group(1)
        m = re.search(r'RSSI:\s*(-?\d+)', wifi_info_line)
        if m:
            rssi = m.group(1)
        m = re.search(r'Link speed:\s*(\d+)', wifi_info_line)
        if m:
            link_speed = m.group(1)
        m = re.search(r'Frequency:\s*(\d+)', wifi_info_line)
        if m:
            freq = m.group(1)
        m = re.search(r'IP:\s*/([\d.]+)', wifi_info_line)
        if m:
            ip_wifi = m.group(1)

    # Fallback: ip addr show wlan0
    if not ip_wifi:
        wlan_raw = _shell(s, "ip addr show wlan0")
        m = re.search(r'inet\s+([\d.]+)/', wlan_raw)
        if m:
            ip_wifi = m.group(1)

    # Frequency band
    freq_band = ""
    if freq:
        try:
            f = int(freq)
            if f < 3000:
                freq_band = "2.4 GHz"
            elif f < 6000:
                freq_band = "5 GHz"
            else:
                freq_band = "6 GHz"
        except ValueError:
            pass

    # ── DNS ────────────────────────────────────────────────────────────────
    # Current DNS servers from /etc/resolv.conf or ndc resolver
    dns_raw = _shell(s, "getprop | grep -E 'net\\.dns[12]|dhcp.*dns'")
    dns_servers: list[str] = []
    for line in dns_raw.splitlines():
        m = re.search(r'net\.dns[12]\]:\s*\[([^\]]+)\]', line)
        if m and m.group(1).strip():
            dns_servers.append(m.group(1).strip())

    if not dns_servers:
        # Try ndc resolver dump
        ndc_raw = _shell(s, "ndc resolver getnetworkinfo 100")
        for line in ndc_raw.splitlines():
            m = re.search(r'DNS servers:\s*(.+)', line)
            if m:
                dns_servers = [d.strip() for d in m.group(1).split() if d.strip()]
                break

    # ── Private DNS ────────────────────────────────────────────────────────
    pdns_mode     = _setting(s, "global", "private_dns_mode")
    pdns_provider = _setting(s, "global", "private_dns_specifier")

    # ── Mobile network ─────────────────────────────────────────────────────
    operator    = _shell(s, "getprop gsm.operator.alpha")
    net_type    = _shell(s, "getprop gsm.network.type")

    # Multi-SIM: getprop may return comma-separated list — take the first non-empty
    if "," in operator:
        operator = next((o.strip() for o in operator.split(",") if o.strip()), "")
    if "," in net_type:
        net_type = next((n.strip() for n in net_type.split(",") if n.strip()), "")

    # ── Active connection type ─────────────────────────────────────────────
    conn_raw  = _shell(s, "dumpsys connectivity | grep -E 'NetworkAgentInfo.*CONNECTED|activeNetwork'")
    conn_type = "Unknown"
    if "WIFI" in conn_raw.upper():
        conn_type = "WiFi"
    elif "CELLULAR" in conn_raw.upper() or "MOBILE" in conn_raw.upper():
        conn_type = "Mobile"
    elif not conn_raw:
        conn_type = "None"

    # ── Output ────────────────────────────────────────────────────────────
    print(f"\n  Network Info — {device.model}\n")

    print(f"  {'Connection':<18}: {conn_type}")
    print()

    print(f"  WiFi")
    print(f"  {'  SSID':<18}: {ssid or '(not connected)'}")
    if ssid:
        print(f"  {'  BSSID':<18}: {bssid or 'n/a'}")
        print(f"  {'  Signal':<18}: {rssi + ' dBm' if rssi else 'n/a'}")
        print(f"  {'  Link Speed':<18}: {link_speed + ' Mbps' if link_speed else 'n/a'}")
        freq_display = f"{freq} MHz ({freq_band})" if freq and freq_band else (freq or "n/a")
        print(f"  {'  Frequency':<18}: {freq_display}")
        print(f"  {'  IP Address':<18}: {ip_wifi or 'n/a'}")
    print()

    print(f"  DNS")
    if dns_servers:
        for i, srv in enumerate(dns_servers, 1):
            print(f"  {'  Server ' + str(i):<18}: {srv}")
    else:
        print(f"  {'  Servers':<18}: (not available)")

    pdns_display = pdns_mode or "off"
    if pdns_mode == "hostname" and pdns_provider:
        pdns_display = f"hostname ({pdns_provider})"
    elif pdns_mode == "opportunistic":
        pdns_display = "automatic"
    print(f"  {'  Private DNS':<18}: {pdns_display}")
    print()

    print(f"  Mobile")
    print(f"  {'  Operator':<18}: {operator or 'n/a'}")
    print(f"  {'  Network Type':<18}: {net_type or 'n/a'}")
    print()


# ---------------------------------------------------------------------------
# 2. DNS Set
# ---------------------------------------------------------------------------

def action_dns_set(device: DeviceInfo, provider: str | None) -> None:
    """Set or display the Private DNS configuration."""
    s = device.serial

    # ── Read-only mode ─────────────────────────────────────────────────────
    if provider is None:
        mode     = _setting(s, "global", "private_dns_mode")
        specifier = _setting(s, "global", "private_dns_specifier")

        print(f"\n  Private DNS — {device.model}\n")
        mode_display = mode or "off"
        if mode == "hostname":
            mode_display = f"hostname"
        elif mode == "opportunistic":
            mode_display = "automatic (opportunistic)"
        print(f"  {'Mode':<12}: {mode_display}")
        if mode == "hostname":
            print(f"  {'Provider':<12}: {specifier or '(not set)'}")
        print()
        return

    # ── Disable ────────────────────────────────────────────────────────────
    if provider.lower() == "off":
        r = run(["adb", "-s", s, "shell", "settings", "put", "global",
                 "private_dns_mode", "off"])
        if r.returncode != 0:
            raise AdbError(f"Failed to disable Private DNS: {r.stderr.strip()}")
        print(f"  Private DNS disabled on {device.model}.")
        return

    # ── Resolve alias ──────────────────────────────────────────────────────
    hostname = _DNS_ALIASES.get(provider.lower(), provider)

    # ── Set hostname mode ──────────────────────────────────────────────────
    r1 = run(["adb", "-s", s, "shell", "settings", "put", "global",
              "private_dns_specifier", hostname])
    r2 = run(["adb", "-s", s, "shell", "settings", "put", "global",
              "private_dns_mode", "hostname"])

    if r1.returncode != 0 or r2.returncode != 0:
        raise AdbError(
            f"Failed to set Private DNS: "
            f"{(r1.stderr or r2.stderr).strip()}"
        )

    alias_note = f" (alias for {hostname})" if hostname != provider.lower() else ""
    print(f"  Private DNS set to '{hostname}'{alias_note} on {device.model}.")


# ---------------------------------------------------------------------------
# 3. Port Forwarding
# ---------------------------------------------------------------------------

def action_port_forward(
    device: DeviceInfo,
    local: str | None,
    remote: str | None,
    clear: bool,
) -> None:
    """Manage ADB port forwards for the connected Nothing phone."""
    s = device.serial

    # ── Remove all ────────────────────────────────────────────────────────
    if clear:
        r = run(["adb", "-s", s, "forward", "--remove-all"])
        if r.returncode != 0:
            raise AdbError(f"Failed to remove forwards: {r.stderr.strip()}")
        print(f"  All port forwards removed on {device.model}.")
        return

    # ── Add new forward ───────────────────────────────────────────────────
    if local is not None and remote is not None:
        local_spec  = f"tcp:{local}"
        remote_spec = f"tcp:{remote}"
        r = run(["adb", "-s", s, "forward", local_spec, remote_spec])
        if r.returncode != 0:
            raise AdbError(
                f"Failed to create forward {local_spec} -> {remote_spec}: "
                f"{r.stderr.strip()}"
            )
        print(f"  Forward added: {local_spec} -> {remote_spec} on {device.model}.")
        return

    # ── List active forwards ──────────────────────────────────────────────
    r_fwd = run(["adb", "-s", s, "forward", "--list"])
    r_rev = run(["adb", "-s", s, "reverse", "--list"])

    fwd_lines = [l.strip() for l in r_fwd.stdout.splitlines() if l.strip()] \
        if r_fwd.returncode == 0 else []
    rev_lines = [l.strip() for l in r_rev.stdout.splitlines() if l.strip()] \
        if r_rev.returncode == 0 else []

    print(f"\n  Port Forwards — {device.model}\n")

    print(f"  Forwards (host -> device):")
    if fwd_lines:
        for line in fwd_lines:
            # Format: <serial> <local> <remote>
            parts = line.split()
            if len(parts) >= 3:
                print(f"    {parts[1]}  ->  {parts[2]}")
            else:
                print(f"    {line}")
    else:
        print("    (none)")

    print()
    print(f"  Reverse forwards (device -> host):")
    if rev_lines:
        for line in rev_lines:
            parts = line.split()
            if len(parts) >= 3:
                print(f"    {parts[1]}  ->  {parts[2]}")
            else:
                print(f"    {line}")
    else:
        print("    (none)")

    print()
