# Module v2 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let module authors validate a language-neutral multi-component v2 package
and learn its compatibility without executing code or changing existing installs.

**Architecture:** Add a separate v2 contract and validation path beside v1. Expose
authenticated preflight and capability discovery; leave installation/activation
of v2 disabled until the subsequent runtime/UI stages are implemented. Existing
Agent assignment and Docker v1 execution remain unchanged.

**Tech Stack:** Go 1.23-compatible standard library, JSON Schema 2020-12 documents,
existing Go HTTP/session/CSRF helpers, existing Prettier. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-17-module-platform-design.md`.

## Global Constraints

- Existing Docker packages must continue working unchanged.
- Keep the v1 execution path intact.
- Native processes are trusted code with their execution identity's OS access.
- Installation alone must never authorize automatic execution.
- Offline manual installation remains available for v1.
- No new dependency, environment variable, service or deployment in this stage.
- RU/EN author documentation; schemas/contracts use English identifiers.
- Work only in Monitor for this stage. Agent, Docker and marketplace executable
  code are unchanged; their later compatibility work is recorded in the roadmap.

## Review Focus

1. V2 payload sent to the v1 installer: explicit rejection, no assignment or file
   changes; v1 reinstallation conflict handling remains intact (tasks 2 and 3).
2. Binary declared for the wrong platform, incorrect digest or aggregate oversize:
   validation rejects the complete package before any write (task 2).
3. Unknown required API capability or a Monitor-only component on an Agent-only
   host: valid package may be incompatible, never silently activated (task 3).
4. Arbitrary paths, duplicate JSON members/component IDs, trailing JSON or unknown
   fields: reject ambiguous input; never interpret identifiers as paths (task 2).
5. Preflight receives a secret-looking configuration default or binary content:
   response must never echo payload values or artifact content (task 3).

## Scope and rollout boundary

This is stage 1, not a usable Telegram runtime. No event loop, privileged module
actions, native Monitor execution, UI injection or v2 marketplace publication is
enabled by completing it. A successful response distinguishes `valid` from
`compatible`; the host reports no executable v2 support yet. Later stage plans
must replace that capability declaration only when their runtime is available.

### Task 1: Freeze the v1 regression baseline

**Files:**

- Modify: `backend/modules_test.go`.
- Modify: `backend/modules_integration_test.go` only if its existing fixture flow
  needs an explicit assertion; do not require downloading a release for unit tests.

**Interfaces:**

- Consumes: `collectorPackage(t)`, `newTestApp`, existing HTTP test helpers.
- Produces: regression assertions used unchanged through later tasks.

- [ ] Add assertions to the existing install/assignment tests: v1 manifest retains
      schema/protocol 1, platform hash and fields; enabled agent assignment contains
      the original ID/version/hash/config; disabled assignments do not appear.
- [ ] Cover delete/reimport of identical bytes and rejection of changed bytes for
      the same version using existing test executables, not a fake shell script.
- [ ] Run `go test ./...` from `backend` with the existing Go installation. This
      characterization task should pass before production code changes.
- [ ] Commit the focused regression changes with `test(modules): pin v1 compatibility`.

### Task 2: Define and validate v2 multi-component packages

**Files:**

- Create: `backend/module_package_v2.go`.
- Create: `backend/module_package_v2_test.go`.
- Create: `docs/modules/module-package-v2.schema.json`.
- Create: `docs/modules/examples/v2-monitor-handler.json`.
- Create: `docs/modules/examples/v2-agent-collector.json`.
- Create: `docs/modules/examples/v2-combined.json`.

**Interfaces:**

- `decodeModulePackageV2(raw []byte) (modulePackageV2, error)` performs strict
  decoding, rejecting duplicate object members, unknown fields and trailing JSON.
- `validateModulePackageV2(pkg modulePackageV2) []moduleValidationIssue` returns
  deterministic `{path, code}` issues without copying user values into messages.
- `modulePackageV2` contains schemaVersion/protocol 2, existing identity metadata,
  descriptive permissions, and `components` (1..16).
- Each component has unique `id`, `location` (`agent` or `monitor`), `protocol:2`,
  `requires` (unique capability strings, at most 32), `fields` (existing field
  types, at most 32) and `triggers` (1..16).
- Each trigger is exactly one of `{type:"interval",seconds:N}` (integer 5..86400),
  `{type:"event",events:[names...]}` (Monitor only, 1..32 unique names), or
  `{type:"manual",action:"identifier"}`. Reject irrelevant trigger fields.
- Component `binaries` uses existing Linux amd64/arm64 and Windows amd64 platform
  keys and `{sha256,content}` values. Require 1..3 platform binaries per component,
  at most 16 MiB each and 48 MiB total decoded bytes across the whole package.
- Reuse current identity/version grammar and metadata limits; do not broaden v1.
  Artifact platform/header and digest checks reuse `validateCollectorPlatform`.
- V2 fields use current configuration types. Reject nonempty `password` defaults;
  runtime secret references are designed in stage 2, not embedded as example data.
- Event/capability names are dot-separated lowercase identifiers with optional
  underscore/hyphen segments, maximum 128 characters; no wildcard subscriptions.
- UI contributions and persistent workers are not valid fields in this first
  contract slice. Add them through a reviewed versioned extension before use.

- [ ] Write table-driven failing tests for Monitor-only, Agent-only and combined
      valid packages; all five invalid-input classes in Review Focus; event triggers
      on Agent; unsupported platform; empty components/triggers; invalid IDs and
      password defaults. Test binaries use the existing test executable helper.
- [ ] Run `go test -run 'TestModulePackageV2' ./...`; confirm failure identifies the
      missing v2 API, then implement strict decoding/validation.
- [ ] Write schema and metadata-only example fixtures. Examples omit executable
      bytes and are explicitly documentation templates, not claimed installable files.
- [ ] Run v2 tests and the full v1 suite; inspect the schema against each validation
      constraint. Do not add a JSON Schema validation dependency without approval.
- [ ] Commit with `feat(modules): validate v2 component packages`.

### Task 3: Add authenticated dry-run preflight and capabilities

**Files:**

- Create: `backend/module_preflight.go`.
- Create: `backend/module_preflight_test.go`.
- Modify: `backend/modules.go` only for route registration and explicit installer
  rejection of schemaVersion 2 before staging files.

**Interfaces:**

- `GET /api/module-capabilities`: current owner session required; returns
  `{schemaVersions:[1,2],installableSchemaVersions:[1],runtimes:[]}`. Schema support
  does not imply execution support.
- `POST /api/module-packages/validate`: owner session and existing CSRF required;
  accepts a v2 package with the existing upload body/deadline limits. Returns
  `{valid:boolean,compatible:false,issues:[{path,code}],compatibilityIssues:[...]}`.
  Valid packages receive `runtime_unavailable`; malformed input returns HTTP 400
  with stable value-free issues; body over limit returns HTTP 413.
- `modulePreflightRoutes(mux *http.ServeMux)` registers both routes inside the
  existing protected API handler; do not create a public unprotected handler.
- No preflight response contains `content`, configuration defaults, binary bytes,
  tokens, inferred filesystem paths or submitted descriptions.
- Existing v1 installer rejects v2 with HTTP 409 and code `module_runtime_unavailable`.
  Unknown versions remain rejected. Existing success response and v1 paths remain
  byte/field compatible; do not add v2 objects to `GET /api/modules`.

- [ ] Add failing HTTP tests for missing session, invalid CSRF, valid dry-run,
      malformed/oversized bodies, unsupported capability, redacted output and explicit
      v2 install rejection. Snapshot the temporary data tree before/after validation
      and assert it is unchanged; prove the supplied executable is never invoked.
- [ ] Run `go test -run 'TestModulePreflight|TestModuleCapabilities' ./...`, confirm
      expected failures, then implement registration/handlers and installer dispatch.
- [ ] Re-run the tests plus all v1 regression tests and `go vet ./...`.
- [ ] Commit with `feat(modules): expose safe package preflight`.

### Task 4: Document author workflow and verify the stage boundary

**Files:**

- Modify: `docs/MODULES.md` and `docs/MODULES.en.md`.
- Modify: `CHANGELOG.md` and `CHANGELOG.ru.md`.
- Modify: `docs/superpowers/specs/2026-10-17-module-platform-design.md` status only
  after all applicable stage-1 verification passes.

**Interfaces:**

- Documentation describes the preflight request/response, strict field limits,
  supported platforms, secret restrictions and stable issue codes from tasks 2/3.
- State explicitly that v2 cannot be installed/run yet; document v1/v2 capability
  negotiation, later Agent/marketplace work and preserved manual Docker install.
- Keep the package ID/version convention unchanged. A v2-only author must not be
  told to publish it as a v1-compatible marketplace release.

- [ ] Update RU/EN docs and changelogs with the new opt-in author preflight flow;
      distinguish schema validation from compatibility and execution.
- [ ] Run `go test ./...` and `go vet ./...` from `backend`; run a Linux race suite
      only if an already available supported environment exists. Report platform limits.
- [ ] Run existing Prettier on changed Markdown/JSON and `git diff --check`.
      No frontend code changes means no unrelated Next.js rebuild is required.
- [ ] Review the full diff for v1 contract changes, accidental secret output and
      writes in the validation path. Report stage 1 complete only with actual results.
- [ ] Commit with `docs(modules): document v2 package preflight`.

## Subsequent plans

The next plan owns Monitor process lifecycle, secrets/state and host API
authorization, followed by durable events/alerts and a separately packaged
Telegram implementation. Agent jobs/actions, catalog/UI and isolated custom UI
each follow their own acceptance criteria in the spec. A backup example must not
claim production readiness before long-running jobs and restore audit are ready.

## Execution handoff

Recommended approach: native implementation in this session because the first
slice is confined to one repository and its tests. No subagents, external
publication or deployment are authorized by choosing that approach. Review this
plan before implementation; the broader architecture is not a claim that the
remaining stages are already implemented.
