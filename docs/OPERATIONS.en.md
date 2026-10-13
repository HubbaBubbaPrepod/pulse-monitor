[Русский](OPERATIONS.md) · **English**

# Operations

## Data and backups

All paths are relative to `-data`. `account.json` contains the owner and bcrypt hash; `servers.json` the registry; `preferences.json` theme and language. `agents/*.json` contains token hashes and latest reports; `agents/*.jsonl` metric history; `logs.jsonl` the log archive; `events.json` events and markers. `modules`, `module-config`, and `module-data` contain module packages, settings, and results. Password fields remain in server configuration; hiding an input does not encrypt it or prevent owner access through the API.

Stop the dashboard, copy the **entire data directory** to a protected location, then restart. Back up TLS keys, service configuration, and protected token files on nodes separately. To restore, stop the service and replace the complete data directory with its original permissions. Signing out revokes the session and does not delete data. Sessions live in memory: sign in again after a restart.

Metrics are written every 10 seconds; charts read the last 24 hours. JSONL archives are not automatically pruned; monitor free disk space. System logs are collected from agent startup; crash recovery depends on retention of the OS logs.

## Upgrades and removal

1. Read both applications' changelogs and create a backup.
2. Stop the dashboard and agents. Linux: `sudo systemctl stop pulse-monitor` / `pulse-agent`. Windows: `Stop-ScheduledTask -TaskName PulseMonitor` / `Stop-Service PulseAgent`.
3. Replace the dashboard executable and `out` from the new archive; replace agent executables. Preserve data, tokens, and agent state.
4. Start the services and sign in again. Check reports and modules. For rollback, restore the matching executables, `out`, and data backup; do not assume newer files are backwards compatible.

On Linux, stop and disable the unit (`sudo systemctl disable --now pulse-monitor` or `pulse-agent`), remove its file from `/etc/systemd/system`, run `sudo systemctl daemon-reload`, then remove the program directory. On Windows, stop and remove the task (`Unregister-ScheduledTask -TaskName PulseMonitor -Confirm:$false`); for the agent, use `Stop-Service PulseAgent` and `sc.exe delete PulseAgent`. Delete data, state, and tokens separately only when no longer needed. Removing a node from the dashboard preserves its log archive.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Dashboard refuses remote startup | Owner exists in the active `-data`; certificate, key, and `-public-url` are supplied together |
| TLS error | Domain, certificate expiry, full chain, OS trust; URL has no path |
| Node waits for data / offline | `-server`, token, URL, firewall, outbound access; `pulse-agent -check` checks local collection |
| Agent receives HTTP 401 | Token replaced or revoked; update the file and restart |
| Missing GPU / logs / processes | Agent permissions, driver, `nvidia-smi`; some OS metrics are unavailable |
| Missing packets | TShark, Npcap on Windows, capture permissions, `tshark -D`, and `-capture` |
| Module error / unsupported | Architecture build, resource permissions, configuration, timeout, and JSON protocol |

Linux: `sudo journalctl -u pulse-monitor -f` / `pulse-agent`. Manual startup prints errors in the terminal. Do not publish tokens, TLS keys, private configuration, or unredacted logs in Issues.
