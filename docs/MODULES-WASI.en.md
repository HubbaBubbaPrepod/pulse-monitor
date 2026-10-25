[Русский](MODULES-WASI.md) · **English**

# Isolated WASI modules

Available with Monitor 0.7.0 and Agent 0.6.0. Native schema-1/schema-2 packages remain supported as full-trust components. Runtime is optional: omitted or `native` preserves native execution; `wasip1` never falls back to an OS process.

## Manifest and consent

A schema-2 component sets `runtime:"wasip1"` and contains exactly one `binaries["wasip1-wasm"]` artifact with its SHA-256 and base64 content. Build a Go guest using `GOOS=wasip1 GOARCH=wasm go build`. Do not embed native PE/ELF code under this target. Version metadata and executable bytes are immutable.

`requires` lists mandatory grants. `optionalCapabilities` lists grants an owner may add separately. A manual trigger may declare `requires` drawn from those two lists. The host checks these again when queuing and authorizing work; hiding or disabling a button is not authorization. Runtime, grant-contract, field, trigger or location changes disable existing configurations and clear grants until renewed consent. Compatible version updates preserve configuration. Offline JSON updates use the same installer as the opt-in Marketplace.

Hosts advertise `capabilities.moduleWASI:"1"`, preserving `moduleRuntime:"2"`. Older agents do not receive WASI leases. New leases add `runtime:"wasip1"`; native leases omit it. Artifact download remains bound to the authenticated operation and exact hash. WASI state uses a separate module/component/node namespace so legacy native state cannot leak into a new guest.

Marketplace queries default to native-compatible revisions for older clients. Updated clients request `runtime=all` for catalog, module history and exact-version metadata. `executionRuntimes` identifies `native`/`wasip1`; the older `runtimes` field still means Agent/Monitor location. Approval applies to one exact immutable version and hash, including previews.

## Boundary and limits

Each invocation gets stdin, bounded stdout, clocks and secure random values. No host directories are mounted. Environment variables, raw network sockets, native subprocesses, node credentials and the Docker socket are not exposed. Password-classified configuration/input fields and reserved host settings `serverId`, `telegramToken`, `telegramChatId`, `socket` are withheld from guest objects; historical secret classifications survive upgrades.

Limits: 16 MiB artifact, 2 MiB input, 128 MiB guest linear memory, 30 seconds including runtime-slot wait, 512 KiB stdout/host reply, 64 KiB host request and 64 host calls. Each host permits at most two simultaneous runtime invocations; dispatch is currently narrower. These are not a hard total host RSS or CPU-share limit. The embedded Go interpreter and broker remain part of the trusted host.

The guest receives scoped private state only with `state.read`; returned state is persisted only with `state.write`. Do not deliberately copy private state to public results. Host files remain private, not encrypted at rest. Native modules still execute with their parent's OS privileges and require full trust; their grants do not form an OS sandbox.

## Broker ABI

Import `pulse_host.request(requestPtr:u32, length:u32, responsePtr:u32, capacity:u32) -> u32`. UTF-8 input is `{"method":"docker.logs","params":{}}`. Allocate a full 512 KiB response buffer and pass exactly that capacity. The host validates both memory regions before external effects. Zero returned bytes means a rejected ABI call; never resend a mutation to query its response size.

The reply is `{ok,status,data,error?}`, where `status` is `ok`, `error` or `unknown`. Duplicate/unknown request keys and invalid scopes are denied. Request parameters never replace the lease's identity, grants, target or private settings. The guest emits the existing protocol-2 `{protocol:2,status,data,state?,error?}` result. Host action/delivery proof overrides guest claims; a guest cannot hide an uncertain attempted side effect by printing `ok`.

## Agent methods

| Method                                          | Grant                                | Scope and budget                                                                              |
| ----------------------------------------------- | ------------------------------------ | --------------------------------------------------------------------------------------------- |
| `docker.snapshot`                               | `docker.read`                        | Fixed local Docker endpoint, at most one snapshot per invocation, at most 100 containers      |
| `docker.logs`                                   | `docker.logs`                        | Exact manual lease action and full 64-character `input.containerId`; last 200 lines, one read |
| `docker.start`, `docker.restart`, `docker.stop` | `docker.control` + `actions.execute` | Exact mutating lease action/target; at most one mutation                                      |

Snapshot params must be empty. Container-action params may be empty or repeat only the exact bound `containerId`; other fields are denied. No arbitrary Docker URLs, request bodies, sockets or HTTP proxying are available. The broker uses `/var/run/docker.sock` or `\\.\pipe\docker_engine`. It waits for the observed state, configured healthcheck and a new StartedAt after restart within a 20-second action budget. Docker acknowledgement alone is not readiness. The daemon and Agent are still privileged: issuing control grants allows those specific container changes.

## Monitor methods

| Method                  | Grant                    | Scope and budget                                                                                 |
| ----------------------- | ------------------------ | ------------------------------------------------------------------------------------------------ |
| `telemetry.snapshot`    | `telemetry.read`         | Empty params; owner-configured `serverId`; one bounded current sample/hardware/disk/GPU snapshot |
| `notification.telegram` | `notifications.telegram` | Only `{message:string}`; one attempt, up to 4096 characters / 16 KiB                             |

Monitor settings may declare reserved `serverId`/`telegramChatId` text fields and `telegramToken` password field with no nonempty defaults. The host keeps these values private. Telegram sends only to the configured chat through fixed HTTPS `api.telegram.org`, without redirects or environment proxies. It checks Telegram's message acknowledgement; this does not prove the recipient read a message. Network loss or an ambiguous confirmation yields `unknown`, without automatic retry. A new interval invocation is distinct; module authors must define their own alert state/deduplication.

## Cancellation and recovery

Disabling/reconfiguring/updating a component revokes queued operations and cancels running guest and broker requests. Persistent exclusive receipts retain at-most-once manual dispatch across Agent restarts. An operation interrupted after an external effect may remain `unknown`; inspect actual state before issuing a new operation. No runtime guarantees exactly-once external side effects.

Interval/manual triggers are supported. Event triggers, durable delivery retry queues, arbitrary native APIs, custom UI scripts and long-running jobs are not implemented. First-party Docker source: `cmd/docker-wasi`; its bridge is separate from legacy native socket clients. Use immutable new releases and explicit per-node grants.
