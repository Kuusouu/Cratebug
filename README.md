# Cratebug

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals.

The project is a fresh implementation built with Go, Wails, React, and TypeScript. It aims to preserve useful workflows and compatibility with existing mod libraries while making filesystem operations safe, predictable, and recoverable.

## Project status

Cratebug is in Phase 0: repository and toolchain foundation. The application has a minimal development shell, a verified frontend-to-Go binding, and canonical validation. Production build, installation, and uninstall instructions will be added during the remaining Phase 0 tasks.

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

## Validation

Run every required frontend and Go check from the repository root:

```powershell
.\check.ps1
```

The frontend scripts are run from `frontend`:

| Command | Purpose |
| --- | --- |
| `mise exec -c "bun run dev"` | Start the Vite development server |
| `mise exec -c "bun run build"` | Type-check and build the frontend |
| `mise exec -c "bun run format"` | Apply Biome formatting |
| `mise exec -c "bun run format:check"` | Check Biome formatting without changes |
| `mise exec -c "bun run lint"` | Run Biome lint rules |
| `mise exec -c "bun run typecheck"` | Run TypeScript without emitting files |
| `mise exec -c "bun run check"` | Run all frontend checks and the build |

Canonical Go commands are run from the repository root:

```powershell
mise exec -c "go fmt ./..."
mise exec -c "go vet ./..."
mise exec -c "go test ./..."
```

## Continuous integration

GitHub Actions runs the canonical checks and a clean Windows application build on Blacksmith for pushes and pull requests to `master`. CI verifies the application without publishing packages or build artifacts.

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE).
