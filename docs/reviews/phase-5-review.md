# Phase 5 Review

**Date:** 2026-08-22
**Status:** Awaiting review

## Outcome

Phase 5 lets Cratebug persist settings, tags, and mod identity across sessions,
and keeps that persisted state safe against interrupted writes, corrupted
files, and schema drift. A new `internal/metadata` package owns the storage
format, safe writes, corrupt-file recovery, schema migration, and the
persistent mod identity that tags attach to; `app.go` wires it into the
existing Wails boundary and into the Phase 4 mutation flow so a rename,
priority change, or move re-points a mod's tags instead of orphaning them.
The frontend gained a "Tags..." context-menu action (extending the pattern
from
[0002-organize-action-pattern](../decisions/0002-organize-action-pattern.md))
and restores the last-used mod root automatically on launch.

## Storage format, schema version, and safe-write/recovery behavior

- One JSON document (`metadata.Document`) is persisted at
  `%AppData%\Cratebug\metadata.json`, carrying an explicit `schemaVersion`
  field (`internal/metadata/store.go`).
- `Store.Save` writes atomically: content goes to a temporary file in the
  same directory first, then `os.Rename` replaces the primary in one step, so
  an interrupted write never leaves the primary partially written. Before
  replacing the primary, `Save` copies its current contents to a
  `.bak` last-known-good backup.
- `Store.Load` cannot fail. A missing primary returns a fresh document. A
  primary that is unreadable, fails to parse, or declares a schema version
  newer than this build supports is quarantined (renamed to `.corrupt`,
  content untouched) rather than discarded, and `Load` falls back to `.bak`.
  If the backup is unusable too, `Load` falls back to a fresh document. A
  successful recovery is written back to the primary path so later loads do
  not repeat the recovery. `Load` returns a `Recovery{Recovered, Cause}`
  value alongside the document so callers can surface what happened.

## How persistent mod identity is derived and reconciled

- The scanner's entry ID (folder + stem + kind) changes whenever a mod is
  renamed, reprioritized, or moved, so it cannot anchor persisted metadata by
  itself. `internal/metadata/identity.go` adds a second, opaque identity
  (`mod-<hex>`) stored in `Document.Mods`, keyed independently of the current
  scanner ID. `EnsureMod` creates or looks up that identity from a scanner ID;
  same-named mods in different folders get distinct scanner IDs and therefore
  distinct persistent identities.
- `App.executeAndReconcile` (`app.go`) wraps `RenameMod`, `SetModPriority`,
  and `MoveMod`: after a successful mutation, it loads the metadata document,
  calls `ReconcileMod(result.PreviousID, result.ID)` to re-point the mod
  record at its new scanner ID, and saves. `SetModEnabled` does not need this
  wrapping because enabling or disabling a primary does not change its
  scanner ID.
- **Known limitation:** `RenameFolder` and `MoveFolder` are not wrapped, because
  their `Result` reports only the folder's own old and new paths, not a
  per-mod ID pair for every mod the folder carries. Tags on mods inside a
  renamed or moved folder are not reconciled. This mirrors the equivalent,
  already-accepted frontend limitation from
  [phase-4-review.md](phase-4-review.md#known-limitations-and-deferred-findings)
  (folder rename/move clears rather than remaps the selection) and was not
  solved here because Phase 4's `Result` type would need to change to close
  it, which is out of this task's scope.

## Tag and settings behavior, including cross-restart persistence

- `internal/metadata/tags.go` implements a tag catalog (`CreateTag`,
  `RenameTag`, `DeleteTag`, case-insensitive duplicate-name rejection) and
  per-mod assignment (`AssignTag`, `UnassignTag`, both idempotent) keyed to
  the persistent mod identity, not the current filename.
- `App` exposes `LoadMetadata`, `SetModRoot`, `CreateTag`, `RenameTag`,
  `DeleteTag`, `AssignModTag`, and `UnassignModTag`. The frontend
  (`LibraryScreen.tsx`) loads persisted metadata once on launch, restores the
  saved mod root and scans it automatically instead of requiring
  reselection, and persists the mod root again after every successful scan.
- Tags are reachable from the mod context menu's new "Tags..." item, which
  opens a checklist dialog (`ModTagDialog`) that assigns or unassigns a tag
  immediately per checkbox toggle, plus an inline "New tag" field that
  creates and assigns in one action. Assigned tags are shown on the selected
  mod's status panel.
- Manually verified end to end against a disposable fixture library
  (see Manual checks): mod root and tag assignment both survived a full
  reload (a `wails dev` browser-tab reload restarts only the frontend, not
  the long-lived Go backend, so this specifically confirms the settings and
  tags round-tripped through the real on-disk file, not an in-memory cache;
  the raw file content on disk was inspected directly and matched
  expectations before and after).

## Corrupt-data, orphaned-metadata, and migration behavior

- `Document.OrphanedMods(liveScannerIDs)` (`internal/metadata/identity.go`)
  reports mod records whose scanner ID is absent from a fresh scan, without
  removing them, so metadata is not lost if the mod reappears (for example,
  after reselecting a folder). This task did not wire a UI or App binding
  for it; see Deferred findings.
- Schema migration (`internal/metadata/migrations.go`) reads the document as
  a raw JSON map first, applies registered `migrationStep`s in sequence until
  the version reaches `CurrentSchemaVersion`, and only then decodes into
  `Document`. A migration step that fails to advance the version is treated
  as an error (guards against an infinite loop from a future migration bug).
  One migration is registered: schema version 0 (no `schemaVersion` field at
  all, e.g. a hand-edited or pre-existing file) stamps version 1; no other
  migration exists yet because version 1 is the only format Cratebug has
  ever written.
- Live-verified: corrupting the real running app's `metadata.json` to
  invalid JSON and reloading did not crash the app or block it from starting.
  It scanned the library normally, quarantined the corrupt file to
  `metadata.json.corrupt` (confirmed byte-identical to the corrupted
  content), restored `metadata.json` from `metadata.json.bak`, and surfaced
  a warning toast identifying the cause
  ("Cratebug found a problem with your saved settings and recovered them
  from a backup: parse metadata file ... invalid character 'n' ...").

## Commands and tests run

```powershell
.\check.ps1
```

This ran Go formatting, frontend format/lint/typecheck/build, `go vet`, and
the full Go test suite: 77 tests passed across `internal/metadata`,
`internal/discovery`, `internal/mutation`, and the root `main` package. Three
pre-existing `SKIP` results remain for directory-symlink tests, unrelated to
Phase 5 (documented in
[phase-4-review.md](phase-4-review.md#commands-and-tests-run); this
machine's account still lacks `SeCreateSymbolicLinkPrivilege`).

`internal/metadata` alone contributes 30 of those tests, covering: atomic
writes and backup maintenance (`store_test.go`), persistent identity and its
survival through real `mutation` package rename/priority/move calls
(`identity_test.go`), the tag catalog and assignment including a real
rename-and-move sequence (`tags_test.go`), corrupted/truncated/unsupported
files, orphan detection, and recovery not discarding unrelated valid data
(`recovery_test.go`), and schema migration including the unversioned case
and a rejected pre-migration version (`migrations_test.go`).

A real bug surfaced only through this test suite and was fixed before
merging: `ReconcileMod` originally replaced a mod's whole record on
reconciliation, silently discarding its `Tags`; `schemaVersionOf` originally
recognized only `float64` (encoding/json's numeric type) and misread a
migration's own freshly-set `int`, tripping the anti-infinite-loop guard on
every migration.

## Manual checks and screenshot paths

All manual verification used a disposable fixture library created under the
session scratchpad directory (two synthetic `.pak` files, one nested), never
a real Marvel Rivals mod directory. It and the app's own dev-mode
`metadata.json`/`.bak`/`.corrupt` files were deleted after verification.

Performed via `mise exec -c "wails dev"`, driving the app at
`http://localhost:34115` and inspecting the DOM (accessibility tree and text
content) plus the real on-disk metadata file, rather than pixel screenshots
(see Known limitations):

- Scanned the fixture library; selected a mod; opened its context menu and
  confirmed a "Tags..." item is present alongside Rename/Priority/Move/Delete.
- Opened the tag dialog on an untagged mod: confirmed the "No tags yet.
  Create one below." empty state.
- Created a tag ("Combat") via the dialog's inline field: confirmed the
  success toast, the new checked checkbox in the dialog, and the tag chip
  appearing on the selected-mod panel.
- Unchecked and rechecked the tag's checkbox: confirmed the corresponding
  "Removed"/toast and chip-disappears / chip-reappears behavior.
- Reloaded the page (restarting only the frontend, not the long-lived Go
  backend): confirmed the mod root field auto-filled, the library
  auto-scanned, and the "Combat" chip was still present on the same mod,
  reading from the real persisted file rather than in-memory state.
- Corrupted the real `metadata.json` to invalid JSON and reloaded: confirmed
  no crash, a normal scan, and the recovery warning toast described above;
  confirmed on disk that `metadata.json` was restored, `metadata.json.bak`
  was intact, and `metadata.json.corrupt` held the exact original corrupted
  bytes.
- Renamed the tagged mod, then changed its priority, then moved it to a
  different folder (three separate operations): confirmed the "Combat" chip
  remained visible immediately after each one, with no page reload. The
  first attempt at this surfaced the frontend staleness bug described above
  (the chip disappeared after rename because the frontend's cached metadata
  was not refreshed); after the fix, all three operations kept the chip
  visible without a reload.

## Known limitations and deferred findings

- **No pixel screenshots.** The available browser tooling in this session
  could not composite frames (`the Browser pane is not displayed, so the
  page is not compositing frames`), and the repository's configured
  `playwright` MCP server was not connected in this session. All UI
  verification above is DOM-level (accessibility tree, text content) and
  on-disk state, not visual. Layout, spacing, color, and typography for the
  new tag dialog and chips were not visually confirmed, only functionally
  confirmed to render the expected content. The new CSS reuses established
  classes and patterns (`.mutation-dialog`, `.mod-facts`-style pill chips)
  rather than introducing new layout, which lowers but does not eliminate
  the risk.
- Folder rename/move does not reconcile tags for the mods it carries (see
  above); this is an accepted extension of Phase 4's existing
  folder-selection limitation, not a new gap.
- `Document.OrphanedMods` has no App binding or UI surface yet; task 5.4
  built the detection capability and task 5.6 did not add a UI for it, since
  neither task's scope explicitly required one. A future task should decide
  how orphaned metadata is surfaced (for example, when a folder is
  reselected and previously-tagged mods are missing).
- No UI exists for renaming or deleting a tag from the catalog
  (`RenameTag`/`DeleteTag` exist on the backend and are tested, but
  TASKS.md's 5.6 scope named only creating, assigning, and removing tags on
  a mod).
- Batch operations, permanent deletion, archive installation, asset conflict
  inspection, BentoMod import, and general settings beyond the mod root
  remain out of scope for this phase, per TASKS.md.

## Review approval

**Decision:** Pending user review.
