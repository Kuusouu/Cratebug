# Phase 12 Review

**Date:** 2026-08-28
**Status:** Awaiting review

## Outcome

Phase 12 gives Cratebug provider-based library auto-detection. A new `internal/gamedetect` package defines a minimal provider interface and registry; the Steam provider is the only implementation, resolving Steam through the Windows registry (`golang.org/x/sys/windows/registry`, already a direct dependency) plus BentoMod's fallback roots, parsing `steamapps/libraryfolders.vdf`, and validating the `steamapps\common\MarvelRivals\MarvelGame\Marvel\Content\Paks` install shape. Detection is a typed three-state result: library found, install found without a library, or nothing found. The Wails-bound `DetectLibrary`/`CreateLibrary`/`SetLibraryProvider` methods expose it, `Settings.LibraryProvider` persists the active provider (empty means Steam), and the frontend routes the three states: apply/switch confirmation, a confirm-first create-library dialog, or a not-found toast. Detection is read-only; the one write outside a configured mod root is creating an empty `~mods` folder inside a provider-verified install, and only after the user confirms it.

Deliberate differences from BentoMod's auto-detect (the behavioral reference): Cratebug asks before creating `~mods` where BentoMod creates silently, and Cratebug re-detects server-side on creation so no frontend-supplied path can aim the write. The Epic Games provider is not implemented; the seam (interface, registry, provider-keyed logo map, settings validation) is in place for it to land without call-site changes, and the Settings selector shows Epic as visible-but-unavailable.

## Backend: `internal/gamedetect` (12.1)

`Provider` is a two-method interface; `Registry` dispatches by name and owns the confirmed-creation flow. `SteamProvider` takes its registry reader and fallback roots as fields, so tests fabricate Steam roots and `libraryfolders.vdf` fixtures under `t.TempDir` and never touch the real registry or install. The vdf parser handles Steam's `\\` escaping, both quoting styles, and de-duplicates near-identical paths case-insensitively (registry SteamPath is often forward-slashed and lowercase). A Paks path that exists as a file rather than a directory is skipped, not reported as an install. `CreateLibrary` re-runs detection, refuses without a verified install, uses plain `os.Mkdir` (parent must already exist, so nothing is created beyond the one folder), and returns the existing path unchanged if the library is already there.

## App bindings and settings (12.2)

`DetectLibrary(provider)` returns the three-state result; `CreateLibrary(provider)` re-detects and creates server-side; `SetLibraryProvider` validates through `gamedetect.ValidProvider` before persisting. `Settings.LibraryProvider` follows the additive-field pattern proven by `LastSeenVersion`: documents written before the field existed load with it empty, which means the default provider. The App's registry defaults to the production Steam registry and is a plain field, so tests substitute a stub provider.

## Frontend (12.3, 12.4)

The library toolbar gained a "Detect Steam library" button carrying the store's logo, and the initial "Choose a mod library" catalog state shows the same button as the primary affordance, with the pasted-path field retained as fallback. The Settings dialog gained a "Mod library detection" section: Steam selectable with a checkmark, Epic Games visible but disabled with a "coming soon" hint. `libraryDetection.ts` holds the outcome branching (apply, create, same-library, not-found) and the case-insensitive, trailing-separator-tolerant Windows path comparison as pure, unit-tested logic. The apply dialog confirms before switching when a library is already configured; a found library identical to the current root (any casing) rescans with an "already active" toast instead of a pointless dialog. The create dialog uses the approved copy: "No mod library was found for Steam" with an explicit statement that only that one folder is created. Store logos are Simple Icons path data rendered with `currentColor` so they hold up in both themes; NOTICE.md records the Valve and Epic trademarks.

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

Validation on the phase branch:

- **`check.ps1`:** Passed — gofmt clean, Biome format and lint clean, TypeScript typecheck clean, Vite production build succeeded, `go vet` clean, all 10 Go packages pass.
- **Go suite:** new coverage in `internal/gamedetect` (vdf parsing, all three detection states, first-library-in-order determinism, file-pretending-to-be-Paks rejection, registry-failure fallback, unknown-provider rejection, create-creates-only-the-missing-folder with a directory-listing assertion, idempotent re-create, refuse-without-install, `ValidProvider`), `internal/metadata` (provider accept/reject/empty-clear plus save-load round trip and pre-field documents loading empty), and root-package tests (provider dispatch, unknown-provider rejection, created-path surfacing, setting persistence with unknown-value rejection).
- **Frontend suite (`bun test`):** 52 tests pass, including 4 new `detectionOutcome` cases (apply, same-library across casing/trailing separators, create, not-found including a malformed libraryFound result).
- **Wails bindings** regenerated with `wails generate module`; `DetectLibrary`, `CreateLibrary`, `SetLibraryProvider`, the `gamedetect.Detection` model, and `Settings.libraryProvider` all emitted.

## End-to-end evidence

Driven live against the running `wails dev` build via playwright-cli (the maintainer had deliberately moved their real `~mods` folder so detection would hit the install-found-without-library state):

1. **Toolbar detect control** — the app loaded with `C:\ModsFixtures` (3 mods), the Steam-branded detect button rendered between the path field and Refresh. Screenshot: `docs/screenshots/phase-12/task-3-toolbar-detect-button.png`.
2. **Detect → create dialog** — clicking Detect produced exactly the install-found state: heading "No mod library was found for Steam", the real install path (`c:\program files (x86)\steam\...\Content\Paks`) shown in a code block, and the create offer. Screenshot: `docs/screenshots/phase-12/task-3-detect-create-library-dialog.png`.
3. **Confirmed creation** — clicking "Create library" closed the dialog, set the path field to the new `~mods` path, and rescanned automatically to the empty state ("No supported mods found"). A filesystem check confirmed the Paks directory gained exactly one entry, the empty `~mods` folder, with the game's own files untouched. Screenshot: `docs/screenshots/phase-12/task-3-created-library-scanned.png`.
4. **Re-detect after creation** — clicking Detect again hit the same-library branch: no dialog, rescan, and the "This library is already active." toast, verified in the accessibility snapshot.
5. **Settings provider section** — Steam pressed/selected with checkmark, Epic Games disabled, hint text present. Screenshot: `docs/screenshots/phase-12/task-3-settings-provider-section.png`.
6. **Restore** — the library was pointed back at `C:\ModsFixtures` through the normal path-field flow and scanned to the expected 3 mods, confirming regular scanning is unaffected.

The maintainer's real Steam installation was used read-only for detection, and for the one confirmed creation exactly as set up beforehand.

## Known limitations and deferred findings

1. **Epic Games provider not implemented.** By design this phase: the selector option is disabled, and the follow-up needs the maintainer's Epic installation to verify the `InstallLocation` layout before the second provider is written.
2. **Browser-session parity.** The end-to-end drive ran in the wails dev browser tab, not the native window; the shared Go backend means the creation and persistence results are real, but per-window frontend state (theme, dialogs) was exercised only in the browser. The maintainer separately drove the same dialog in the native window during development (the comma-spacing fix came from that pass).
3. **`notFound` live state not exercised.** The maintainer's machine has Steam installed, so the not-found path is covered by unit tests and code review rather than a live run; producing it live would require uninstalling or hiding the game, which was not worth doing.
4. **Detection runs on demand only.** Per the phase's out-of-scope list, there is no background or periodic re-detection; the toolbar button and the initial-state affordance are the only entry points.
5. **Console noise pre-existing.** Two console errors appear in the dev session (a Wails `ipc.js` TypeError at page load and a favicon 404); both predate this phase and are unrelated to detection.

## Review decision

**Pending maintainer review.** All Phase 12 tasks (12.1 through 12.5) are implemented with the canonical checks, unit suites, and a live end-to-end drive passing. Approval closes the phase; the Epic Games provider follow-up is then scoped against the installed Epic copy.
