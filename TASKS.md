# Cratebug Active Tasks

**Phase:** 21 - Backup and restore
**Status:** Designed, awaiting approval. No active implementation.

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

## 21.2 Library restore [DESIGNED]

Restore a user-chosen zip through staged extraction with install-grade traversal protection. Preview the zip file's date (labeled as such) with counts from a live scan of the staged contents; trust nothing in the zip. Replace the library wholesale with park+replace+rollback under the game-running lock, restoring `metadata.json` through the store's safe-write path. Report `Restored X mods (Y IoStore, Z Classic, W Invalid)` from a post-restore scan; invalid entries restore as-is and are counted, not dropped.

**Verify:**

1. Restore 21.1 backups and `metadata.json`-free zips into disposable libraries. Confirm byte-identical trees and matching reported counts; confirm metadata survives the round-trip and stays untouched when absent.
2. Test traversal/absolute-path/symlink zips (rejected pre-mutation), mid-restore failure and cancellation (previous library intact), restore while the game lock is held (blocked), and restore into a non-empty library (replacement warning shown, explicit confirmation required).
3. Run `check.ps1` and the frontend tests.
4. Stop for review. Do not start 21.3. (App-drive verification of restore moves to 21.3, which owns all menu wiring.)

## 21.3 Wire Backup / Restore into the Tools menu [DESIGNED]

Replace the separate backup placeholder cards with one "Backup / Restore" card holding side-by-side Backup and Restore buttons bound to 21.1 and 21.2. Keep the character-data and BentoMod placeholder cards untouched.

**Verify:**

1. Add frontend presentation tests for the wired card.
2. Run `check.ps1` and the frontend tests.
3. Drive the app against a disposable fixture library. Run a backup and a restore from the wired menu; screenshot the menu, progress, and success states.
4. Stop for review.
