[Русский](INSTALL.md) · **English**

# Installation

## Archives and first launch

From [Releases](https://github.com/HubbaBubbaPrepod/pulse-monitor/releases), choose `pulse-monitor_v0.4.0_linux_amd64.tar.gz`, `pulse-monitor_v0.4.0_linux_arm64.tar.gz`, or `pulse-monitor_v0.4.0_windows_amd64.zip`. Compare SHA-256 with `checksums.txt`: Linux `sha256sum <archive>`, Windows `Get-FileHash <archive> -Algorithm SHA256`. Extract the executable, `out`, and `deployment` together. Keep `data`, tokens, and private keys outside `out`.

The first launch must use loopback; see the [quick start](../README.en.md#quick-start). The owner's password must contain at least 12 characters and at most 72 UTF-8 bytes. A second owner cannot register after the first. With an SSH tunnel, the server listens on `127.0.0.1:8080` and the browser opens the local tunnel address.

## HTTPS

Remote use requires DNS, a trusted certificate, and its private key. Supply `-tls-cert`, `-tls-key`, and `-public-url` together. The last argument is an HTTPS origin without a path, query, credentials, or fragment, such as `https://monitor.example.com:8443`. Open that port in the firewall. The process needs read access to the key and write access to its data directory. For an internal CA, add it to the agent OS and browser trust stores. TLS verification is not disabled.

## Automatic startup

Register using the same data directory as the installed service. Stop the manual instance before starting the service. If the owner already exists in the archive's `data` directory, stop the service and move the entire directory into the service's data location with the correct permissions; the installer does not migrate it. Linux uses `/var/lib/pulse-monitor` (owner `pulse-monitor`, mode `0700`); Windows uses `C:\ProgramData\PulseMonitor\data`. Otherwise, register on loopback in the new empty installed dashboard.

Linux, from the extracted archive:

```bash
sudo bash deployment/install-monitor.sh .
sudo systemctl status pulse-monitor
```

Defaults: loopback, `/opt/pulse-monitor`, and `/var/lib/pulse-monitor`. After installation, register through an SSH tunnel. For HTTPS, set `PULSE_ADDR`, `PULSE_PUBLIC_URL`, `PULSE_TLS_CERT`, and `PULSE_TLS_KEY` during installation, or change the unit arguments. Reinstallation preserves data. Check certificate paths and permissions before enabling remote listening.

Windows, in an administrative PowerShell:

```powershell
./deployment/install-monitor.ps1 -PackageDirectory .
```

This installs the `PulseMonitor` scheduled task under `C:\ProgramData\PulseMonitor` by default. For HTTPS, supply `-ListenAddress 0.0.0.0:8443 -PublicURL https://monitor.example.com:8443 -TLSCert C:\PulseTLS\fullchain.pem -TLSKey C:\PulseTLS\privkey.pem`. Register the owner in the installed dashboard's data directory before remote listening.

## Agent and source builds

Follow the [Pulse Agent guide](https://github.com/HubbaBubbaPrepod/pulse-agent/blob/main/README.en.md). Each node needs its own ID, token, and state directory. The address in the dashboard registry describes the node's location; the agent uses the dashboard's `-url`. Replacing a token revokes the old one: update the file and restart the agent.

Build from source with Node.js 22.13+, Go 1.23+, and PowerShell 7.4+. Go must be in PATH or supplied through `-GoExe`:

```powershell
npm ci
./scripts/build.ps1 -Version v0.4.0
```

Archives and `checksums.txt` appear in `artifacts/releases`. `-SkipFrontend` uses the existing `out`; the export must match the current source.
