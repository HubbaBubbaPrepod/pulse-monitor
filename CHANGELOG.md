**English** · [Русский](CHANGELOG.ru.md)

# Changelog

## [0.6.4] — 2026-10-19

- Keep Monitor running when an offline-event write times out. Retry availability checks without discarding unsaved events.
- Use a small native module fixture so race-enabled CI tests stay within upload limits.

## [0.6.3] — 2026-10-18

- Fill the overview process card with a responsive number of rows based on the adjacent log card height.

## [0.6.2] — 2026-10-18

- Match the height of the overview process and log cards when displayed side by side.

## [0.6.1] — 2026-10-18

- Align preview log columns with the source visibility breakpoint, keep timestamps readable and allow horizontal scrolling of grouped archives on phones. Backend behavior is unchanged.

## [0.6.0] — 2026-10-18

- Update modules from the opt-in Marketplace or offline JSON import. Bind catalog updates to the selected publisher, preserve compatible v2 configuration and require renewed grants after contract changes. Retire legacy v1 manifests when importing their v2 replacement; retain old binaries and data.
- Discover prerelease updates through an explicit preview toggle. Show install progress and published/installed versions.
- Make node addresses optional metadata: agents use outbound HTTPS to Monitor and require no inbound node port or node certificate. Editing this metadata no longer revokes the agent token.
- Fit the process summary to its card and align grouped/archive log rows.

## [0.5.0] — 2026-10-18

- Preserve migration checksums across Git LF/CRLF checkouts and send only writable module configuration fields from the settings UI.

- Store telemetry receipts, logs, metric history and events in SQLite WAL with resumable legacy import and consistent snapshot backups. Add per-server receipt-time log retention; migrated nodes keep cleanup disabled.

- Add native v2 Agent/Monitor interval and manual components with explicit scoped grants,
  private configuration/state, persistent operations and non-replaying uncertain outcomes.
- Add generic component settings and table/standalone actions, selected-object logs,
  container IDs and standard resource charts. Keep v1 assignments and data APIs intact.
- Validate immutable metadata size/case portability and preserve password classification
  across upgrades; recheck catalog opt-in/identity under the installation lock.

- Add owner-authenticated v2 package preflight, a multi-component manifest schema
  and author templates. Validation does not install or execute packages; v1
  collectors remain compatible and unsupported v2 installation is explicit.

- Add an opt-in module marketplace catalog with persistent origin settings, search, platforms, published version selection and explicit trust confirmation. Fetch and verify exact release snapshots through the existing installer; preserve offline JSON import and reject installed-ID replacement.

## [0.4.0] — first public release

### Added

- Independent self-hosted dashboard: one Go executable serves the static UI and API, with Linux amd64/arm64 and Windows amd64 archives.
- Real nodes through the separate Pulse Agent: metrics, processes, disks, supported GPU metrics, connections, logs, and events. Empty installations wait for agents.
- First-owner registration, bcrypt hashing, cookie sessions, CSRF, login attempt limits, and HTTPS for remote access.
- Persistent data, metric history, log archives with filtering, grouping, and CSV export; interval selection, live updates, and paused viewing.
- Russian/English UI, appearance editor, custom palettes, JSON import/export, and persistent owner preferences.
- Experimental v1 modules: JSON packages with native collectors, settings, and table/cards/chart/logs views; Docker module in the separate pulse-module-docker repository.
- Linux/systemd and Windows installers, build checks, documentation, and MIT license.

### Fixed

- The selected server persists across reloads; duplicate module links were removed from the sidebar.
- Module settings cannot be saved before the selected node's configuration loads; optional manifest collections return arrays.
- Demo archive migration streams logs, preserves the original copy, and retains real previously connected nodes.

### Notes

- Compatible first public release pair: Pulse Monitor v0.4.0 / Pulse Agent v0.4.0.
- Future compatibility is not guaranteed before v1.0. Modules run without a sandbox using agent permissions.
- GPU/packet availability depends on OS, drivers, and privileges; JSONL archives are not automatically pruned.
- Earlier 0.1–0.3 versions identified internal development stages, not public releases of these two repositories.
