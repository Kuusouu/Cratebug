<div align="center">

<img src="frontend/src/assets/logo.png" alt="Cratebug" width="120" />

# Cratebug

**A fast, safe mod manager for Marvel Rivals**

[![Latest release](https://img.shields.io/github/v/release/Kuusouu/Cratebug?style=flat-square)](https://github.com/Kuusouu/Cratebug/releases/latest)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue?style=flat-square)](LICENSE)

<img src="main.png" alt="The Cratebug library: mods with hero portraits, category and priority badges, and enable toggles" width="100%" />

</div>

Cratebug is an open-source, Windows-first mod manager for Marvel Rivals. It treats related mod files as one logical bundle, makes every filesystem change planned and recoverable, and shows you exactly what is about to happen before it happens.

## Features

- **Whole-library view** - classic and IoStore mods side by side, with hero, skin, category, and portrait detected automatically
- **Safe enable and disable** - flip mods on and off without breaking their sidecar files
- **Organization** - rename, set priority, sort into folders, and tag mods; tags and settings survive renames and moves
- **Recoverable deletion** - mods go to the Recycle Bin, never straight to the void
- **Archive installs with a preview** - drop in a `.zip`, `.7z`, `.rar`, or bare `.pak` and review exactly what will be installed first
- **Install from a URL** - paste a direct download link, get the same preview flow
- **Conflict detection** - find mods stepping on the same assets, with a one-click priority fix
- **Self-updating** - check for updates in Settings, download, restart, done

## Install

1. Download `Cratebug-amd64-installer.exe` from the [latest release](https://github.com/Kuusouu/Cratebug/releases/latest).
2. Run it. Cratebug installs for your user account only - no administrator rights needed.
3. Launch it from the Start Menu or the desktop shortcut.

Cratebug is not code-signed yet, so Windows SmartScreen may warn on first run. See [troubleshooting](docs/TROUBLESHOOTING.md).

## Updating

Open **Settings** and click **Check for updates**. If a newer release exists, Cratebug shows the changelog, downloads it, and applies it silently on restart. You can also update manually from the [releases page](https://github.com/Kuusouu/Cratebug/releases/latest).

## Installing mods

Use the install button, drag and drop files onto the window, or paste a direct download link (the link icon in the header). Every path ends at the same preview: see the destination folder, the mod name, and any collisions before anything is written. Details in the [user guide](docs/USER_GUIDE.md).

## Building from source

You need 64-bit Windows 10 (1909+) or 11, Git with [Git LFS](https://git-lfs.com/), and the Microsoft WebView2 Runtime. [`mise`](https://mise.jdx.dev/) pins the toolchain but is optional:

```powershell
git lfs install
git clone https://github.com/Kuusouu/Cratebug.git
Set-Location Cratebug
mise install
Push-Location frontend
mise exec -c "bun install --frozen-lockfile"
Pop-Location
mise exec -c "wails dev"        # run the app
.\check.ps1                     # run every check
```

The full contributor workflow, including running without `mise` and building the installer, is in [CONTRIBUTING.md](CONTRIBUTING.md).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for getting started, the coding standards, and the project's conventions.

## Documentation

- [User guide](docs/USER_GUIDE.md) - installing, updating, and URL installs in detail
- [Troubleshooting](docs/TROUBLESHOOTING.md) - SmartScreen, WebView2, uninstalling, and other common problems
- [Changelog](CHANGELOG.md)
- [Roadmap](ROADMAP.md) - what is done and what is next

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE) and [NOTICE.md](NOTICE.md) for origin and credits.
