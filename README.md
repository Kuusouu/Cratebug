<div align="center">

<img src="frontend/src/assets/logo.png" alt="Cratebug" width="120" />

# Cratebug

</div>

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals.

The project is a fresh implementation built with Go, Wails, React, and TypeScript, built to be what BentoMod was meant to be: a better implementation of [Repak-X](https://github.com/XzantGaming/Repak-X). It aims to preserve useful workflows and compatibility with existing mod libraries while making filesystem operations safe, predictable, and recoverable.

## Project status

Cratebug is under active development and not yet ready for general use. See
[ROADMAP.md](ROADMAP.md) for full phase detail. This table tracks feature
status at a glance.

[implemented]: https://img.shields.io/badge/Implemented-3ddc97?style=flat-square
[next]: https://img.shields.io/badge/Next-8ab6e0?style=flat-square
[deferred]: https://img.shields.io/badge/Deferred-9e9e9e?style=flat-square

| Feature | Status |
| --- | --- |
| Read-only mod discovery and library browsing | ![Implemented][implemented] |
| Safe mod enable/disable | ![Implemented][implemented] |
| Rename, priority, folder organization, recoverable deletion | ![Implemented][implemented] |
| Metadata and settings persistence, tags | ![Implemented][implemented] |
| Mod type/hero/skin classification via UAssetToolRivals | ![Implemented][implemented] |
| Archive installation (zip/7z/tar/rar, drag-and-drop) | ![Implemented][implemented] |
| Asset conflict detection and inspection | ![Implemented][implemented] |
| Automatic app updates, remote mod downloads | ![Next][next] |
| Release hardening (signing, upgrades, accessibility) | ![Next][next] |
| BentoMod/Repak-X state import | ![Deferred][deferred] |
| Batch operations, filesystem watching, full backup/restore | ![Deferred][deferred] |
| Browser intake, game launching, crash monitoring | ![Deferred][deferred] |
| Character data updates, recompression, VFX updating | ![Deferred][deferred] |
| Virtual collections, permanent deletion, external-rename reconciliation | ![Deferred][deferred] |

See:

- [Product specification](SPEC.md)
- [Roadmap](ROADMAP.md)
- [Active tasks](TASKS.md)
- [Contributor and agent guidance](AGENTS.md)
- [Toolchain baseline](docs/decisions/0001-toolchain-baseline.md)
- [Organize action pattern](docs/decisions/0002-organize-action-pattern.md)

## Prerequisites

Development currently targets 64-bit Windows 10 version 1909 or newer and Windows 11. Install:

- Git and [Git LFS](https://git-lfs.com/)
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

Initialize Git LFS and clone the repository:

```powershell
git lfs install
git clone https://github.com/Kuusouu/Cratebug.git
Set-Location Cratebug
git lfs pull
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
