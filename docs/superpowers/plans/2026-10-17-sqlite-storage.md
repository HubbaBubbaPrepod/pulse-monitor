# SQLite Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate Monitor telemetry, log history, events and node retention into durable SQLite without losing legacy data or module compatibility.

**Architecture:** One local SQLite database, WAL/FULL, serialized writer and bounded reader pool. One report transaction persists receipt, logs, history, events and current node before acknowledgement. Existing owner/module stores remain unchanged.

**Tech Stack:** Approved modernc.org/sqlite v1.60.1 and transitive dependencies; Monitor Go minimum1.26, builds with existing1.27.2; existing Next/React UI.

**Spec:** ../specs/2026-10-17-durable-monitor-storage-design.md

## Global Constraints

- Continue in existing codex/module-platform; no old-code checkout or separate server-only implementation.
- Preserve v1/v2 module contracts, raw legacy files and private credentials.
- Existing installations keep retention disabled; new nodes default30 days.
- No Kafka, new infrastructure or production change before common release verification.
- Public report/log API remains compatible; cleanup is per-server and bounded.

## Review Focus

- Crash after commit before response: retry does not duplicate logs or events.
- Invalid/truncated legacy archive: startup fails, originals preserved, safe import retry.
- Slow body from one authenticated agent cannot hold ingestion lock for other nodes.
- Retention is based on receipt time, cannot cross node boundary or delete event/action audit.
- Windows DB handles close in tests/shutdown; WAL backup includes committed data.

### Task1: Database lifecycle and versioned schema

Files: backend/storage.go, backend/storage_schema/001_telemetry.sql, backend/storage_test.go, backend/store.go, backend/main.go, backend/fixtures_test.go, backend/go.mod/go.sum.
Interfaces: openSQLite(dir string)(*sqliteStore,error); Close() error; context-aware query and transaction helpers. Write pool1, read pool4; WAL/FULL, busy timeout5s; private database0600; migration checksum verified.

- [ ] Test engine version, pragmas, reopen and checksum mismatch RED.
- [ ] Add approved dependency and lifecycle; close on startup failure and shutdown after workers. Register explicit test handle cleanup.
- [ ] Run focused tests GREEN and commit.

### Task2: Preserved resumable legacy import

Files: backend/storage_import.go and storage_import_test.go.
Interfaces: migrateLegacy(ctx context.Context, dir string, servers []server) error; import progress and activation marker in database.

- [ ] Tests import nodes/history/logs/events, duplicate IDs, corruption/retry and unchanged source hashes RED.
- [ ] Back up eligible legacy sources before activation. Stream logs/history in bounded transactions with durable byte offsets. Import node/event identities idempotently. Verify counts before activation; never serve a partial import.
- [ ] Test fresh empty data and repeated startup; GREEN, commit.

### Task3: SQLite log queries and telemetry reads

Files: backend/storage_logs.go, storage_nodes.go, store.go, telemetry.go, tests.
Interfaces: appendLogs(ctx,rows,receivedAt); queryLogs(ctx,filters) (logPage,error); loadNodes(ctx) map; saveNode(ctx,id,node); bounded history loading.

- [ ] Preserve log filters/order/total, unicode search and timestamp boundary; test read-only/full errors RED.
- [ ] Replace JSONL scanning and node/history file reads; retain bootstrap/API shapes. Existing test fixtures seed files only before import.
- [ ] Run full backend suite GREEN, commit.

### Task4: Atomic report receipt and events

Files: backend/storage_reports.go, storage_events.go, telemetry.go, events.go and tests.
Interfaces: commitReport(ctx,id,old,next,events) error; deterministic event builder; event acknowledgement persistence.

- [ ] Tests transaction rollback/duplicate retry/reopen and slow-body concurrency RED.
- [ ] Decode/validate bounded body before lock, recheck credentials under lock, commit all accepted effects together, then publish memory state. Duplicate only advances contact, never fresh metric time. Preserve exact module cancellation semantics.
- [ ] Full suite+vet GREEN; benchmark old archive workload; commit.

### Task5: Retention policy and bounded cleanup

Files: backend/storage_retention.go, storage_retention_test.go, server.go, store.go, main.go.
Interfaces: node logRetentionDays pointer field (nil keep current/default;0 disabled;1..3650 active); cleanupLogs(ctx,now,batchSize)(int64,error). Worker uses batch1000 once per minute, reports errors.

- [ ] Tests scoped cutoff/clock skew/restart/disabled/default/invalid settings RED.
- [ ] Policy persists atomically independent of unrelated server file settings; delete only received-at expired log rows. Existing migrated nodes start0, newly added nodes30. Omitting field on old clients preserves policy.
- [ ] Full tests GREEN, commit.

### Task6: Node retention UI and RU/EN copy

Files: components/servers.tsx, lib/data.ts, locales ru/en, lib/retention.ts + tests.

- [ ] Test policy validation and reduction warning RED.
- [ ] Add presets/custom day input and disabled option in node settings; explicit warning acknowledgement on enabling/reducing policy, source logs unaffected. Preserve native number input keyboard/mobile handling.
- [ ] Frontend tests, lint/build GREEN; isolated UI check, commit.

### Task7: Backup, recovery and operational documentation

Files: backend/storage_backup.go + tests, docs/STORAGE.md/.en.md, README/changelogs.
Interfaces: Backup(ctx,destination) error using SQLite-aware consistent snapshot, refusing overwrite/live paths.

- [ ] Test backup with WAL commits, restore and invalid destination RED.
- [ ] Implement tested backup command and migration diagnostics/rollback instructions. Pruning rows reuses pages, no blocking automatic VACUUM. Record real binary/memory/disk measurements.
- [ ] Full backend/frontend/module checks GREEN; commit.

### Task8: Common release and deployment

- [ ] One fresh review, fix important findings with RED/GREEN tests.
- [ ] Run Windows/Linux builds and applicable race/integration/vulnerability checks; record limitations honestly.
- [ ] Integrate today's tested Monitor/Agent/Docker/marketplace changes into shared repositories; publish compatible public release artifacts.
- [ ] Back up installations; deploy the same published artifacts, validate migration/counts, node freshness and module actions only on disposable targets. Resolve separate Windows public-route upload failure; do not assume SQLite fixes it.

## Execution

User explicitly approved the written design/dependencies and then requested execution of the combined modules+SQLite release. Preserve native execution in this session; no repeated permission request for routine plan steps. Dependency approval already covers modernc/sqlite and Go minimum1.26. Servers stay unchanged until the verified common release.
