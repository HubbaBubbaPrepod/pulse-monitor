# Isolated Module Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute optional WASI modules behind a capability broker while preserving explicit native compatibility.

**Architecture:** Monitor and Agent retain independent release repositories and the existing invocation/receipt model. Each embeds the same small runtime adapter; host-specific brokers retain privileged I/O. Docker becomes a guest requesting a narrow host API.

**Tech Stack:** Go 1.27.3 toolchain, wazero v1.12.0 interpreter, existing TypeScript/Next.js/Nest.js stack.

**Spec:** `docs/superpowers/specs/2026-10-25-module-isolation.md`

## Global Constraints

- Original project folders and existing branches only, as requested by the owner.
- Preserve native schema-1/schema-2 behavior; no WASM-to-native fallback.
- Runtime dependency/API change approved by the owner; keep node/account secrets outside guest inputs.
- 128 MiB linear memory, 16 MiB artifact, 30 seconds, 512 KiB output, 64 host calls.
- No filesystem mounts, inherited environment, raw network, subprocess or raw Docker API.
- New immutable versions; each Marketplace publication still requires its own moderation decision.

## Review Focus

- A WASI lease reaching an old Agent must fail closed without native execution (Task 3).
- Malformed pointers/output buffers must not dispatch external effects (Task 1).
- A guest choosing another container/node or retrying a mutation must be denied (Task 2).
- Revocation during a host call must cancel I/O without replay (Task 3).
- Package labels must not describe native binaries as isolated (Task 4).

### Task 1: Runtime boundary

**Files:** Create `backend/internal/modulewasm/runtime.go`, `runtime_test.go`, `testdata/guest/main.go`; mirror under `PulseAgent/internal/modulewasm/`. Modify both `go.mod`/`go.sum` files.

**Interfaces:** `Run(ctx context.Context, artifact, input []byte, handler Handler) ([]byte,error)`; `Handler func(context.Context,json.RawMessage) (json.RawMessage,error)`. All host authority is captured by the handler, not decoded from the guest.

- [ ] Write real guest tests: `TestGuestCannotReadHostFiles`, `TestGuestCannotWriteHostFiles`, `TestGuestHasNoInheritedSecrets`, `TestGuestDeadline`, `TestGuestOutputLimit`, `TestGuestMemoryLimit`, `TestInvalidHostBuffersDoNotDispatch`.
- [ ] Run `go test ./internal/modulewasm` and observe failure before implementation.
- [ ] Add the approved pinned dependency and implement the adapter with the exact spec limits.
- [ ] Run the package tests and `go vet ./internal/modulewasm`; expect PASS.
- [ ] Commit runtime and tests with `feat(modules): add isolated WASI execution`.

### Task 2: Host brokers

**Files:** Create Agent `module_broker.go`, `docker_broker*.go` and tests; Monitor `backend/module_broker.go`, `module_broker_test.go`. Reuse the reviewed Docker parsing/state-verification behavior, not arbitrary request forwarding.

**Interfaces:** Each host constructs a `modulewasm.Handler` from its authorized lease and private configuration. Requests use `{method,params}`; responses use bounded `{ok,data,error,status}` objects. Methods: `docker.snapshot`, `docker.logs`, `docker.start`, `docker.restart`, `docker.stop`, `telemetry.snapshot`, `notification.telegram`.

- [ ] Add denial tests for missing grant, wrong action, wrong target, second mutation, oversized message, guest-supplied URL/token and cancellation during a call; observe RED.
- [ ] Implement host adapters with fixed methods/paths and per-invocation budgets; keep Docker postcondition checks on the trusted side.
- [ ] Run `go test -run 'Broker|Docker.*State' ./...` in each Go host; expect PASS.
- [ ] Commit `feat(modules): broker explicitly granted host operations`.

### Task 3: Package, lease and runtime dispatch

**Files:** Monitor `backend/module_package_v2.go`, `modules.go`, `module_extensions.go`, `module_operations.go`, `telemetry.go`; Agent `wire.go`, `module_operations.go`; related tests.

**Interfaces:** Optional `runtime` and `wasip1-wasm` artifact, lease runtime discriminator, reported WASI support. Omitted fields keep native behavior. Password fields are withheld from guest input and retained by its broker.

- [ ] Add tests for valid WASI package, mismatched header/platform, unsupported Agent, native fallback denial, secret-field isolation, target-bound lease and revocation; observe RED.
- [ ] Extend validation, file naming, dispatch and capability negotiation; reuse existing receipt/idempotency paths.
- [ ] Run both full Go suites and native fixture tests; expect PASS.
- [ ] Commit `feat(modules): negotiate isolated runtimes without native fallback`.

### Task 4: Marketplace and trust UI

**Files:** Marketplace `apps/api/src/packages/validate-v2.mjs`, contracts and validation tests; Monitor `lib/module-extensions.ts`, `components/modules.tsx`, `components/module-extensions.tsx`, dictionaries and related tests.

**Interfaces:** Runtime metadata remains public; executable content remains stripped. Native trust labels and WASI broker grants are visibly distinct.

- [ ] Add tests rejecting mislabeled native/WASM packages and checking runtime metadata survives stripping; observe RED.
- [ ] Update runtime validation and RU/EN trust/grant copy using the existing UI.
- [ ] Run API tests with real PostgreSQL, frontend tests, lint, typecheck and production builds; expect PASS.
- [ ] Commit `feat(modules): expose isolated runtime metadata and consent`.

### Task 5: Docker guest

**Files:** Docker-module WASI entry/bridge and build script, manifest, guest tests and documentation. Preserve native source entry for legacy artifacts.

**Interfaces:** Use `pulse_host.request`; guest transforms bounded broker data into existing views and requests only the lease-selected action. No Docker socket client or node credentials are linked into the guest path.

- [ ] Add guest tests for snapshot/log/action envelopes and denied broker response propagation; observe RED.
- [ ] Build `GOOS=wasip1 GOARCH=wasm` artifact and update a new package version with explicit broker requirements.
- [ ] Run guest end-to-end against an isolated disposable Docker container; verify readiness/unknown receipts and unchanged native tests.
- [ ] Commit `feat(docker): add an isolated broker-backed guest`.

### Task 6: Release and operations

**Files:** Bilingual module/development/security docs, changelogs, release versions and CI tests in the affected repositories.

- [ ] Run Windows/Linux security fixtures, race tests, `go mod verify`, `govulncheck`, npm audits and all required CI checks.
- [ ] Have an independent reviewer inspect the complete change and fix important findings with regression tests.
- [ ] Publish new immutable host/module releases, deploy official artifacts with backups and verify real telemetry/native compatibility.
- [ ] Obtain the separate human moderation decision for the new Docker version; do not bypass the Marketplace queue.
- [ ] Record remaining limitations, exact deployed versions and rollback paths in the closure report.
