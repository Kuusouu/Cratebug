# Cratebug Active Tasks

**Phase:** 18 - Linux distribution
**Status:** Active

This file contains only the active work. Do not start the next phase.

## Objective

Make Cratebug run on Linux as a first-class build. Marvel Rivals runs on Linux through Proton with working anti-cheat; the managed mods are the same Windows bundles in the same Steam library layout. The goal is to make Cratebug portable without changing mod formats or core architecture.

## Design decisions

* **Ubuntu 22.04 as build baseline.** Building on Ubuntu 22.04 links against glibc 2.35, ensuring the resulting AppImage runs across Ubuntu, Debian, Fedora, Bazzite, Arch, and SteamOS.
* **AppImage primary distribution.** A single-file AppImage provides maximum portability without sandboxing constraints or immediate app store review queues.
* **Safe trash semantics.** File deletion on Linux must route through FreeDesktop Trash (`gio trash` or `~/.local/share/Trash`). Deletion must fail with an explicit error rather than silently degrading to permanent file deletion.
* **Secret storage with fallback.** Use the FreeDesktop Secret Service API (via D-Bus) for Nexus API key storage, with a secure local key file (`0600` permissions under `~/.config/cratebug/`) when no secret daemon is available.
* **Proton process inspection.** The running-game check reads `/proc` to detect `marvel-win64-shipping.exe` running under Proton or Wine.
* **Desktop integration via FreeDesktop standards.** `nxm://` protocol handling uses `~/.local/share/applications/cratebug-nxm.desktop` and `xdg-mime`. File reveal uses D-Bus `FileManager1.ShowItems` or `xdg-open`.

## Out of scope

* macOS and ARM architectures
* Flatpak / Flathub submission (planned for post-release)
* Native distribution package managers (.deb, .rpm, AUR)
* Managing Proton prefixes differently from standard Steam libraries

---

## 18.1 Portable Go foundation and build tag isolation

Isolate Windows-specific imports behind `//go:build windows` tags and create portable stubs:
- Add `window_other.go` with `primaryWorkArea() (int, int)` returning `(0, 0)`.
- Isolate `internal/update/apply.go` behind `//go:build windows` and add `apply_other.go` stub.
- Split `internal/uassettool/worker.go` executable naming (`UAssetTool` vs `UAssetTool.exe`) and process attributes (`HideWindow`).
- Split `internal/mutation/mutation.go` (`moveFileWithoutReplace`) and `internal/mutation/folders.go` (`requireDirectory`).
- Add `//go:build windows` to `internal/mutation/recycle_windows.go` and `internal/mutation/game_running_windows.go`.

**Verify:** `GOOS=linux go vet ./...` passes on Windows without missing symbol or import errors.

---

## 18.2 Linux game detection and running-game process check

Implement Linux Steam detection and running-game check:
- Split `internal/gamedetect/steam.go` into `steam_windows.go` and `steam_linux.go`.
- On Linux, check `~/.local/share/Steam`, `~/.steam/steam`, `~/.steam/root`, and Flatpak Steam paths. Parse `steamapps/libraryfolders.vdf` using the existing parser.
- Implement `internal/mutation/game_running_linux.go` by inspecting `/proc` for `marvel-win64-shipping.exe`.

**Verify:** `go test ./internal/gamedetect ./internal/mutation -run "TestSteam|TestGameRunning" -count=1` passes with mock filesystem and `/proc` fixtures.

---

## 18.3 Linux file operations and desktop integration

Implement safe Linux file mutations and desktop hooks:
- Implement `internal/mutation/recycle_linux.go` using `gio trash` with fallback to FreeDesktop trash spec.
- Implement `internal/reveal/open_linux.go` using D-Bus `org.freedesktop.FileManager1.ShowItems` or `xdg-open`.
- Implement `internal/urlscheme/registry_linux.go` using `.desktop` entry registration and `xdg-mime`.

**Verify:** `go test ./internal/mutation ./internal/reveal ./internal/urlscheme -count=1`.

---

## 18.4 Linux secret storage and worker packaging

Implement Linux credential protection and worker binary extraction:
- Implement `internal/secret/protect_linux.go` supporting Secret Service via D-Bus with permission-checked `0600` local fallback.
- Create `fetch-uassettool.sh` (or update download script) for `UAssetTool-linux-x64.tar.gz`.
- Ensure executable permissions (`0755`) on extraction and confirm runtime Oodle library resolution.

**Verify:** `go test ./internal/secret ./internal/uassettool -count=1`.

---

## 18.5 Wails Linux packaging and AppImage build

Configure Wails for Linux builds:
- Validate WebKit2GTK compilation on Ubuntu 22.04 / WSL2.
- Create AppImage packaging recipe and `build-linux.sh`.
- Verify application launch, UI rendering, and window sizing.

**Verify:** `wails dev` launches and serves the UI. AppImage builds and runs standalone.

---

## 18.6 Cross-distro verification and CI

Complete testing and CI integration:
- Run canonical check script on Linux: Go tests, Biome linting, and frontend tests.
- Validate mod lifecycle: scan, enable, disable, move, rename, delete to trash, encrypt, strip companion, install.
- Update GitHub Actions workflow to build and attach Linux AppImage to releases.
- Document distro prerequisites in `README.md`.

**Verify:** All checks pass on Linux. AppImage runs cleanly.
