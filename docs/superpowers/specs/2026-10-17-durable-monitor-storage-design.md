# Durable Monitor storage and per-server log retention

Status: proposed design for review; no SQLite dependency, migration or cleanup has run.

## Purpose

Ship one supported product to all users, with the same public release artifacts
used on the operator's servers. Keep a standalone Monitor deployment small and
avoid requiring another service. Kafka is excluded from this stage: the intended
outcome is reliable ingestion and bounded storage, not a distributed event platform.

## Evidence and present boundaries

The current receiver scans the complete JSONL archive for each log line and holds
a global ingestion lock during that work. A local 30 MB / 20-line benchmark took
3.820 seconds before batch ingestion and 0.184 seconds after it. The deployed
archive was approximately 128 MB. Both Linux agents showed repeated response
header timeouts while their services remained running. A narrow hotfix was
installed on 2026-10-17; the future public release must include it too.

After the hotfix, both Linux nodes advanced their sequences and had no new loss
of contact events during the observed interval. The Windows pending report was
accepted at the TLS origin in 0.017 seconds but timed out through the public
Cloudflare path. This is a separate unresolved transport problem; changing the
database does not claim to fix it.

## Recommended architecture

Agent -> authenticated Monitor HTTP endpoint -> SQLite transaction -> API/UI.
SQLite lives on local storage on the Monitor host, never on a network share.
No data is routed through a centrally hosted Pulse broker. Module JSON remains
an installation format; native artifacts remain immutable files on disk.

The first storage migration covers agent receipt checkpoints, current reports,
metric history, collected logs, monitor events and per-node retention policy.
Account credentials, unrelated preferences and module private stores remain on
their current formats for this bounded stage, with no claim of a complete
application-wide storage migration. A later migration must preserve those
contracts separately rather than silently expanding this change.

Use a maintained Go SQLite driver supporting Windows amd64 and Linux amd64/arm64
without requiring a compiler on the user's machine. Pin and verify the driver's
SQLite engine includes the WAL-reset fix (3.51.3 or a documented fixed backport).
Driver version, transitive dependencies and any build toolchain change require
explicit approval before installation.

Use WAL and synchronous=FULL. Serialize bounded write transactions; use separate
bounded read connections, busy timeouts and context cancellation. Do not hold a
Go ingestion mutex while reading a network body. Bound query pages and write
batches, and manage checkpoints without unbounded long-lived read transactions.

## Receipt and recovery contract

A successful report response follows a committed transaction containing the
receipt identity and all corresponding log/metric/event changes. Bind identity
to server, boot and sequence, with a stable log index; retries are idempotent.
Failure before commit returns no acceptance. Failure after commit and before the
response is recovered by retry without duplicate log entries or module actions.

Keep last authenticated contact distinct from the last fresh measurement. A
retry of an old report can prove connectivity but cannot fabricate fresh metrics.
Preserve existing Agent protocol compatibility; any collection-time field or
heartbeat addition needs an explicit compatible wire-contract change and tests.

Agent buffering and delivery must be bounded and survive process restart. A
network stall must not silently discard collected logs or allow unlimited disk
or memory growth. Define and verify the buffer-full policy in the subsequent
Agent delivery plan; it must be visible to the operator. Native mutating module
actions remain at-most-once and are not replayed by telemetry recovery.

## Per-server log retention

Add an independently stored logRetentionDays setting per server, represented in
the existing node settings UI. New installations default to 30 days. Offer
1/7/14/30/90-day presets and a custom integer from 1 to 3650 days. The exact UI
text explains that the policy applies to copies stored in Monitor, not journals
on the monitored host. There is no silent automatic truncation of source logs.

For existing installations, keep retention disabled during migration until the
owner explicitly enables a policy. Suggest 30 days in the UI; merely upgrading
must not unexpectedly remove a year's existing history. An enabled policy is
persisted across restart and applies only to that server's collected logs.
Reducing it shows a deletion warning before saving. This consent covers future
scheduled cleanup under the selected policy, not unrelated data.

Use Monitor receipt time in UTC for the storage age, keeping source timestamps
for display. This avoids accidental mass deletion caused by a remote clock
jump. Explain this distinction in settings help and RU/EN documentation.

Delete expired records in bounded transactions in the background. Do not scan
the entire archive or block report ingestion. Metrics, operational events,
module operation audit and account records are not deleted by this setting.
Cleanup failure is visible and retried with bounded backoff. Deleting rows
reuses SQLite pages; do not promise an immediate smaller database file. Avoid
blocking VACUUM on the request path.

## Migration, disk pressure and backups

Take a consistent backup before import, keep original JSON/JSONL files, import
with a persistent progress marker and verify per-server counts and checkpoints.
A crash cannot produce a half-active database or overwrite the only original
copy. Corrupt records stop migration with a precise diagnostic; never skip them
silently. Disk-full and write failures must return failure instead of falsely
acknowledging data. Existing indexes and data remain intact on rollback.

Use an SQLite-aware snapshot/backup mechanism; copying only an open .db file
while ignoring its WAL is not a valid backup. Verify restoration in tests. Backup
storage must be independent of the live database for disk-loss protection.
No single-host database provides availability while that host or its disk is down.

## Verification and common release

Tests must cover duplicate and partial retries, process termination around
commit, concurrent nodes, slow uploads, long UI reads, disk-full/read-only errors,
restart after cleanup, server-specific cutoff boundaries, retention reduction,
remote clock skew and migration restart/corruption. Benchmark the same archive
and multi-node workload before/after, record ingestion latency and actual binary,
memory and disk costs. Do not infer throughput from generic database benchmarks.

Run all existing Monitor/Agent/Docker/marketplace checks. Publish compatible
versioned artifacts and RU/EN migration/rollback docs from the shared repositories.
Deploy those exact public artifacts on the operator's hosts; retain rollback
copies. No owner-only fork or permanently private patch is the final deliverable.

## Sources

- https://sqlite.org/whentouse.html
- https://sqlite.org/wal.html
- https://sqlite.org/pragma.html#pragma_synchronous
- https://sqlite.org/backup.html
- https://kafka.apache.org/intro/
