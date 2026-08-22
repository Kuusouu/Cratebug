# Cratebug Active Tasks

**Phase:** 5 - Metadata and persistence
**Status:** Active. Create `docs/reviews/phase-5-review.md` and update this
header when the phase is approved.

This file contains only the active phase. Replace it when Phase 5 is complete.

## Phase objective

Let settings, tags, and mod metadata survive safely across sessions and
across the controlled operations introduced in Phase 4 (rename, priority
change, move). Persistence is Go-owned and versioned; React requests loads
and saves through a typed API and never touches the storage file directly.

Phase 4 deliberately deferred general metadata persistence and used only a
scanner-derived, non-persistent identity (folder + stem + kind) that changes
whenever a mod is renamed or moved. Phase 5 introduces the stable identity
that tags and other metadata need to survive those same operations.

## Exit criteria

* Same-named mods in different folders can hold separate metadata.
* Tags survive controlled rename and move.
* Corrupt or unreadable metadata does not destroy or discard the rest of the
  library.
* A failed write preserves the last known-good state.
* Schema migrations are tested across supported prior versions.
* Review approves stored data format and recovery behavior.

## Out of scope

Phase 5 does not include:

* BentoMod import
* Cloud sync
* Perfect reconciliation of metadata after an external (non-Cratebug) rename
* Batch operations, permanent deletion, archive installation, asset conflict
  inspection (unchanged from Phase 4's exclusions)

## 5.1 Define the persistent metadata storage boundary

* Choose a storage location and a versioned schema envelope (explicit schema
  version field) for app settings, tags, and mod metadata.
* Implement safe writes: write to a temporary file and rename into place, and
  retain a last-known-good backup that a failed or interrupted write cannot
  corrupt.
* Keep all storage access in Go; expose only narrow typed load/save
  operations to the frontend, matching the mutation boundary's existing
  pattern of not exposing arbitrary filesystem access.

**Verify:** Disposable fixtures prove a round-trip write/read is exact, and
that a write interrupted partway (failure injection) leaves the prior valid
file intact and readable.

## 5.2 Implement persistent internal mod identity

* Introduce a mod identity that stays stable across rename, priority change,
  and move, independent of the scanner's deterministic folder+stem+kind ID
  used through Phase 4.
* Reconcile this identity using the `id` / `previousID` values Phase 4's
  mutation operations already return, so a rename or move re-points existing
  metadata at the mod's new location instead of orphaning it.
* Support same-named mods in different folders as distinct identities.
* Define what makes a persisted identity orphaned (no matching scanned mod)
  for 5.4 to detect and surface.

**Verify:** Disposable fixtures prove identity and its attached metadata
survive rename, priority change, and move; same-named mods in separate
folders keep independent metadata.

## 5.3 Implement tags and settings persistence

* Add a tag catalog (create, rename, delete) and per-mod tag assignment keyed
  to the identity from 5.2, not to the current filename or path.
* Persist app settings needed across sessions, starting with the selected mod
  root path, so the library does not require reselecting it every launch.
* Route all writes through the safe-write path from 5.1.

**Verify:** Disposable fixtures and manual testing confirm tags and the
selected mod root survive an app restart, and that tag assignment follows a
mod through rename and move rather than resetting.

## 5.4 Handle corrupt and orphaned metadata

* Validate loaded metadata against the versioned schema on load; quarantine
  (do not silently discard) a corrupt, truncated, or unreadable file and fall
  back to the last-known-good backup from 5.1 instead of crashing or
  resetting the library.
* Detect metadata whose identity no longer matches any scanned mod and
  surface it as orphaned rather than deleting it, in case the mod reappears
  (for example, after a folder is reselected).

**Verify:** Disposable fixtures cover a corrupted metadata file, a truncated
write, an unknown/future schema version, and orphaned entries. None of these
crash the app, block startup, or discard unrelated valid metadata.

## 5.5 Add schema migration support

* Define the migration mechanism from one schema version to the next and
  apply it automatically on load.
* Reject an unsupported future schema version safely rather than applying it
  partially or corrupting it.

**Verify:** Fixture files at each supported prior schema version migrate to
the current version and round-trip correctly; an unsupported future version
is rejected without modifying the file on disk.

## 5.6 Add tag and settings UI

* Add UI for creating tags, assigning and removing tags on a mod, and viewing
  the persisted mod root on launch instead of requiring reselection.
* Follow the existing context-menu organize pattern
  (`frontend/src/library/ContextMenu.tsx`,
  [0002-organize-action-pattern](docs/decisions/0002-organize-action-pattern.md))
  for how tag actions are reached, rather than inventing a second pattern.
* Show corrupt-metadata and recovery states from 5.4 in the UI as actionable,
  non-crashing feedback.

**Verify:** Visually verify tag creation, assignment, removal, persistence
across an app restart, and corrupt/recovery messaging against a
user-designated disposable library.

## 5.7 Validate persistence safety and complete the review

* Run the canonical repository validation command and focused disposable
  persistence tests.
* Verify before-and-after storage state for successful, failed, corrupt, and
  migrated cases.
* Launch the application and exercise tag and settings persistence across a
  restart against a user-designated disposable library.
* Capture screenshots of tag creation/assignment, persisted settings on
  relaunch, and corrupt/recovery states.
* Confirm no real Marvel Rivals mod files or metadata were used during
  automated testing.
* Create `docs/reviews/phase-5-review.md` with validation results, screenshot
  paths, limitations, deferred findings, and review approval.

The review must record:

* Storage format, schema version, and safe-write/recovery behavior
* How persistent mod identity is derived and reconciled through rename,
  priority, and move
* Tag and settings behavior, including cross-restart persistence
* Corrupt-data, orphaned-metadata, and migration behavior
* Commands and tests run
* Manual checks and screenshot paths
* Known limitations and deferred findings
* Review approval

**Verify:** Review approval grants permission to begin Phase 6.

## Phase 5 completion report

Report:

* What changed
* Files changed
* Validation and results
* Manual checks and screenshot paths
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 5 as one bounded implementation pass unless a material
design decision or explicit review gate is encountered. Stop before Phase 6.
