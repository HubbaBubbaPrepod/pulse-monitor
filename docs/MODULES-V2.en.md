[Русский](MODULES-V2.md) · **English**

# Native modules v2

Status: development branch, not a published compatibility release. v1 modules remain unchanged. V2 supports short native invocations on Agent or Monitor, private configuration/state, interval scheduling and authenticated manual requests. Event dispatch, sustained alerts, delivery retries, custom HTML/JS and long-running jobs are not implemented by this slice.

## Installation and trust

Upload a complete schemaVersion/protocol 2 JSON through Modules → Install module or `POST /api/extensions`. Packages contain component binaries for Linux amd64/arm64 and/or Windows amd64. Installation only stores verified artifacts; it does not enable native code. Configure each component and grant its required capabilities explicitly. Agent components have a separate configuration per server; Monitor components run once on the Monitor host.

Native code has the OS privileges of its parent process. Grants gate host dispatch and state access; they are **not an OS sandbox**. `network.outbound` does not constrain a native program's destinations. Hashes establish integrity, not author trust. Review source before installation. A Docker socket may grant host-level control even with read-only dashboard grants.

Component IDs must be unique ignoring case. Package IDs cannot alias another installed/retained directory by case. Versions are immutable: executable bytes and metadata cannot be replaced under the same version. Installing/reinstalling disables configurations and clears grants; re-authorize explicitly. To migrate a v1 ID, remove its active installation first; retained data does not automatically migrate.

## Configuration API

All owner endpoints use the existing cookie session; mutations require Origin/CSRF protection.

- `GET /api/extensions`: installed v2 metadata, without executable content.
- `DELETE /api/extensions/{module}`: deactivate, revoke configuration and cancel queued/running work; retain private data/artifacts.
- `GET/PUT /api/extensions/{module}/components/{component}/configuration?server={id}`: `{enabled,config,grants,clearSecrets?}`. Omit `server` for Monitor components.
- `GET /api/extensions/{module}/components/{component}/data?server={id}`: latest interval snapshot `{version,at,status,data,error?}`; other node scopes and old configuration revisions are not returned.

Capabilities currently supported: `state.read`, `state.write`, `network.outbound`, `actions.execute`. Each `requires` entry must be granted. Mutating manual triggers additionally require `actions.execute` even when it is not in `requires`. Unsupported capabilities and event triggers cannot be enabled.

Password values are omitted from GET configuration. Omitted password fields retain saved values; `clearSecrets:["field"]` deletes explicitly. A password's private classification survives a package upgrade, including password→text changes. Secrets and state are stored in private files (0600 where supported), not encrypted at rest. The designated Agent necessarily receives its own invocation configuration over the authenticated connection. Neither telemetry nor credentials are sent to the marketplace.

## Process protocol

One executable is started without a shell, receives one JSON document on stdin, writes exactly one result to stdout and exits:

```json
{
  "protocol": 2,
  "id": "<operation-id>",
  "serverId": "node-id",
  "trigger": "manual",
  "action": "logs",
  "config": {},
  "input": { "containerId": "<full-id>" },
  "state": {},
  "grants": []
}
```

Monitor components omit `serverId`. An interval uses `trigger:"interval"` and no action. The response is:

```json
{
  "protocol": 2,
  "status": "ok",
  "data": { "containerId": "<full-id>", "logs": ["message"] },
  "state": { "cursor": 1 }
}
```

`status` is `ok`, `error`, or `unknown`; an optional `error` is at most 256 bytes. `data` and optional `state` must be objects. stdout is at most 512 KiB; the process budget is at most 30 seconds. A queued operation expires after 60 seconds. State reads/writes require their grants. Encoded state is bounded to 1 MiB, and final Agent receipts to 768 KiB, including JSON escaping. Oversized sanitized output produces an explicit bounded error/unknown result.

State is private module-owned data; it may contain credentials/cursors and is deliberately not redacted or exposed as public result data. Known password strings in public Data/Error keys and values are redacted in one pass; encoded or transformed secrets cannot be reliably detected. A trusted module author must not deliberately expose credentials. stderr is discarded.

## Manual operations and recovery

`POST /api/extensions/{module}/components/{component}/operations?server={id}` accepts `{action,input}` and returns HTTP 202 with a persisted operation. Poll `GET /api/module-operations/{id}`. Public results bind operation/module/component/server/action, lifecycle timestamps and nonsecret audit target. Private input, event/state, configuration digest and secrets are omitted.

Statuses: `pending`, `running`, `succeeded`, `failed`, `unknown`, `cancelled`, `expired`. Configuration/version/grant changes revoke pending work and cancel executing processes. Monitor revalidates authorization before dispatch and completion. Agents must advertise `moduleRuntime:"2"`; older/offline agents are denied.

A persisted Agent receipt is created exclusively before execution; duplicate leases never execute again. Running operations become `unknown` after host restart. Lost result delivery may leave an unknown outcome even if an action happened: observe actual state before manually retrying. Manual requests are never automatically replayed. At most one active Agent lease is issued per node; regular telemetry runs independently.

Interval schedules are durable, do not replay missed time, and keep at most one outstanding invocation per component scope. The interval is a minimum scheduling delay, not a real-time guarantee when the host is busy.

## Declarative views and actions

Components may declare the existing `table`, `cards`, `chart`, `logs` views. Charts use actual snapshot times, unavailable gaps, independent scales and the current theme. History is scoped to the open server/module/configuration and retains at most 60 samples.

A table adds `rowKey:"id"` and `actions:["logs","restart"]`. Each referenced manual trigger declares `action`, optional `label`, `mutating`, `inputFields`, `inputKey` and `resultView`. `inputKey` receives the full row identity. `resultView:"logs"` opens selected-object logs; its result must echo the same input identity. Optional `when:{key:"state",values:["running"]}` controls observed-state availability in the UI; it is not server authorization. Standalone manual actions can present typed inputs without a table. Mutating actions require confirmation; uncertain outcomes are explicit.

Docker v2 uses scoped action names `logs`, `start`, `restart`, `stop` within component `engine`. It validates full 64-character IDs, accesses only the local socket/pipe, and uses a 5-second stop/restart grace period within its 20-second request budget. Logs include the last 200 lines from the clicked container. Switching containers does not rewrite configuration. The legacy `logContainer` setting remains only in the v1 compatibility package.
