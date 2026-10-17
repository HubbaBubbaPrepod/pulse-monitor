# Monitor storage

Monitor stores agent reports, chart samples, collected logs, events and per-node
log retention in `data/pulse.db` (SQLite). No Kafka or separate database service
is required. Account credentials, unrelated preferences and module private stores
retain their existing formats in this stage. Module installation JSON is unchanged.

## Upgrade and migration

Stop the old Monitor before upgrading. On first startup, the new Monitor preserves
legacy agent JSON/history, logs, events, server definitions and account files in
`data/legacy-sqlite-backup` and imports them. Original files remain unchanged. Allow
space for originals, backup and the database. HTTP starts only after migration
succeeds; a large import may take time.

Line archives import in transactions of 500 records with durable byte offsets.
An interruption resumes after the last committed batch. Corrupt records stop startup
with the file and offset; they are never silently skipped. Changes to original
sources during an unfinished import stop startup too: reconcile source and backup
before retrying. Do not delete backups to bypass diagnostics.

After activation, normal receipt no longer scans or appends legacy archives.
Rolling back to an old binary with legacy files restores only the pre-migration
state: it cannot read new SQLite records. Preserve a current SQLite snapshot and
plan data reconciliation before rollback; it does not restore all current history.

## Per-node log retention

Node settings provide 1/7/14/30/90 days, a custom whole number from 1 to 3650, or
no automatic deletion. New nodes default to 30 days. Migrated nodes default to
no cleanup; an upgrade must not silently delete existing history. Enabling or
shortening retention requires a confirmation in the form.

Age uses Monitor receipt time in UTC, not the remote machine's clock. Source
journal timestamps remain available for display. Imported records receive their
import time, so migration starts their retention clock again.

A background pass runs once a minute and deletes at most 1000 expired rows per
node. Large backlogs require multiple passes. This policy does not delete source
journals, metrics, events, module operation audit, or backup archives.

SQLite reuses freed pages; the database file need not shrink immediately. No
blocking automatic VACUUM runs in the request path. Monitor disk capacity and
keep backups on independent storage.

## Consistent backup and recovery

```sh
pulse-monitor -data /opt/pulse-monitor/data -backup /backup/pulse-2026-10-17.db
```

Use the corresponding executable and paths on Windows. This command creates a
consistent SQLite snapshot including committed WAL data, refuses to overwrite an
existing file, and exits. It neither starts module execution nor changes operation
statuses. Automated backups need unique names and a checked exit status.

For recovery, stop Monitor and preserve its complete previous data directory.
Place the snapshot as `pulse.db` in a new data directory along with the required
account, node definitions and module files. Never mix the snapshot with stale
`pulse.db-wal`/`pulse.db-shm` files. Start the same compatible version and verify
nodes, history and settings before switching users. A SQLite snapshot alone is
not a backup of account credentials or native module artifacts.

Do not copy only an open `.db` while ignoring its WAL. WAL with synchronous=FULL
and transactional acknowledgements protects committed writes under SQLite's
storage assumptions, not against a failed disk, lost host or faulty hardware.

## Development

Monitor sources require Go 1.26 or newer; release scripts use Go 1.27.2. The pinned
modernc.org/sqlite 1.60.1 driver includes SQLite 3.53.4 with the WAL-reset fix.
Users run the compiled executable without installing Go or SQLite on monitored hosts.
