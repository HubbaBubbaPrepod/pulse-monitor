[Русский](MODULES.md) · **English**

# Modules

## V2 runtime (unreleased)

This branch supports native Agent and Monitor components with bounded interval and manual invocations. Existing v1 collectors remain supported. Published v0.4.0 binaries do not gain v2 support automatically: update both Monitor and Agent from compatible builds before using the interactive package.

See [the v2 author/runtime contract](MODULES-V2.en.md) for installation, grants, secrets, state, operations and table actions. Event declarations are validated but cannot be enabled yet; alerts, Telegram delivery, custom JavaScript and long-running jobs are separate subsequent stages.

`GET /api/module-capabilities` reports schemas/installable schemas `[1,2]`, runtimes `agent.process.v2` and `monitor.process.v2`, and triggers `manual`/`interval`. Owner-authenticated `POST /api/module-packages/validate` is read-only; `compatible` depends on supported capabilities, triggers and the Monitor platform. It returns value-free issue paths/codes and never executes uploaded code. The v1 `/api/modules` endpoint still rejects v2; use `/api/extensions` or the dashboard importer.

Limits: 1–16 components, 32 fields and capabilities per component, 1–16 triggers, interval 5–86400 seconds, 16 MiB per executable, 48 MiB decoded artifacts in total, 68 MiB request body, 2 MiB encoded persisted metadata. IDs are portable across case-insensitive filesystems. Duplicate members, unsupported types, unknown fields, wrong hashes/platforms and nonempty password defaults are rejected.

The [schema](modules/module-package-v2.schema.json) describes the shape. Files in [examples](modules/examples/) are documentation templates with empty binaries; add real compiled artifacts and hashes before installation.

## Legacy v1 collectors

A module adds a collector and dashboard views. Upload its JSON package in dashboard settings, then assign it to a node, complete the fields, and enable it. The agent receives the assignment and matching executable automatically. A missing platform build results in `unsupported`.

## Trust and permissions

A collector is an arbitrary native program running with the agent's permissions. There is no sandbox. `permissions` describes expected access for the operator; it does not enforce OS restrictions. SHA-256 checks integrity, not trust in the author. Review the source and package origin before uploading. An agent running as root/SYSTEM grants the same privileges to its modules.

Values of `password` fields are stored in the server's private data directory and sent to the assigned agent over HTTPS; the owner can access them through the API. A hidden input is not encrypted storage.

## Package format

One JSON object contains `schemaVersion: 1`, `protocol: 1`, `id`, `version`, `name`, `author`, `description`, `permissions`, `fields`, `views`, and `binaries`. Example structure (replace the executable and digest with a real build):

```json
{
  "schemaVersion": 1,
  "protocol": 1,
  "id": "example",
  "version": "1.0.0",
  "name": "Example",
  "author": "Your name",
  "description": "Example collector",
  "permissions": ["Read local service status"],
  "fields": [{ "key": "endpoint", "label": "Endpoint", "type": "text", "required": true }],
  "views": [
    {
      "kind": "cards",
      "title": "Service",
      "key": "service",
      "columns": [{ "key": "status", "label": "Status" }]
    }
  ],
  "binaries": {
    "linux-amd64": { "sha256": "<64 hexadecimal characters>", "content": "<base64 executable>" }
  }
}
```

`id` and keys accept letters/digits, `_`, and `-`, start with a letter/digit, and have a maximum length of 64. Versions use `X.Y.Z` or `X.Y.Z-suffix`, up to 48 characters. Executable keys: `linux-amd64`, `linux-arm64`, `windows-amd64`. `content` is the executable's base64 bytes; `sha256` is their SHA-256. Limits: 16 MiB per executable, 48 MiB in total. Architecture and executable format must match the platform key.

Up to 32 fields: `text`, `password`, `number`, `boolean`, with optional `required` and `default`. Up to 16 views; `key` selects a top-level result field:

| kind    | Result value                                                             |
| ------- | ------------------------------------------------------------------------ |
| `table` | Array of objects; columns select row keys                                |
| `cards` | Object; columns select card values                                       |
| `chart` | Object of numeric values; the current view retains the latest 60 samples |
| `logs`  | Array of messages                                                        |

A column contains `key`, `label`, and optional `unit`. Packages contain no UI JavaScript; the dashboard renders the views. Change `version` when changing executables; conflicting content for an existing version is rejected.

## Process contract

The agent starts the collector as a separate process and sends one JSON object to stdin:

```json
{ "protocol": 1, "config": { "endpoint": "local-service" } }
```

The collector prints exactly one JSON object to stdout and exits with code 0, for example `{"service":{"status":"running"}}`. Send diagnostics to stderr. stdout is limited to 512 KiB; download/installation and execution of one module share a 5-second timeout. The agent waits 5 seconds between cycles; this does not guarantee an exact interval with multiple modules. Process failure, timeout, or invalid JSON results in `error`. stderr is not sent to the dashboard.

## Docker module

The separate [Pulse Module Docker](https://github.com/HubbaBubbaPrepod/pulse-module-docker) collects the Docker Engine version, containers, CPU/memory, network counters, and optionally recent logs from a selected container. It releases independently; package v1.0.2 uses module protocol v1.

Download `docker-module.json` from the [module releases](https://github.com/HubbaBubbaPrepod/pulse-module-docker/releases), or build from the root of its clone with Go 1.23+ and PowerShell 7.4+:

```powershell
./scripts/build.ps1 -Version 1.0.2
```

The build produces `artifacts/releases/docker-module.json`. Upload it to dashboard module settings and enable it for a node with Docker. Linux uses a Unix socket; Windows uses a named pipe. The collector makes read requests, but Docker socket access usually grants privileged control over the host; it is not a security boundary. Do not expose the Docker API to the network for this module.

## Release package acceptance

The Go backend can verify a downloaded package without modifying a running monitor. From `backend`:

```sh
go test -tags=integration -run='^TestInstallReleasedDockerModule$' -count=1 -v -args -module-package=/absolute/path/docker-module.json -module-sha256=<EXPECTED_SHA256> -module-version=1.0.3
```

Supply an independently verified SHA-256 and expected release version. The test installs the real JSON through the HTTP handler into `t.TempDir()`, checks all three platform files and rejects reinstalling the same immutable version. It never executes binaries or contacts production agents. Without a package argument, this additional test is skipped.

## Marketplace catalog

Open **Modules → Marketplace**, enter the catalog origin and select Connect catalog. Connections are disabled by default. Settings persist in `data/marketplace.json`. HTTPS origins without paths, queries or credentials are supported; HTTP is accepted only for a literal loopback IP such as `http://127.0.0.1:3100`. Private DNS destinations and redirects are rejected. On a remote Monitor, 127.0.0.1 means the Monitor server itself, not the browser computer.

Monitor requests the catalog when opening the tab, searching, paging or refreshing. Only search filters and selected module requests are sent; telemetry, logs, node configuration and sign-in credentials are never sent. Agents never contact the marketplace. Disconnecting blocks new requests and installation of already downloaded results; installed modules continue running.

Select a module and published version, inspect its publisher, permissions, platforms, source and SHA-256, then confirm trust. Monitor rechecks publication, verifies the complete JSON size and digest and uses the shared installer with ELF/PE headers and binary digest validation. Assign the installed module to individual servers separately. Catalog approval does not guarantee native code safety.

Protocol v1 module IDs are global. Catalog installation never replaces an existing ID, including another publisher's module. Use existing manual JSON import for updates; installed modules and manual import remain available offline. Search, platform filters and next-page loading are supported. Downloading from a catalog requires no marketplace account.

Monitor owner API: `GET/PUT /api/marketplace`, `GET /api/marketplace/catalog?query=&platform=&cursor=`, `GET /api/marketplace/versions?owner=&id=`, `POST /api/marketplace/install` with `{owner,id,version,sha256}`. Existing cookie-session and CSRF protection apply to mutations. Catalog requests and installation return 409 while disconnected.

For end-to-end verification against a running local catalog, add `-module-marketplace http://127.0.0.1:3100` to the released-package test command. The test creates a temporary Monitor data directory, finds the approved release through the catalog API and installs through `/api/marketplace/install`; it does not modify a running Monitor or agent assignments.
