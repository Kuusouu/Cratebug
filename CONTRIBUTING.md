# Contributing to Cratebug

Thanks for helping out. This covers getting a development environment running, the project's conventions, and where the rules live.

## Requirements

Development targets 64-bit Windows 10 version 1909 or newer and Windows 11. Install:

- Git and [Git LFS](https://git-lfs.com/)
- Microsoft WebView2 Runtime (Windows 11 already includes it)
- NSIS 3, only if you want to build the installer
- [`mise`](https://mise.jdx.dev/) - recommended, not required (see below)

```powershell
winget install jdx.mise
winget install NSIS.NSIS --silent
```

Restart the terminal after installing system tools so their updated paths are available.

### mise is optional

`mise` pins Go, Bun, and the Wails CLI to the versions the project expects, so `mise exec -c "<command>"` is the form used throughout the docs. If you prefer not to use it, install the same pinned versions yourself and run the commands directly - nothing in the build depends on `mise` being present. The exact pinned versions and the upgrade policy are in the [toolchain decision](docs/decisions/0001-toolchain-baseline.md).

## Getting started

```powershell
git lfs install
git clone https://github.com/Kuusouu/Cratebug.git
Set-Location Cratebug
git lfs pull

mise install
mise exec -c "go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"
$env:Path = "$(mise exec -c "go env GOPATH")\bin;$env:Path"

Push-Location frontend
mise exec -c "bun install --frozen-lockfile"
Pop-Location
```

Verify the toolchain:

```powershell
mise exec -c "go version"
mise exec -c "bun --version"
mise exec -c "wails version"
```

## Day-to-day development

Start the Wails development application from the repository root:

```powershell
mise exec -c "wails dev"
```

Wails starts Vite through Bun, generates typed application bindings, and launches the desktop window.

Run every required check before submitting anything:

```powershell
.\check.ps1
```

That runs `gofmt`, Biome format and lint, the TypeScript typecheck, the Vite build, `go vet`, and the full Go test suite. Frontend scripts (run from `frontend`):

| Command | Purpose |
| --- | --- |
| `bun run dev` | Start the Vite development server |
| `bun run build` | Type-check and build the frontend |
| `bun run format` | Apply Biome formatting |
| `bun run format:check` | Check Biome formatting without changes |
| `bun run lint` | Run Biome lint rules |
| `bun run typecheck` | Run TypeScript without emitting files |
| `bun run check` | Run all frontend checks and the build |

Canonical Go commands (repository root):

```powershell
go fmt ./...
go vet ./...
go test ./...
```

## Production build

Build the production Windows AMD64 application:

```powershell
mise exec -c "wails build -clean -platform windows/amd64 -nopackage -nocolour"
```

The executable is written to `build/bin/Cratebug.exe`. With NSIS installed, build the per-user installer instead:

```powershell
mise exec -c "wails build -clean -platform windows/amd64 -nsis -installscope user -nocolour"
```

The installer is written to `build/bin/Cratebug-amd64-installer.exe`.

## Coding standards

Follow [CODING_GUIDELINES.md](CODING_GUIDELINES.md). The short version:

- Go is formatted with `gofmt`, idiomatic, and keeps domain and filesystem behavior independent of Wails and React.
- The frontend is TypeScript and React checked by Biome; components render and interact, and filesystem policy lives in Go.
- Tests use disposable fixtures (`t.TempDir`) and never touch a real Marvel Rivals mod directory.
- Changes stay small and reviewable; comments explain why, not what.

## Project docs

- [SPEC.md](SPEC.md) - product behavior and boundaries
- [ROADMAP.md](ROADMAP.md) - phase order and status
- [AGENTS.md](AGENTS.md) - contributor and AI-agent workflow rules
- [docs/decisions/](docs/decisions/) - accepted technical decisions

## Commits and pull requests

- Commit messages follow Conventional Commits, as the history does: `feat(install): ...`, `fix(update): ...`, `chore: ...`.
- Pull requests target `master`. CI runs the canonical checks and a clean Windows build; it needs to pass before review.
- Keep generated files, secrets, caches, logs, and build output out of version control.

## License

By contributing, you agree that your contributions are licensed under the GNU General Public License version 3, matching [LICENSE](LICENSE).
