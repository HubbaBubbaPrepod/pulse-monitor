[Русский](OPERATIONS.md) · **English**

# Operations

## Data and backups

All paths are relative to `-data`. `account.json` contains the owner and bcrypt hash; `servers.json` the registry; `preferences.json` theme and language. `pulse.db` stores agent receipts, reports, metric samples, logs, events and retention policies in SQLite WAL mode. `pulse.db-wal` and `pulse.db-shm` are live SQLite files: never delete or copy them independently while Monitor is running. Legacy JSONL/JSON archives are migration inputs, not the active archive. Module packages, settings, results and state remain in their respective private directories. Password fields are stored in private configuration, not encrypted storage.

Stop the dashboard, copy the **entire data directory** to a protected location, then restart. Back up TLS keys, service configuration, and protected token files on nodes separately. To restore, stop the service and replace the complete data directory with its original permissions. Signing out revokes the session and does not delete data. Sessions live in memory: sign in again after a restart.

Metrics are written every 10 seconds; charts read the last 24 hours. Log retention is configured per server; zero disables expiry. Cleanup runs in bounded batches each minute. History reads have a five-second deadline and recent-log polling avoids archive counts. Passive WAL checkpoints report a backlog over 64 MiB; journal reuse is limited to 16 MiB after successful recycling, not a hard WAL quota. Telemetry ingestion returns 503 below 128 MiB free space without acknowledging the report. System logs are collected from agent startup; crash recovery depends on retention of the OS logs.

## Upgrades and removal

1. Read both applications' changelogs and create a backup.
2. Stop the dashboard and agents. Linux: `sudo systemctl stop pulse-monitor` / `pulse-agent`. Windows: `Stop-ScheduledTask -TaskName PulseMonitor` / `Stop-Service PulseAgent`.
3. Replace the dashboard executable and `out` from the new archive; replace agent executables. Preserve data, tokens, and agent state.
4. Start the services and sign in again. Check reports and modules. For rollback, restore the matching executables, `out`, and data backup; do not assume newer files are backwards compatible.

On Linux, stop and disable the unit (`sudo systemctl disable --now pulse-monitor` or `pulse-agent`), remove its file from `/etc/systemd/system`, run `sudo systemctl daemon-reload`, then remove the program directory. On Windows, stop and remove the task (`Unregister-ScheduledTask -TaskName PulseMonitor -Confirm:$false`); for the agent, use `Stop-Service PulseAgent` and `sc.exe delete PulseAgent`. Delete data, state, and tokens separately only when no longer needed. Removing a node from the dashboard preserves its log archive.

## Troubleshooting

| Symptom                          | Check                                                                                          |
| -------------------------------- | ---------------------------------------------------------------------------------------------- |
| Dashboard refuses remote startup | Owner exists in the active `-data`; certificate, key, and `-public-url` are supplied together  |
| TLS error                        | Domain, certificate expiry, full chain, OS trust; URL has no path                              |
| Node waits for data / offline    | `-server`, token, URL, firewall, outbound access; `pulse-agent -check` checks local collection |
| Agent receives HTTP 401          | Token replaced or revoked; update the file and restart                                         |
| Missing GPU / logs / processes   | Agent permissions, driver, `nvidia-smi`; some OS metrics are unavailable                       |
| Missing packets                  | TShark, Npcap on Windows, capture permissions, `tshark -D`, and `-capture`                     |
| Module error / unsupported       | Architecture build, resource permissions, configuration, timeout, and JSON protocol            |

Linux: `sudo journalctl -u pulse-monitor -f` / `pulse-agent`. Manual startup prints errors in the terminal. Do not publish tokens, TLS keys, private configuration, or unredacted logs in Issues.

## Native modules and operation results

Windows Job Objects and Linux process groups clean up ordinary descendants after module completion/cancellation. These are lifetime controls, not a security sandbox: trusted native code still has the host process identity and can access its files/network. A Docker socket grants host-control authority. Do not install untrusted native packages. Docker control now verifies the exact container state, configured healthchecks and a newer start timestamp for restart. The verification budget is 20 seconds; failure to establish readiness is `unknown`, not a reason to repeat a restart automatically. A running container without a Docker healthcheck does not prove application readiness.

Windows installation requires NTFS. The scheduled task runs as Local Service; the executable/static tree is read-only to that identity and its data directory is writable. Preserve private ACLs for data, certificates and backups. The installer does not relocate an existing exFAT installation.
