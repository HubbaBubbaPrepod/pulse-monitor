[Русский](PROTOCOL.md) · **English**

# Compatibility and protocol v1

Pulse Monitor and Pulse Agent are independent applications. Their first public versions, `v0.4.0`, are designed to work together. Application and protocol versions are separate concepts. Before v1.0, read the changelog before upgrading; future compatibility is not implied.

## Agent reports

`POST /api/agent/{id}/report`, `Content-Type: application/json`, `Authorization: Bearer <node-token>`. HTTPS is required for remote origins; HTTP is allowed only on loopback. Tokens are not sent in URLs. The agent does not follow redirects and verifies TLS certificates.

Body fields: `bootId`, increasing `sequence`, `os`, `hardware`, `sample`, `processes`, `disks`, `gpus`, `connections`, `packets`, `logs`, `capabilities`, and optional `logCursor`. The formal structures are in `backend/telemetry.go` and `backend/models.go`; the agent's matching structures are in its `wire.go`. Reports have no separate `protocol` field: v1 means the release's compatible contract, not HTTP version negotiation.

The body limit is 2 MiB. Repeating `bootId`/`sequence` is idempotent and does not duplicate accepted logs. The agent retains one unsent report in `pending.json`, retries with a delay up to 30 seconds, and resumes collection after acceptance. This is not an unlimited offline buffer. A node becomes offline after 15 seconds without a report. Restarting the dashboard requires a fresh heartbeat.

Unavailable process GPU metrics are `null`; GPUs/disks have a `supported` map; reports have `capabilities`. A missing metric does not mean zero load. Real node snapshots use `source: agent`; nodes without data use `waiting`.

## Owner API

Monitoring routes require a cookie session; mutations require `X-CSRF-Token` and JSON. The first owner registers on loopback. `GET /api/bootstrap` returns the registry; `PUT/DELETE /api/servers/{id}` modifies a node; `GET /api/servers/{id}/snapshot`, `/api/servers/{id}/stream`, and `/api/servers/{id}/logs` read data. Other routes include `/api/servers/{id}/token`, `/api/events`, `/api/preferences`, and `/api/modules`. The owner creates and revokes tokens; the raw token is shown on issuance, while the server stores SHA-256.

## Modules

The manifest and collector stdin explicitly contain `protocol: 1`. The agent fetches the list through `GET /api/agent/{id}/modules?os=linux&arch=amd64`, downloads executables through `/modules/{module}/binary`, and sends results through `POST /api/agent/{id}/modules/{module}/data`. All requests use the node's Bearer token. Results contain `status` (`ok`, `error`, `unsupported`), `error`, and `data`; the server assigns `at`. See [format and limits](MODULES.en.md).
