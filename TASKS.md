# Cratebug Active Tasks

**Phase:** 21 - Backup and restore
**Status:** Tasks 21.1 and 21.2 are implemented. Task 21.3 is in progress.

Phase 20 is complete (see `ROADMAP.md` for the summary). Phase 19 stays deferred.

## 21.1 Library backup [IMPLEMENTED] (2026-10-04)

Zip the full mod-root tree together with Cratebug's `metadata.json` from the config directory (at the zip root; no extra manifest file). Scan read-only; never mutate the library. Report `Backed up X mods (Y IoStore, Z Classic, W Invalid)`.

Backend only: `internal/backup` plus the `BackupLibrary`/`CancelBackup` Wails bindings in `app_backup.go`. No UI; menu wiring belongs to 21.3.

**Verify:**

1. Back up disposable fixture libraries: mixed IoStore/classic, disabled forms, nested folders, incomplete bundles, and tags. Confirm the zip holds `metadata.json` and a byte-identical tree.
2. Test cancellation mid-backup and unreadable-file failures. The library stays intact and the result is reported honestly.
3. Run `check.ps1` and the frontend tests.
4. Stop for review. Do not start 21.2. (App-drive verification of backup moves to 21.3, which owns all menu wiring.)

**Results (2026-10-04):**

- New `internal/backup` package: read-only scan, destination-outside-root guard, top-level `metadata.json` collision guard, missing-metadata tolerance, per-file progress, cancellation and failure cleanup of the partial zip.
- 7 new Go tests cover mixed-format counts, byte-identical archives, missing metadata, in-library destination, collision, cancellation, and missing library.
- `check.ps1` passed. All 88 frontend tests passed (unchanged).
- No app drive: no UI trigger exists until 21.3.

## 21.2 Library restore [IMPLEMENTED] (2026-10-04)

Backend only: `internal/backup` restore flow plus the `RestorePreview`/`RestoreApply`/`DiscardRestorePreview`/`CancelRestore` Wails bindings in `app_backup.go`. No UI; menu wiring belongs to 21.3.

Restore a user-chosen zip through staged extraction with install-grade traversal protection. Preview the zip file's date (labeled as such) with counts from a live scan of the staged contents; trust nothing in the zip. Replace the library wholesale with park+replace+rollback under the game-running lock, restoring `metadata.json` through the store's safe-write path. Report `Restored X mods (Y IoStore, Z Classic, W Invalid)` from a post-restore scan; invalid entries restore as-is and are counted, not dropped.

**Verify:**

1. Restore 21.1 backups and `metadata.json`-free zips into disposable libraries. Confirm byte-identical trees and matching reported counts; confirm metadata survives the round-trip and stays untouched when absent.
2. Test traversal/absolute-path/symlink zips (rejected pre-mutation), mid-restore failure and cancellation (previous library intact), restore while the game lock is held (blocked), and restore into a non-empty library (replacement warning shown, explicit confirmation required).
3. Run `check.ps1` and the frontend tests.
4. Stop for review. Do not start 21.3. (App-drive verification of restore moves to 21.3, which owns all menu wiring.)

**Results (2026-10-04):**

- Staged restore in `internal/backup`: `Preview` extracts with install-grade traversal/link protection, separates `metadata.json` from the tree, and scans staged contents live (zip date shown labeled as file date, never trusted). `Apply` parks the current tree aside, moves staged entries in with per-move progress, restores metadata through the store's safe-write path (unparseable or newer-schema metadata keeps the current file with a note), and verifies with a post-restore scan. Any failure rolls the parked tree back; a leftover park refuses rather than guessing.
- 11 new restore tests cover round-trip replacement, live preview counts, metadata-free and corrupt-metadata restores, traversal rejection, non-zip rejection, unknown tokens, discard, leftover-park refusal, metadata-failure rollback, and apply progress.
- 3 new app tests cover the restore game-running block and the missing-context errors for the dialog-backed bindings.
- `check.ps1` passed. All 88 frontend tests passed (unchanged).
- No app drive: no UI trigger exists until 21.3.

## 21.3 Wire Backup / Restore into the Tools menu [IN PROGRESS]

Replace the placeholder cards with one "Backup / Restore" card holding side-by-side Backup and Restore buttons bound to 21.1 and 21.2. The character-data and BentoMod placeholders are removed; the dialog holds only the live card.

**Verify:**

1. Add frontend presentation tests for the wired card.
2. Run `check.ps1` and the frontend tests.
3. Drive the app against a disposable fixture library. Run a backup and a restore from the wired menu; screenshot the menu, progress, and success states.
4. Stop for review.

**Watcher fix results (2026-10-04):**

- Tools releases all library directory handles before the menu opens. Scans and restore retries keep the watcher suspended until dismissal.
- Close and Escape wait until the active operation finishes. Dismissal restores the directory watches and refreshes the catalog.
- Three Go regression tests cover nested folder moves, cancellation and retry, root changes during suspension, and event detection after resume.
- `check.ps1` and all 96 frontend tests passed. The T3 browser verified handle release, event suppression, Close, Escape, and catalog refresh.
- Screenshots: `.playwright-cli/screenshots/phase21/task-21.3-tools-watcher-suspended.png` and `task-21.3-tools-watcher-resumed.png`. Native file-dialog verification remains incomplete because the Computer Use connection is unavailable.

**Review fixes (2026-10-04):**

- Apply owns each restore token until it finishes. Concurrent apply and discard cannot remove active staging.
- Retry requires a complete rollback. Failed rollback preserves recovery files, attempts independent return moves, and blocks retry.
- The watcher drains filesystem channels independently from the event handler. The 200-cycle suspension test completes during continuous file writes.
- Restore reports confirmed cancellation through its result. The UI shows rollback errors and preserves success after late Cancel.
- Successful restore reloads metadata and preferences. It also removes tag filters absent from the restored metadata.
- Six new Go tests cover session ownership, cancellation, late cancellation, Windows rollback locks, channel drainage, and suspension stress.
- `check.ps1` and all 96 frontend tests passed. The race detector could not run because CGO is disabled and no C compiler is available.
- Browser checks used controlled restore results on `C:\ModsFixtures`. They verified visible rollback errors, blocked unsafe retry, late-cancel success, restored tags and preferences, and obsolete filter removal.
- Screenshots: `.playwright-cli/screenshots/phase21/task-21.3-review-rollback-error.png` and `task-21.3-review-late-cancel-success.png`. Native file dialogs remain unverified.

The generated backup models have no trailing whitespace in the reviewed diff. `git diff --check` passed.
