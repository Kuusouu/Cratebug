# Cratebug

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals.

The project is a fresh implementation built with Go, Wails, React, and TypeScript. It aims to preserve useful workflows and compatibility with existing mod libraries while making filesystem operations safe, predictable, and recoverable.

## Project status

Cratebug has completed its Phase 0 repository and toolchain foundation and
Phase 1 read-only discovery scanner. Phase 2 read-only library UI work has not
started. The application currently contains a minimal shell and one
frontend-to-Go connectivity check while the discovery behavior is being
integrated into the interface.

See:

- [Product specification](SPEC.md)
- [Roadmap](ROADMAP.md)
- [Active tasks](TASKS.md)
- [Contributor and agent guidance](AGENTS.md)
- [Toolchain baseline](docs/decisions/0001-toolchain-baseline.md)

## Prerequisites

Development currently targets 64-bit Windows 10 version 1909 or newer and Windows 11. Install:

- Git
- [`mise`](https://mise.jdx.dev/installing-mise.html)
- Microsoft WebView2 Runtime
- NSIS 3 when building the installer

Install `mise` and NSIS with Windows Package Manager:

```powershell
winget install jdx.mise
winget install NSIS.NSIS --silent
```

Restart the terminal after installing system tools so their updated paths are available.

## Setup

Clone the repository:

```powershell
git clone https://github.com/Kuusouu/Cratebug.git
Set-Location Cratebug
```

Install the pinned Go and Bun versions through `mise`:

```powershell
mise install
```

Install the pinned Wails CLI:

```powershell
mise exec -c "go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0"
$env:Path = "$(mise exec -c "go env GOPATH")\bin;$env:Path"
```

Install frontend dependencies from the committed Bun lockfile:

```powershell
Push-Location frontend
mise exec -c "bun install --frozen-lockfile"
Pop-Location
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

## Production build

Build the production Windows AMD64 application from the repository root:

```powershell
mise exec -c "wails build -clean -platform windows/amd64 -nopackage -nocolour"
```

The production executable is written to `build/bin/Cratebug.exe`. The generated `build` directory is intentionally ignored by Git.

## Windows installer

After installing NSIS, build the per-user Windows AMD64 installer:

```powershell
mise exec -c "wails build -clean -platform windows/amd64 -nsis -installscope user -nocolour"
```

The installer is written to `build/bin/Cratebug-amd64-installer.exe`. A default installation uses `%LOCALAPPDATA%\Programs\Cratebug`, creates Start menu and desktop shortcuts, and can be removed from Windows Settings or with the installed `uninstall.exe`.

Phase 0 packages are unsigned, so Windows may display an unrecognized-app warning. Signing, upgrades, and release publishing are deferred to release hardening.

## Continuous integration

GitHub Actions runs the canonical checks and a clean Windows application build on Blacksmith for pushes and pull requests to `master`. CI verifies the application without publishing packages or build artifacts.

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE).
