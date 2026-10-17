# Pulse module platform design

Status: architecture agreed; stage-1 contract/preflight implemented and tested.
Runtime, Docker interactivity and the remaining stages are in progress, not released.

## Purpose

Third-party authors must be able to extend Pulse without maintaining a fork of
Monitor or Agent. Extensions may collect data, react to events, perform explicitly
authorized actions, connect external services, and provide configuration and UI.
Telegram notifications are the first Monitor-side use case. Existing Docker
packages must continue working unchanged.

## Verified starting point

- `backend/modules.go` accepts only schema/protocol v1 packages with native
  collector binaries and declarative table/cards/chart/logs views.
- `PulseAgent/modules.go` starts each collector with `{protocol:1,config}`, accepts
  one JSON object, and applies a five-second install/execution budget per module.
- Assignments and configuration are currently per server. There is no Monitor-side
  module process or event subscription in this module protocol.
- Native collectors run with Agent's OS permissions, without a sandbox.
  Manifest permissions are descriptive, not OS-enforced restrictions.
- Marketplace validation and Monitor's marketplace installer also need explicit
  v2 support; publishing a new format alone does not make it installable.

## Recommended architecture

A versioned package may contain Agent components, Monitor components, UI
contributions, or a combination. A notification-only module must not require an
Agent executable. Installation and activation are separate: Monitor components
are enabled for the Monitor instance; Agent components are assigned to selected
servers. UI contributions appear only for enabled components and appropriate users.

Use separate native processes and a versioned JSON communication contract. This
keeps the host independent of Go plugin ABI and lets authors use any language that
can produce a supported executable. A supported platform build is required;
source files alone are not an installable package. Do not require a new language
runtime to be installed automatically on users' servers.

In-process native plugins would couple builds and failures to Pulse. A WASM-only
runtime would improve isolation but constrain host integrations and introduce a
new runtime requirement. Neither is the initial implementation approach.

## Extension points

| Contribution  | Location         | Examples                                          |
| ------------- | ---------------- | ------------------------------------------------- |
| Collector     | Agent            | Docker, databases, hardware and custom metrics    |
| Event handler | Monitor          | Telegram, webhooks, incident integrations         |
| Scheduled job | Monitor or Agent | Periodic checks and reports                       |
| Action        | Monitor or Agent | User-requested integration operations             |
| UI            | Monitor browser  | Settings, result views and dedicated module pages |

The agreed scope also includes backup and restore, deployment and rollback,
service/container management, diagnostics, inventory, security checks, reports,
file operations and integrations. These are module applications, not special
cases to embed in Pulse's core. Destructive operations require distinct grants
and an execution audit trail. Automatic execution requires an explicit rule;
installation alone must never authorize it.

Contribution types must not be a closed list of business domains such as Docker
or Telegram. New functionality is expressed through stable host services. Some
future needs will still require new versioned host APIs; an extension platform
cannot promise every conceivable capability without changes to its host.

## Contract and lifecycle

- Add a v2 manifest and protocol without changing the meaning of v1 fields.
  Declare component IDs, execution location, trigger, required host API versions,
  platform artifacts, settings scopes and requested capabilities.
- Keep the v1 execution path intact. Unknown versions or unsupported required
  capabilities fail with an explicit compatibility error before activation.
- Negotiate host capabilities when starting a component. Document request,
  response, error, cancellation and shutdown messages. Do not execute shell text
  from a manifest.
- Support bounded invocations for event, schedule and action triggers first.
  Persistent workers/streaming are a separate later contract, not simulated by
  leaving v1 collectors running indefinitely.
- Configure deadlines, output bounds, concurrency and queue limits on the host.
  Disable/remove stops new invocations, cancels active work and prevents queued
  work from restarting the component. An unhealthy component must not crash or
  block Monitor's ingestion or other modules.
- Provide host-managed state scoped by installation/component/server as needed,
  configuration, secret references, redacted diagnostic logs and health status.
  Shared global filesystem paths are not the module state API.

## Events, alerts and delivery

Monitor derives online/offline transitions from heartbeat freshness even when
no new Agent message arrives. Offline timing uses one documented threshold shared
with the UI. Persist the last state to avoid generating false transitions after a
Monitor restart. A newly started Monitor does not infer historical events it did
not observe.

Define versioned event envelopes with ID, type, source, server ID when applicable,
timestamp and payload schema. Start with server online/offline, metric samples,
and module health/data. A host alert service evaluates sustained thresholds and
produces fired/resolved events. Routing and evaluation must work with the browser
closed. A module can implement its own domain rules using subscribed data too.

Use a bounded durable delivery queue, explicit retries/backoff and event IDs.
Delivery is at least once, not exactly once. Modules must make handlers idempotent
where possible. A timeout after an external service accepted a message can still
cause a duplicate; do not promise duplicate-free Telegram delivery. Record final
failures for the operator and do not retry permanently invalid credentials forever.

## Permissions and trust

Separate user consent, host API authorization and OS isolation. Scope API access
to declared/granted resources; require explicit grants for actions that change
servers or external systems. Installing a package does not grant all capabilities.
An update requesting more access requires a new grant before activation.

Native processes are trusted code with their execution identity's OS access.
Host API checks and an outbound allowlist do not sandbox direct filesystem,
network or process access. Display this distinction for native Monitor components
as well as Agent components. Do not advertise enforced native isolation until
OS-specific containment has been implemented and tested. Keep secrets out of
catalog metadata, frontend payloads, ordinary logs and event broadcasts; provide
them only to the explicitly configured consumer.

## UI and developer experience

First extend built-in views with scoped settings, status, action buttons and
dedicated module routes. No redundant sidebar link to the generic Modules page.
Roles and API authorization must apply independently of navigation visibility.

Custom HTML/JavaScript applications are a later, explicit UI contribution, hosted
in an isolated origin/sandboxed frame with a narrow authenticated message bridge.
Never import a module's JavaScript into Monitor's privileged React application or
expose session cookies/tokens to an embedded app. This stage requires its own
reviewed origin, CSP, message validation and access-control design before shipping.

Publish RU/EN author documentation, machine-readable schemas, conformance tests,
and runnable Agent and Monitor examples. Provide Go helpers as an optional
convenience; the protocol itself stays language-neutral. Packages and release
hashes remain immutable. Offline manual installation remains available.

## Telegram acceptance example

An independently packaged Monitor component with bot token and chat settings,
server selection, rule configuration, a test-message action and delivery status.
It receives alert/health events, sends notifications and recovery messages, and
keeps working without an open browser. It can notify about an offline Agent.
Bot credentials must not be distributed to the monitored agents. Test against a
local fake Telegram endpoint before an operator explicitly configures real sends.

## Cross-domain acceptance scenarios

Use three independently developed modules to test the extension boundaries:

- Telegram: Monitor-side event processing, secrets and external delivery.
- Backup: Agent-side scheduled work, durable state, progress, explicit restore
  authorization and audit. Long-running backup jobs require a later worker/job
  lifecycle; the initial bounded invocation runtime does not support them.
- Docker management: Agent-side collection plus separately authorized manual
  actions and UI. Existing read-only Docker v1 remains unchanged.

Modules may publish versioned namespaced events for other modules. For example,
a certificate-check module emits a renewal-needed event, an explicitly configured
automation invokes an authorized renewal action, and Telegram reports its result.
Cross-module wiring must not bypass permissions, create unbounded event loops or
expose another module's configuration/secrets. Do not require the three acceptance
modules to be implemented before the foundational contract is shipped.

## Docker interactive acceptance requirements

The existing Docker module is the first Agent-side interactive example. Its
upgrade spans Monitor, Agent and the independently released Docker package.

- Add a Container ID column to the container table. Show a short readable prefix,
  expose/copy the full ID, and always send the full immutable ID in requests.
  Never address a destructive operation by row index or mutable container name.
- Use the existing Pulse chart presentation: theme variables, grid, timestamps,
  hover/keyboard value inspection and unavailable-value gaps. CPU total uses
  percent-of-one-core units and may exceed 100%; memory uses MiB on its own scale.
  Do not show a fictitious historical range: current module history is limited
  to measurements retained during the current view.
- A container name/details button opens a container-specific log panel. Choosing
  another container sends a read request for that container, not a persistent
  edit of the module assignment or shared `logContainer` setting. Preserve the
  legacy setting for existing v1 installations; it is no longer required for
  interactive log viewing.
- Log requests are bounded, return container identity and collection time, and
  handle stopped/deleted containers, empty logs, unsupported logging drivers,
  access errors and timeouts explicitly. Selecting a different server/container
  cancels or ignores stale responses so logs cannot be mislabeled.
- Expose start, restart and stop as separately declared, authorized manual
  actions. Module installation alone does not grant container control. Operators
  explicitly enable management for selected servers. Stop and restart require
  a confirmation naming the exact container and server; all actions show pending,
  success/failure and the resulting state, not optimistic unconditional success.
- Requests travel browser -> authenticated Monitor API -> assigned Agent ->
  module -> local Docker socket/pipe. Do not open Docker TCP access or put Docker
  socket access into the browser/Monitor for remote hosts.
- Validate action names, input schema, enabled assignment, grant, server,
  component, installed version and full container ID at the relevant boundaries.
  Do not add a Docker-specific command-execution endpoint to Pulse's core.
- Persist an operation ID and audit entry before dispatch. Do not automatically
  replay an uncertain start/restart/stop after a timeout, disconnect or host
  restart. Report an unknown outcome and refresh observed container state;
  read-only log requests may be retried explicitly. Server-side authorization
  applies even when a client bypasses the visible buttons.
- Avoid conflicting concurrent control operations on the same container. Disable
  affected buttons while an operation is pending. Successful actions refresh the
  container list without clearing another server's state or collecting every
  container's logs on each telemetry cycle.
- Validate Docker behavior against a fake Engine API including multiplexed and
  plain TTY logs, HTTP 204/304/404/409/500 responses, cancellation, malformed IDs
  and read-only assignments. Do not use a production container as a test target.
- Test unchanged v1 Docker, an old Agent with a new Monitor, and unsupported
  interactive capabilities. Hide/disable unavailable controls with a clear update
  explanation rather than silently issuing unsupported commands.

## Delivery stages

1. **v2 foundation:** manifest/schema, compatibility preflight, platform validation
   and unchanged Docker v1 regression coverage. Validation is not activation:
   v2 installation stays unavailable until its runtime and UI support are ready.
2. **Monitor runtime:** lifecycle, settings/state/secrets, host APIs, durable event
   delivery and health. Test restart, cancellation, overload and invalid output.
3. **Alerts and Telegram:** offline detection, sustained threshold rules,
   fired/resolved events, module example and end-to-end fake-service tests.
4. **Agent extensions and actions:** v2 collectors, schedules, authorized actions,
   mixed-version behavior and Linux/Windows coverage. Long-running jobs need a
   separately reviewed progress/cancellation/restart protocol before backup or
   deployment modules may rely on that capability.
5. **Catalog and UI:** compatible marketplace validation/metadata, installation
   consent, settings/actions/views and author documentation. Verify manual and
   marketplace installation of both v1 and v2 packages.
6. **Custom UI applications:** separate reviewed isolation design and implementation.

Each stage needs its own bounded implementation plan. Do not claim the platform
is complete after only adding manifest fields or a Telegram-specific host branch.
Do not deploy protocol changes until compatible releases and rollback instructions
exist. No new dependency or service is authorized by this proposal alone.
