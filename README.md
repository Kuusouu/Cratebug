# Cratebug

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals.

The project is a fresh implementation built with Go, Wails, React, and TypeScript. It aims to preserve useful workflows and compatibility with existing mod libraries while making filesystem operations safe, predictable, and recoverable.

## Project status

Cratebug is in Phase 0: repository and toolchain foundation. The application now has a minimal development shell and a verified frontend-to-Go binding. Canonical validation, production build, installation, and uninstall instructions will be added during the remaining Phase 0 tasks.

See:

- [Product specification](SPEC.md)
- [Roadmap](ROADMAP.md)
- [Active tasks](TASKS.md)
- [Contributor and agent guidance](AGENTS.md)
- [Toolchain baseline](docs/decisions/0001-toolchain-baseline.md)

## Toolchain setup

Install the pinned Go and Bun versions through `mise`:

```powershell
mise install
```

Install the pinned Wails CLI:

```powershell
mise exec -c "go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"
```

Verify the installed versions:

```powershell
mise exec -c "go version"
mise exec -c "bun --version"
mise exec -c "wails version"
```

See the [toolchain decision](docs/decisions/0001-toolchain-baseline.md) for exact versions and the upgrade policy.

## Development

Start the Wails development application from the repository root:

```powershell
mise exec -c "wails dev"
```

Wails installs frontend dependencies from `frontend/bun.lock`, starts Vite through Bun, generates typed application bindings, and launches the desktop window.

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE).
