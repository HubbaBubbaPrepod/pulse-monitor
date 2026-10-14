[Русский](MODULES.md) · **English**

# Modules (experimental protocol v1)

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
