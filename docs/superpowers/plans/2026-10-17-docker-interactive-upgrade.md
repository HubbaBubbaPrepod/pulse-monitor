# Docker Interactive Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Display container IDs and standard Pulse charts, open any container's
logs from its row, and support explicitly authorized start/restart/stop actions.

**Architecture:** Use the generic v2 Agent action/read-request mechanism. Docker
implements the Engine-specific operations; Monitor authenticates and authorizes
requests and Agent executes the assigned component locally. Retain read-only
Docker v1 behavior for existing users.

**Tech Stack:** Existing Go standard library, Next.js/React/TypeScript UI, Docker
Engine HTTP API over existing Unix socket/Windows pipe transports. Windows named pipe transport uses the explicitly approved Microsoft/go-winio v0.6.2 dependency.

**Spec:** `../specs/2026-10-17-module-platform-design.md`, especially Docker
interactive acceptance requirements.

## Global Constraints

- Existing Docker v1 packages and assignments remain valid.
- Container actions require explicit per-server management authorization.
- Use a full container ID, not a container name or row index, for every request.
- No Docker TCP listener, remote shell commands or direct browser socket access.
- Manual actions are not executed from a periodic collection invocation.
- Do not automatically replay an uncertain mutating operation.
- Use existing theme, chart conventions and RU/EN translations.
- No new dependencies, services or production deployment without its applicable
  authorization. Real production containers are not verification fixtures.

## Review Focus

1. A late response from a prior server/container must not replace the selected
   container's logs (task 4).
2. A stale name, truncated ID, traversal string or mismatched operation result
   must never target or relabel another container (tasks 2/3).
3. A user bypassing disabled controls must still be denied on a read-only or
   disabled assignment, at Monitor and module execution boundaries (tasks 2/3).
4. Disconnect/restart after Docker accepted a command must not replay restart or
   claim definite failure when the outcome is unknown (task 2).
5. CPU above 100%, unknown memory, server changes and missing samples must remain
   truthful on charts and container status (tasks 1/4).

## Dependency and stage boundary

Task 1 can ship on the existing collector protocol. Tasks 2-5 depend on the v2
foundation and generic Agent manual invocation API. The foundation's preflight
alone does not satisfy that dependency. Implement and verify that runtime slice
before exposing controls. This plan does not silently authorize a separate
Docker-only execution mechanism or unreviewed execution of native Monitor modules.

### Task 1: Container identity and consistent resource charts

**Files:**

- Modify `D:/Documents/ChatGPT/PulseModuleDocker/manifest.json`.
- Modify `components/modules.tsx`, `lib/modules.ts`, `lib/modules.test.mjs`.
- Create `components/module-chart.tsx` using the established chart presentation.
- Modify `app/globals.css` only for needed module chart/ID styles; preserve
  existing chart classes.

**Interfaces:**

- Docker collector already returns `containers[].id` as the full Engine ID.
- Table displays an ID prefix and exposes full ID for copying/inspection.
- Chart accepts `{at:string,value:number|null}[]`, label and unit; timestamps are
  actual sample times, CPU and memory have independent scales. No fabricated data.

- [ ] Test identity formatting and chart data derivation against full IDs, missing
      values, values above 100 and real timestamps. Watch the new behavior tests fail.
- [ ] Add the manifest column and replace the bare module polylines with the
      standard grid/area/tooltip/keyboard presentation using current theme variables.
- [ ] Ensure changing server/module clears or scopes history immediately. Confirm
      charts do not mix samples or silently turn missing values into zero.
- [ ] Run the Monitor tests, typecheck/lint and Docker tests. Inspect desktop and
      mobile UI on a local fixture; no production data mutation is needed.

### Task 2: Generic Agent read/action transport

**Files:**

- Monitor: create `backend/module_operations.go` and its test file; integrate with
  `backend/modules.go`, `backend/server.go` and existing persistence/auth helpers.
- Agent: create `D:/Documents/ChatGPT/PulseAgent/module_operations.go` and matching
  tests; integrate with `modules.go`, `main.go` and existing process cancellation.
- Document the reviewed wire contract in `docs/MODULES.md` and `.en.md` before
  changing public API behavior; keep v1 collection payloads unchanged.

**Interfaces:**

- Operation binds ID, server, module/component, exact version/digest, operation
  name, typed input, creation/expiry time, grant and lifecycle status.
- Separate read-only requests from mutating actions. Persist before dispatch and
  record result/audit without exposing secret configuration.
- Capability negotiation prevents dispatch to agents lacking action support.
- Agent reports against the exact operation ID and never receives another node's
  assignments, payloads or results.
- Mutating timeout/disconnect recovery reports unknown outcome; no automatic
  replay. A subsequent observation can show state without asserting causation.

- [ ] First write failing HTTP/process-boundary tests for auth/CSRF, disabled
      assignments, missing grants, unsupported Agent, version mismatch, expiry,
      conflicting operations and mismatched result identity.
- [ ] Add persistence/recovery tests around dispatch before/after process exit and
      result reporting; prove no second automatic execution after uncertain delivery.
- [ ] Implement bounded dispatch/cancellation/output limits and independent Agent
      execution so ordinary telemetry is not blocked by a stop/restart request.
- [ ] Run both Go suites, vet and supported race checks. Do not expose UI controls
      until the transport supports these behaviors and its contract is reviewed.

### Task 3: Docker log and control handlers

**Files:**

- Create `D:/Documents/ChatGPT/PulseModuleDocker/operations.go` and
  `operations_test.go`.
- Modify Docker `main.go`, `manifest.json` and `scripts/build.ps1`.

**Interfaces:**

- Read operation: `container.logs`, input `{containerId}`. Return containerId,
  collection timestamp and bounded decoded lines; no persistent assignment edit.
- Actions: `container.start`, `container.restart`, `container.stop`, input
  `{containerId}`. Require the explicit management grant and strict full hexadecimal
  Engine ID. Use POST on the local Engine API; never exec a Docker CLI shell.
- Keep a context deadline covering socket/pipe connection, API request and result.
  Stop/restart grace period must fit the host's declared action budget. Describe
  the grace period in the confirmation UI.
- Accept Engine's documented success/idempotent responses intentionally; return
  typed errors for conflicts, removed containers, denied access and timeouts.
- Keep legacy collection and optional `logContainer` behavior compatible.

- [ ] Write failing tests using a local fake Engine server. Verify the correct
      selected ID/method/path, HTTP outcomes, plain/multiplexed logs, empty logs,
      malformed IDs, denied management and cancellation.
- [ ] Implement handlers and dispatch separately from periodic `collect`.
- [ ] Run all Docker tests/vet; cross-build existing Linux amd64/arm64 and Windows
      amd64 targets. Change release version only when the package can be consumed by
      compatible Monitor/Agent versions; document those minimum versions.

### Task 4: Container interaction UI

**Files:**

- Monitor `components/modules.tsx`, `lib/modules.ts`, module-focused interaction
  component(s), `lib/locales/ru.json` and `lib/locales/en.json`.
- Existing UI styles and appropriate behavior tests; do not add a UI test library
  without approval if none exists.

**Interfaces:**

- Container name/details opens a log panel associated with selected server and
  full container ID. Show loading/empty/error states and explicit refresh.
- Start/restart/stop buttons derive availability from observed state, enabled
  assignment, granted capabilities and pending operations; backend remains authoritative.
- Stop/restart confirmation names server/container and expected interruption.
  Pending action blocks conflicting clicks; final result refreshes observations.
- Server/container switch invalidates prior reads; selected log target never
  changes shared module settings. Logs are treated as untrusted plain text.

- [ ] Test request identity, stale result suppression, state-dependent controls,
      confirmation and pending/error/unknown outcomes before implementation.
- [ ] Build accessible row controls and log panel. Preserve text selection/copy,
      horizontal table scrolling and keyboard navigation.
- [ ] Verify local end-to-end flow against an isolated fixture, including an old
      Agent and denied-control assignment. Do not start/stop real user's containers.
- [ ] Run full Monitor tests, lint/typecheck/build and inspect mobile/desktop in
      dark/light themes. Report UI verification limitations explicitly.

### Task 5: Packaging, catalog compatibility and release documentation

**Files:**

- Docker README/README.en.md, CHANGELOG/CHANGELOG.ru.md, manifest/build outputs.
- Monitor and Agent RU/EN module docs and changelogs.
- Marketplace `apps/api/src/packages/validate-package.mjs` and its tests,
  `packages/contracts/src/index.ts`, module detail permission/capability display
  only where required by the reviewed v2 schema.

**Interfaces:**

- Immutable release package declares runtime requirements and control capabilities.
- Marketplace understands v2 before advertising its package as installable.
  Manual installation remains supported by compatible hosts.
- Existing published Docker package is not overwritten or silently upgraded.

- [ ] Test package validation and installation for current v1 and new interactive
      package; reject wrong platforms/digests/runtime requirements.
- [ ] Update RU/EN docs with grants, log selection, grace periods, unknown outcomes
      and minimum compatible versions. Native socket privileges remain disclosed.
- [ ] Build and verify release artifacts/checksums locally; record actual tests.
- [ ] Publish/deploy only after compatible components are ready and the applicable
      release/deployment authorization is established. Keep rollback packages.

## Execution handoff

Review with the v2 foundation plan. Recommended execution is native in this
session with tests at each boundary. Stage 1 identity/chart improvements may be
implemented first; interactive controls require the generic runtime and must not
be presented as working until end-to-end verification passes.

## Local verification record (2026-10-17)

Implementation is available in the development branches, not yet a public release.
Monitor Go tests/vet and frontend lint/production build passed. Agent tests/vet
passed. Docker tests/vet, Windows real named pipe HTTP/cancellation tests and
three-platform builds passed. Fresh v1 and v2 JSON installation integration tests
passed. Marketplace API/web tests and lint passed. A disposable local Docker
container verified selected-target logs, denied control, stop/start/restart; it
was removed. No production container action was performed.

An isolated native UI fixture verified separate selected-target logs, plain-text
rendering, observed-state controls, confirmation, standard charts, Full HD and
390px mobile layouts and both themes. Linux race tests remain a CI check; no
local Linux race result is claimed. Release integration and public artifacts
remain pending; events/alerts/Telegram/custom HTML are subsequent stages.
