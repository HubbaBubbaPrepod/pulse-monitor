# Isolated module execution

The owner approved a WebAssembly runtime and a capability broker on 2026-10-23 and requested continuation on 2026-10-25. Existing native modules remain explicitly full-trust compatibility components.

## Execution boundary

- Extend schema 2 with optional component `runtime`: omitted/`native` preserves existing behavior; `wasip1` selects a WebAssembly artifact under the `wasip1-wasm` key.
- Never fall back from a failed WebAssembly invocation to native execution. An older Agent must not receive a WebAssembly lease.
- Pin wazero v1.12.0, MIT-compatible Apache-2.0 dependency, in Monitor and Agent. The standard Go library cannot execute WebAssembly.
- Use the interpreter, 128 MiB linear-memory limit, 16 MiB artifact limit, 30-second invocation deadline and 512 KiB stdout/response limit. Linear-memory limits are not a hard limit on total Go-process RSS.
- No host filesystem mounts, inherited environment, direct network sockets, subprocesses or inherited credentials. WASI clocks/randomness are available. State stays in the existing scoped JSON state contract.
- Register only `pulse_host.request` for host operations. Validate guest pointers and request/output sizes before dispatch; allow at most 64 host calls per invocation. Do not repeat a mutating operation to determine its response size.

## Broker authority

- The trusted host owns invocation identity, grants, node, configuration and action target; guest arguments cannot override them.
- Node tokens, account data, password-type module fields and Docker sockets stay outside guest memory.
- Agent Docker operations: bounded snapshot reads, selected-container logs, and start/restart/stop for the exact action and full container ID in a mutating lease. Require the corresponding explicit Docker grant and `actions.execute`; at most one mutating call per invocation. Keep observed-state/readiness verification on the host.
- Monitor operations: selected-node telemetry and Telegram delivery with owner-configured bot/chat settings. The guest supplies message text, never the endpoint or token. Require explicit grants and bound message size/call count.
- Module state remains scoped by existing module/component/node identities. Revocation cancels the invocation and its host requests.
- Host errors returned to the guest are bounded and do not include credentials or stack traces. Uncertain external effects remain `unknown`, not successful or automatically replayed.

## Packaging and compatibility

- Package digest, immutable version, installation trust confirmation and per-node grants still apply.
- Update Monitor/Agent readers before publishing a WASI module. Add runtime support to Agent capability reports and lease validation.
- Marketplace validates the WASM header, digest, runtime/platform combination and metadata; never classifies native PE/ELF artifacts as isolated.
- Port the first-party Docker guest and keep native packages available as legacy versions. New broker grants require owner consent; do not silently inherit unrelated permissions.
- Show runtime/trust mode in installation and module settings, with Russian and English copy.

## Verification

Run real WASI guest tests on Windows and Linux for forbidden file reads/writes, environment secrets, network/process imports, deadline termination, oversized memory/output, malformed pointers and broker target/grant denial. Run existing native-module, receipt, revocation, package-validation and Docker health tests unchanged. Finish with independent code review, vulnerability scans, CI, new immutable releases and deployment with backups.
