**English** · [Русский](SECURITY.ru.md)

# Security policy

## Versions and scope

Report vulnerabilities in the current public release, Pulse Monitor, or the current main branch. Monitor and Agent release independently; include both versions and the Docker module version when relevant. This project does not promise a support period or response deadline.

## Private reporting

Use [Report a vulnerability](https://github.com/HubbaBubbaPrepod/pulse-monitor/security/advisories/new). Include affected versions, prerequisites, minimal reproduction, expected impact, and a proposed fix if available. Use synthetic data. Do not post vulnerability details in public Issues before maintainers have assessed the report; do not include real tokens, passwords, TLS keys, private server addresses, or user logs/screenshots.

Agent issues belong in [Pulse Agent Security](https://github.com/HubbaBubbaPrepod/pulse-agent/security/advisories/new); collector issues belong in [Pulse Module Docker Security](https://github.com/HubbaBubbaPrepod/pulse-module-docker/security/advisories/new). If the component is unclear, explain the full path in the Monitor report.

## Deployment boundaries

Register the first owner on loopback in the service's data directory. Remote access requires trusted HTTPS. Keep tokens and private keys outside the static `out` directory; restrict filesystem permissions and protect backups. Modules run arbitrary native code with agent privileges and no sandbox. Password configuration fields are not encrypted storage. Docker socket access can grant control over the host. See [installation](docs/INSTALL.en.md), [modules](docs/MODULES.en.md), and [operations](docs/OPERATIONS.en.md).
