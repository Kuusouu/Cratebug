# Phase 4 Review

**Date:** 2026-08-21
**Status:** Approved

## Outcome

Phase 4 lets users organize a discovered mod library: rename mods and change
their filename-based priority, move mods and folders, create and rename
nested folders, and delete a mod bundle through the Windows Recycle Bin. All
filesystem policy lives in Go (`internal/mutation`); React requests one named
operation per action and reconciles the catalog from the returned result. The
UI settled on a right-click context menu as the primary way to reach organize
actions (see
[0002-organize-action-pattern](../decisions/0002-organize-action-pattern.md)),
with a small set of persistent controls reserved for state that has no single
target (New Folder) or that is used constantly (Enable/Disable).

## Supported bundle, priority, and folder transitions

- Rename a mod's primary and recognized sidecars as one bundle-aware
  operation; preserve `.pak_crateoff` / `.bak_bento` / `.pak_disabled`
  suffixes across a rename.
- Change filename-based priority using BentoMod-compatible forms (leading
  `!` for zero, trailing runs of nines for larger values).
- Move a complete mod bundle between existing physical folders below the mod
  root.
- Create a folder below the root or an existing folder; rename an existing
  folder; move a folder (and everything nested under it) below another
  existing folder or the root.
- Delete a scanner-recognized bundle through the Windows Recycle Bin.
  Deletion is the one operation that is intentionally permitted on an
  **incomplete** IoStore bundle (missing `.utoc` or `.ucas`): it sends
  whichever recognized members are actually present, since there is nothing
  unsafe about recycling files that exist. Rename, priority, and move all
  refuse an incomplete bundle instead, because those operations would leave
  a partially renamed/moved set of files.
- Every mutation is rejected while `Marvel-Win64-Shipping.exe` is running,
  enforced in Go before any filesystem change
  (`internal/mutation/executor.go`).

## Metadata-identity preservation behavior

Scanner-issued entry IDs are deterministic (folder + stem + kind), not
persisted metadata. A rename, priority change, or move changes an entry's
folder or stem and therefore its ID; the frontend tracks this explicitly by
reading `result.id` / `result.previousID` off each mutation response and
re-pointing the current selection at the new ID
(`updateMutatedEntry` in `frontend/src/library/LibraryScreen.tsx`). A folder
rename or move changes IDs for every entry it carries, not just one, so the
frontend does not attempt to remap the selection there: it clears the
selected mod and reconciles by rescanning
(`renameFolder`, `moveFolder` in the same file). This is a known, accepted
limitation carried from the scanner's identity model established in Phase 1,
not a new gap introduced this phase.

## Recycle Bin implementation and confirmation behavior

Deletion goes through a reviewed Windows Recycle Bin boundary
(`internal/mutation/deletion.go`); there is no permanent-delete fallback, and
`deleteModWithRecycle` refuses to proceed without `confirmed: true`. The UI
adds a second, independent safeguard on top of that backend requirement: a
3-second countdown gates the confirm button in `DeleteConfirmDialog`, showing
`Delete (N)` disabled until it reaches zero. The dialog also lists exactly
which files will be sent to the Recycle Bin, derived from the entry's actual
primary path and present sidecars, and shows an explicit warning when the
bundle is incomplete so the missing member is not mistaken for something the
operation should have removed.

## Collision, path-safety, game-running, rollback, and reconciliation behavior

These are Go-owned invariants from tasks 4.1-4.4, exercised by the automated
suite and re-verified manually this session:

- Destination collisions (mod or folder) are rejected before any file moves.
- Paths are confirmed to stay inside the configured mod root; traversal and
  absolute destinations are rejected.
- A folder cannot be moved into itself or one of its own descendants
  (`isDescendantFolder`); the move-destination dropdown in the folder dialog
  filters these out client-side as well, so the invalid choice is not even
  offered.
- Every unsafe mutation is blocked while the game process is running,
  checked in the executor before the operation runs.
- A multi-file rename or move attempts rollback on partial failure, then
  rescans and reports the reconciled state rather than claiming success.
- The frontend reconciles narrowly where it can (patching just the affected
  entry after a rename or priority change) and rescans fully where an
  operation's blast radius is not a single entry (any folder mutation, and
  mod moves, since folder membership changes what a scanner-derived ID
  encodes).

## Interaction resilience (task 4.5.6)

Auditing every `disabled=` gate in the library UI surfaced two real
conflicting-action gaps that the automated Go tests could not catch, since
they are pure frontend lock-scoping bugs:

- `FolderMutationDialog`'s busy state was wired to `isFolderMutating`
  instead of the combined `isMutationLocked`. A mod mutation running
  elsewhere left the folder dialog's Create/Rename/Move button enabled; a
  click during that window silently no-op'd against the backend's own guard
  instead of visibly waiting.
- `ModCatalog` derived its per-card lock (`mutatingEntryIDs.size > 0`)
  purely from mod mutations, and `setModEnabled`'s guard checked only
  `mutatingEntryIDsRef`, not the folder-mutation ref. A folder rename or
  move left every mod card's Enable/Disable button clickable, which could
  have raced a mod mutation against the folder mutation touching the same
  subtree.

Both were fixed by routing every lock check through the single
`isMutationLocked` value (`mutatingEntryIDs.size > 0 || isFolderMutating`).
Verified live by patching `window.go.main.App.RenameFolder` with an
artificial delay and confirming, via direct DOM inspection during the
pending call, that unrelated mod cards' action buttons report
`disabled: true`.

Also verified: an invalid scan path produces a readable error
(`Could not scan this folder`, with the underlying Go error) without
crashing or corrupting the visible catalog; the layout at 700px width
reflows to the single-column responsive breakpoint cleanly; the context
menu's viewport clamping and scroll-to-dismiss behavior both work as
designed.

## Commands and tests run

```powershell
.\check.ps1
```

This ran Go formatting, frontend format/lint/typecheck/build, `go vet`, and
the full Go test suite. All passed. The `go test ./...` run included three
pre-existing `SKIP` results for directory-symlink tests
(`TestDeletionPlanRejectsDirectoryLinkIntroducedAfterPlanning`,
`TestCreateFolderRejectsDirectoryLinkIntroducedAfterScanning`,
`TestBundlePlanRejectsDirectoryLinkIntroducedAfterPlanning`); this machine's
account lacks `SeCreateSymbolicLinkPrivilege`, which is an environment
limitation unrelated to Phase 4 changes.

No automated test targeted a real Marvel Rivals mod directory. Go tests use
`t.TempDir()` fixtures throughout.

## Manual checks

All manual verification used `C:\ModsTest`, a user-designated disposable
library, plus small synthetic fixtures under the session scratchpad for
cases `C:\ModsTest` did not already contain (an incomplete IoStore bundle
missing `.ucas`). No real Marvel Rivals mod directory was used.

Filesystem-level verification (not just UI claims):
- After a complete-bundle delete, confirmed via
  `Shell.Application`/`Namespace(10)` that all three files
  (`.pak`, `.utoc`, `.ucas`) were present in the actual Windows Recycle Bin.
- After an incomplete-bundle delete, confirmed only the two present files
  (`.pak`, `.utoc`) were recycled; no phantom `.ucas` entry appeared.
- Confirmed the deleted mod's files were gone from the source directory in
  both cases.

## Known limitations and deferred findings

- Folder rename/move clears the mod selection rather than remapping it,
  because the affected entries' IDs all change at once. This matches the
  scanner's existing identity model (Phase 1) and is not new to Phase 4.
- Right-click context menus are inherently mouse-first. Keyboard access
  works (browsers dispatch `contextmenu` for Shift+F10 / the Menu key on a
  focused element, verified live in the running app), but there is no
  visible on-screen affordance telling a keyboard-only user that shortcut
  exists.
- Batch operations, permanent deletion, archive installation, asset conflict
  inspection, BentoMod import, and general tag/metadata persistence remain
  out of scope for this phase, per TASKS.md.

## Review approval

**Decision:** Approved by user on 2026-08-21.
**Notes:** Phase 4 is complete. Phase 5 may begin after its active task plan
is established.
