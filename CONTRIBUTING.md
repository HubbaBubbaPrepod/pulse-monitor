**English** · [Русский](CONTRIBUTING.ru.md)

# Contributing

Open an issue with a reproducible scenario or a small PR. Include dashboard and agent versions, OS/architecture, and expected versus actual behavior. Remove tokens, private addresses, and sensitive logs. Discuss substantial contract changes before implementation.

Requirements: Node.js 22.13+ and Go 1.23+. Clone this repository and install dependencies:

```powershell
git clone https://github.com/HubbaBubbaPrepod/pulse-monitor.git
cd pulse-monitor
npm ci
```

See [development commands](README.en.md#development). The agent is developed in a [separate repository](https://github.com/HubbaBubbaPrepod/pulse-agent). Platform packaging also needs PowerShell 7.4+.

Run checks from the repository root:

```powershell
npm run lint
npm run typecheck
node --test lib/*.test.mjs
npm run build
cd backend
go test ./...
go vet ./...
```

Run `go test -race ./...` when CGO and a C compiler are available. Do not claim it ran when your environment cannot run it. For UI changes, check both languages, themes, narrow screens, and reduced motion. For protocol changes, update tests, the [protocol guide](docs/PROTOCOL.en.md), and changelog.

Preserve the architecture and keep changes small; add dependencies only when necessary. Commit subjects use `type(scope): summary`, up to 72 characters. PR descriptions should contain **Summary**, **Changes**, **Testing**, and **Risks/Notes**: explain the problem, changed files, checks actually run, and limitations. [Translations](docs/TRANSLATIONS.en.md) follow the same process; update both documentation languages when behavior changes.

Contributions are distributed under [MIT](LICENSE). Report vulnerabilities through [private GitHub Security reporting](https://github.com/HubbaBubbaPrepod/pulse-monitor/security/advisories/new), following [SECURITY.md](SECURITY.md). Never attach live credentials to a public issue.

## Formatting

Node.js 22.13+ is required for the development formatter. Install the pinned local version with `npm ci`. Node.js is not required to run the Go binaries.

```shell
npm run format
npm run format:check
```

Prettier formats supported source files, JSON, CSS, Markdown and YAML. Go files use `gofmt`. Generated files, local data and verification files are excluded through `.prettierignore`. CI checks formatting before accepting changes.
