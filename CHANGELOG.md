**English** · [Русский](CHANGELOG.ru.md)

# Changelog

## [Unreleased]

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
