# Phase 13 Review

**Date:** 2026-09-02
**Status:** Approved

## Outcome

Phase 13 adds the Epic Games library-detection provider behind the Phase 12 seam. `EpicProvider` reads `%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests\*.item`, matches Marvel Rivals by the verified `CatalogNamespace` or `DisplayName`, skips incomplete installs and DLC, and validates `{InstallLocation}\MarvelGame\Marvel\Content\Paks`. The same three-state result as Steam (`libraryFound` / `installFound` / `notFound`) is returned. `CreateLibrary` is unchanged: it still re-detects by provider name and creates only `Paks\~mods` after confirmation. Steam remains the default (empty `LibraryProvider`). No new Wails methods.

Live layout used for matching, verified on this machine: install folder `C:\Program Files\Epic Games\MarvelRivalsjKtnW` (Epic's random suffix), Paks at `{InstallLocation}\MarvelGame\Marvel\Content\Paks`. Steam's `steamapps\common\MarvelRivals` prefix is not present.

## Backend: `EpicProvider` (13.1, 13.2)

`internal/gamedetect/epic.go` keeps types and constants above functions. Manifests directory is an injectable field so tests write fixture `.item` files under `t.TempDir` and never touch ProgramData or the real game. `.item` files are read in sorted filename order; unreadable or invalid JSON is skipped. `ProviderEpic` is registered in `providerNames` and `NewDefaultRegistry` together. `ValidProvider("epic")` is now true; unknown-provider tests use `"egs"`.

## Frontend (13.3)

`"epic"` is in `libraryProviders`, labels, and `providerLogos`. Settings lists Epic as a selectable option; the disabled "coming soon" control is gone. `DetectLibraryDialog` renders `providerLogos[provider]` instead of a hardcoded Steam logo. Toolbar copy was already provider-aware.

## Docs (13.4)

`docs/USER_GUIDE.md` and `docs/TROUBLESHOOTING.md` name both Steam and Epic. Troubleshooting notes that Epic reads launcher manifests and that the install folder name often has a random suffix.

---

## Commands and tests run

```powershell
.\check.ps1
```

```powershell
go test ./internal/gamedetect/ ./internal/metadata/ -count=1
```

```powershell
bun test
```

Validation on `feat/epic-gamedetect`:

- **`check.ps1`:** Passed. gofmt clean, Biome format and lint clean, TypeScript typecheck clean, Vite production build succeeded, `go vet` clean, all 10 Go packages pass.
- **Go suite:** new coverage in `internal/gamedetect` (Epic library-found, install-found, missing manifests, wrong game, incomplete install, DLC, Paks-as-file, first `.item` in sorted filename order, DisplayName match when namespace differs, malformed JSON skipped, `ValidProvider` accepts `epic`) and `internal/metadata` (accepts `epic`, still rejects `egs`).
- **Frontend suite (`bun test`):** 52 tests pass. Outcome branching is provider-agnostic and unchanged.
- **Wails bindings:** not regenerated. No new exported methods.

## End-to-end evidence

Driven live against the running `wails dev` build via playwright-cli (Chromium, `http://localhost:34115`). Detection is read-only. **Create library was not confirmed** on the earlier empty-Paks pass.

1. **Toolbar detect control** — empty library state, "Detect Epic Games library" with the Epic logo in the toolbar and as the empty-state affordance. Screenshot: `docs/screenshots/phase-13/task-3-toolbar-detect-button.png`.
2. **Settings provider section** — Steam and Epic Games both selectable; Epic Games pressed with checkmark; no "coming soon" hint. Screenshot: `docs/screenshots/phase-13/task-3-settings-provider-section.png`.
3. **Detect → create dialog** — heading "No mod library was found for Epic Games", Epic logo, path `C:\Program Files\Epic Games\MarvelRivalsjKtnW\MarvelGame\Marvel\Content\Paks`, create offer. Cancel closed the dialog with no write. Screenshot: `docs/screenshots/phase-13/task-3-detect-create-library-dialog.png`.
4. **Detect → library found** — after the maintainer added `~mods` with fixtures, Detect reported the same library, showed "This library is already active.", and rescanned 74 mods from `...\Paks\~mods`. Screenshots: `docs/screenshots/phase-13/task-5-already-active-toast.png`, `docs/screenshots/phase-13/task-5-epic-library-scanned.png`.

The playwright tab is a separate process from the native WebView2 window. Shared Go backend means detection results are real. Size- and DPI-sensitive layout was not checked in the native window.

## Known limitations and deferred findings

1. **`notFound` not exercised live.** This machine has Marvel Rivals installed through Epic. Unit tests cover empty manifests, wrong game, incomplete, and DLC.
2. **Browser-session parity.** The drive ran in the wails dev browser tab, not the native window.
3. **Steam remains the default.** Users with only Epic pick it in Settings.
4. **Console noise pre-existing.** Two console errors appear in the dev session (a Wails `ipc.js` TypeError at page load and a favicon 404); both predate this phase.

## Review decision

**Decision:** Approved. All Phase 13 tasks (13.1 through 13.5) and exit criteria are met per the canonical checks, the unit suites, and the live end-to-end drive (`installFound` create dialog and `libraryFound` already-active rescan). Phase 13 is complete.
