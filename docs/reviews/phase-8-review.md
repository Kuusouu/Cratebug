# Phase 8 Review

**Date:** 2026-08-26
**Status:** Approved

## Outcome

Phase 8 gives Cratebug the ability to install mods from local archives (zip, 7z, tar, tar.gz, rar) or direct `.pak`/`.utoc`/`.ucas` selections, instead of requiring users to place files in the library folder themselves. Archive handling is unified behind `github.com/mholt/archives` in a new `internal/install` package, with staged extraction, path-traversal and unsafe-link rejection, bundle discovery, collision detection, and a transactional apply that rolls back cleanly on partial failure. The frontend gained an `InstallPreviewDialog` that shows every mod's derived name (editable), destination (defaulting to the current library view), format, size, source archive, and any collision or staging warning before anything touches the library, with per-mod collision resolution by rename or overwrite. Both triggers required by 8.4 are wired: an "Install mod" button opening the native OS file picker, and real drag-and-drop onto the application window.

A safety gap surfaced during review is closed as part of this phase: a companion sidecar failing to copy during staging previously produced a silently incomplete or misclassified bundle with no warning anywhere in the pipeline. Staged mods now carry discovery issues end to end, and the preview blocks installing any selected item that has one.

---

## Archive unification and extraction safety (8.1)

`internal/install/archive.go` wraps `github.com/mholt/archives` behind a single `ExtractArchive` entry point used for every supported format, replacing any need for format-specific extraction code. During extraction:

- Archive entries containing symlinks or hard links are rejected outright (`info.Mode()&fs.ModeSymlink != 0 || info.LinkTarget != ""`).
- Every entry's path is cleaned and checked to stay within the destination directory before any file is written, rejecting zip-slip-style path traversal.
- `hasBundleExtension` recognizes `.pak` and its disabled-suffix variants plus `.utoc`/`.ucas` as raw Unreal bundle files rather than archives to unpack, so a bare `.pak` selection and an archive selection share the same staging path.

## Backend installation pipeline (8.2)

`internal/install/stage.go` and `apply.go` implement the staged, transactional pipeline:

- **Staging:** Every selected file or archive is extracted or copied into a session-scoped temp directory before anything reaches the real library. Loose `.pak` selections automatically pull in same-stem companion sidecars and disabled-suffix variants from the same source directory, so selecting only the `.pak` of a complete IoStore bundle still stages the full set.
- **Discovery:** The staged directory is scanned with the existing `internal/discovery` scanner to identify classic and IoStore bundles, reusing the same bundle-format and issue detection the library view already relies on.
- **Collision detection:** `BuildPreview` checks each staged mod's destination against the live library for matching filenames or display names, surfacing a description and the colliding entry's ID rather than a bare boolean.
- **Transactional apply:** `Apply` plans every file move up front, executes them in order, and on any failure partway through rolls back created files and restores overwritten ones from a backup taken before each overwrite, then rescans and reports the reconciled library either way.
- **Progress and cancellation:** Staging reports phase/percent progress via Wails runtime events, and an in-flight or unapplied session can be cancelled through `CancelInstall`, which removes the staging directory with no trace left behind.

## Installation preview UI (8.3)

`frontend/src/library/InstallPreviewDialog.tsx` and `installPresentation.ts` implement the preview:

- Each staged mod renders as its own card: an editable name field, a destination-folder dropdown (defaulting to wherever the user is currently browsing), format and size badges, hero/category classification when resolvable, and a collapsible file list showing each file's archive origin.
- Collision handling is per mod: a live collision (recomputed as the user edits the name or destination, not just the initial backend snapshot) shows a warning banner with an explanation and an "Overwrite existing mod" checkbox; editing the name to something distinct clears it without needing to overwrite anything.
- **Staging-issue warnings (the safety fix):** `StagedMod` and `PreviewItem` now carry `discovery.Issue`s, populated both from the underlying scanner (a genuinely incomplete bundle) and from a new `companionCopyFailures` signal tracked during staging (a sidecar that was present on disk but failed to copy, with any partial write cleaned up so it can't masquerade as present). Any selected item with an issue blocks installing until it's excluded, the same way an unresolved collision does.
- A dedicated empty state distinguishes "nothing installable was found" from a normal preview, and initial focus moves into the dialog on open, matching every other dialog in the app.

## Drag-and-drop and install triggers (8.4)

- `main.go` enables Wails' native `DragAndDrop` option (`EnableFileDrop`, `DisableWebViewDrop` as a safety net against the WebView falling back to its own file-open behavior).
- `LibraryScreen.tsx` registers `OnFileDrop` for real filesystem paths and separately tracks drag-enter/leave state (distinguishing real OS file drags from the app's own internal drag-to-organize system by `dataTransfer.types`) to show a drop-target overlay across the whole window.
- The existing "Install mod" header button opens the native multi-file picker via `SelectFilesForInstall`.
- Both triggers feed the same `installDialogFiles` state and the same preview dialog.

---

## Commands and tests run

```powershell
.\check.ps1
```

Validation output:

- **Go formatting (`gofmt`):** Passed with 0 changes.
- **Frontend checks:** Biome format check, Biome lint (0 errors, 0 warnings), TypeScript typecheck (`tsc --noEmit`), and the Vite production build all passed clean.
- **Go vet (`go vet ./...`):** Passed with 0 warnings.
- **Go test suite (`go test ./...`):** 203 tests passed across the repo, 19 of them in `internal/install`, covering staged extraction across zip/tar/7z/rar, path-traversal and symlink rejection, collision detection, transactional apply with rollback, cancellation cleanup, and companion-sidecar edge cases (disabled-suffix variants, differently-stemmed siblings excluded, a copy failure surfacing as a warning instead of an incomplete install). Rar coverage includes a real multi-volume archive with only its first volume selected, which fails cleanly rather than truncating silently since Cratebug reads one file at a time, and a real empty single-volume rar, which decodes without error and is rejected the same way any archive with no mod files is.
- **Frontend test suite (`bun test`):** 24 tests passed in `installPresentation.test.ts`, covering collision detection (including the live-recompute and initial-snapshot-fallback branches), batch-collision detection within one install, and mod-name validation.

## Manual checks

1. **Live app verification (real fixtures, `wails dev`):** Drove the running app directly (via `PrepareInstall`/`ApplyInstall` for backend-level checks, and the real UI for drag-and-drop and the preview dialog) against the fixtures in `C:\ModInstallations`: classic-only, IoStore across all four archive formats, a real bare `.pak` with automatic sidecar discovery, a multi-bundle archive installing two mods at once, a nested-folder archive, both invalid fixtures rejected cleanly, and a real `.rar` (`AkkabanAcolyteUI_9999999_P.rar`) installing correctly with classification. Collision detection was exercised for real by reinstalling the same mod into the same folder, then resolved once by rename and once by overwrite. A prepared-but-not-applied session was cancelled and left zero trace on disk.
2. **Drag-and-drop:** confirmed working against the actual native application window, not just the dev-server browser proxy used for other checks.
3. **Screenshots:**
   - `docs/screenshots/phase-8/task-8.5-library-empty.png` captures the empty-library state (`ModsFixtures` cleared) before an install.
   - `docs/screenshots/phase-8/task-8.3-install-preview-collision.png` captures the installation preview dialog mid-flow with two real mods staged from real archives (one from a `.zip`, one from a `.rar`), a live collision banner with its overwrite checkbox and rename hint, hero classification, and the disabled "Install 2 mods" button reflecting the unresolved collision.

## Known limitations and deferred findings

1. **Native-window-specific behavior isn't provable from a separate browser instance.** `useDialogFocusTrap.ts`'s wheel-lock (needed to keep the library from scrolling behind an open dialog) depends on the native WebView2 window's real dimensions and DPI; a separately-launched browser at a synthetic viewport size showed no scroll leak with or without it, which turned out to be a property of that test window's size, not proof the mechanism is unnecessary. Confirmed necessary against the real native window instead.
2. **Bare-sidecar-selection edge case.** If a user selects only a `.utoc`/`.ucas` and its `.pak` companion fails to copy during staging, the mod has no primary at all and disappears from the preview silently, rather than surfacing a warning the way the more common partial-sidecar case now does. Closing this fully needs staging to also surface orphaned-sidecar entries as flagged candidates, which is a larger change than this phase's remediation covered.
3. **Rar test fixtures.** A rar archive containing zero mod files is covered by a real, committed fixture. A rar containing real mod content was verified manually against a fixture supplied for testing, not committed, since it's real mod data rather than something to check into the repository. No Go tooling available in this environment can author a new rar archive (the format is proprietary and read-only in the open-source ecosystem), so both fixtures used here are either reused from a dependency's own test suite or supplied directly rather than synthesized.

## Review approval

**Decision:** Approved. All Phase 8 tasks (8.1 through 8.5) and exit criteria are complete, verified against real fixtures and the live native application, and ready for Phase 9.
