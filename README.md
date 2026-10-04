<div align="center">

<img src="frontend/src/assets/logo.png" alt="Cratebug" width="120" />

# Cratebug

**A fast, safe mod manager for Marvel Rivals**

[![Latest release](https://img.shields.io/github/v/release/Kuusouu/Cratebug?style=flat-square)](https://github.com/Kuusouu/Cratebug/releases/latest)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue?style=flat-square)](LICENSE)

<img src="main.png" alt="The Cratebug library: mods with hero portraits, category and priority badges, and enable toggles" width="100%" />

</div>

Cratebug is an open-source, Windows-first (and Linux soon!) mod manager for Marvel Rivals. It treats related mod files as one logical bundle, makes every filesystem change planned and recoverable, and shows you exactly what is about to happen before it happens.

## Features

- **Library auto-detect** - find your Marvel Rivals mod folder from Steam or Epic Games, or paste the path yourself
- **Whole-library view** - classic and IoStore mods side by side, with hero, skin, category, and portrait detected automatically
- **Safe enable and disable** - flip mods on and off without breaking their sidecar files
- **Organization** - rename, set priority, sort into folders, and tag mods; tags and settings survive renames and moves
- **Recoverable deletion** - mods and folders go to the Recycle Bin, never straight to the void
- **Archive installs with a preview** - drop in a `.zip`, `.7z`, `.rar`, or bare `.pak` and review exactly what will be installed first
- **Nexus Mods installs** - paste your own API key, then a Marvel Rivals mod page. Premium accounts can download with a simple URL, free accounts use **Mod Manager Download** button in Nexus!
- **Conflict detection** - find mods stepping on the same assets, with a one-click priority fix
- **Self-updating** - check for updates in Settings, download, restart, done
- **Linux support** - standalone AppImage with Proton game detection, FreeDesktop trash, and Secret Service integration

## Install

### Windows

1. Download `Cratebug-amd64-installer.exe` from the [latest release](https://github.com/Kuusouu/Cratebug/releases/latest).
2. Run it. Cratebug installs for your user account only - no administrator rights needed.
3. Launch it from the Start Menu or the desktop shortcut.

Cratebug is not code-signed yet, so Windows SmartScreen may warn on first run. See [troubleshooting](docs/TROUBLESHOOTING.md).

### Linux

1. Download `Cratebug-x86_64.AppImage` from the [latest release](https://github.com/Kuusouu/Cratebug/releases/latest).
2. Make the file executable:
   ```bash
   chmod +x Cratebug-x86_64.AppImage
   ```
3. Run the AppImage:
   ```bash
   ./Cratebug-x86_64.AppImage
   ```

Runtime requirements (these may already be installed):
- 64-bit Linux distribution with glibc 2.35 or newer (Ubuntu 22.04+, Debian 12+, Fedora 36+, Arch, SteamOS, Bazzite).
- WebKit2GTK 4.1 (`libwebkit2gtk-4.1-0` on Ubuntu/Pop!_OS/Debian, `webkit2gtk4.1` on Fedora, `webkit2gtk-4.1` on Arch/CachyOS).
- FUSE 2 (`libfuse2` on Ubuntu/Debian) or run with `--appimage-extract-and-run`.

The AppImage opened in Hyper-V VMs on [Fedora](docs/screenshots/fedora-initial-launch.png) and [Pop!_OS](docs/screenshots/popos-initial-launch.png) without extra packages.
[CachyOS](docs/screenshots/cachyos-initial-launch.png) required `sudo pacman -Syu webkit2gtk-4.1`.

## Finding your library

On first launch, open **Settings**, pick **Steam** or **Epic Games** under **Mod library detection**, and click the detect button in the toolbar. If the game is installed but the mod folder is not there yet, Cratebug asks before creating it. You can always paste a folder path instead. Details in the [user guide](docs/USER_GUIDE.md).

## Updating

Open **Settings** and click **Check for updates**. If a newer release exists, Cratebug shows the changelog, downloads it, and applies it silently on restart. You can also update manually from the [releases page](https://github.com/Kuusouu/Cratebug/releases/latest).

## Installing mods

Use the install button, drag and drop files onto the window, or the download icon to install from Nexus Mods. Every path ends at the same preview: see the destination folder, the mod name, and any collisions before anything is written. Details in the [user guide](docs/USER_GUIDE.md).

## Building from source

### Windows

You need 64-bit Windows 10 (1909+) or 11, Git with [Git LFS](https://git-lfs.com/), Go `1.26.5`, Bun `1.3.14`, and the Microsoft WebView2 Runtime:

```powershell
git lfs install
git clone https://github.com/Kuusouu/Cratebug.git
Set-Location Cratebug
Push-Location frontend
bun install --frozen-lockfile
Pop-Location
.\fetch-uassettool.ps1          # fetch the pinned UAssetTool worker
wails dev                       # run the app
.\check.ps1                     # run every check
```

### Linux

You need a 64-bit Linux distribution (Ubuntu 22.04+ recommended), Git with Git LFS, Go `1.26.5`, Bun `1.3.14`, and development libraries for GTK 3 and WebKit2GTK 4.1:

```bash
# Ubuntu / Debian prerequisites
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev

git lfs install
git clone https://github.com/Kuusouu/Cratebug.git
cd Cratebug
cd frontend && bun install --frozen-lockfile && cd ..
./fetch-uassettool.sh          # fetch the pinned UAssetTool worker and Oodle library
./build-linux.sh               # build the standalone AppImage
./check.sh                     # run every check
```

The full contributor workflow, including building the installer, is in [CONTRIBUTING.md](CONTRIBUTING.md).

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for getting started, the coding standards, and the project's conventions.

## Documentation

- [User guide](docs/USER_GUIDE.md) - installing, finding your library, updating, and URL installs in detail
- [Troubleshooting](docs/TROUBLESHOOTING.md) - SmartScreen, WebView2, uninstalling, and other common problems
- [Changelog](CHANGELOG.md)
- [Roadmap](ROADMAP.md) - what is done and what is next

## License

Cratebug is licensed under the GNU General Public License version 3. See [LICENSE](LICENSE) and [NOTICE.md](NOTICE.md) for origin and credits.
