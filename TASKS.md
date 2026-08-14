# Cratebug Active Tasks

**Phase:** 4 - Organization and recoverable deletion
**Status:** Not started

This file contains only the active phase. Replace it when Phase 4 is complete.

## Phase objective

Let users organize a discovered mod library safely: rename and reprioritize
bundles, move them between physical folders, manage nested folders, and delete
mods through the Windows Recycle Bin.

All mutation policy and filesystem operations belong in Go. React requests an
operation, displays its progress and result, and reconciles the affected catalog
state.

Every controlled operation must plan against the complete recognized bundle,
remain inside the selected mod root, and report its final filesystem state.

## Exit criteria

* Rename, priority, move, and deletion operations keep primaries and recognized
  sidecars synchronized.
* Priority behavior matches compatibility fixtures, including leading `!` and
  trailing runs of nines.
* Folder operations remain inside the selected mod root; the root cannot be
  renamed or deleted.
* Normal deletion uses the Recycle Bin and never silently becomes permanent.
* Destination collisions, invalid paths, and partial failures are reconciled
  and reported accurately.
* Controlled rename and move preserve the metadata identity mechanism selected
  for this phase.
* The UI reflects successful, rejected, busy, and recoverably deleted states.
* Running-app screenshots receive review approval.

## Out of scope

Phase 4 does not include:

* Batch operations
* Permanent deletion
* Archive installation
* Asset conflict inspection
* BentoMod import
* Tags, tag management, and general metadata persistence beyond preserving
  controlled-operation identity

## 4.1 Define bundle-aware organization operations

* Extend the Go-owned mutation boundary with narrow operations for mod rename,
  priority change, move, folder management, and recoverable deletion.
* Identify mods and folders from scanner-produced data rather than arbitrary
  frontend paths.
* Plan every mutation against the primary and all recognized sidecars before
  modifying the filesystem.
* Preserve or introduce only the minimum internal identity needed for controlled
  rename and move; defer general persistence and tags to Phase 5.
* Apply the existing game-running safety restriction to every unsafe mutation.
* Return specific, actionable errors and final reconciled state.

**Verify:** The application layer exposes only the required organization
operations and does not expose arbitrary filesystem access.

## 4.2 Implement bundle-aware rename and priority

* Rename a mod by applying the planned filename transformation to its primary
  and recognized sidecars as one logical bundle.
* Implement filename-based priority changes compatible with established
  BentoMod patterns, including a leading `!` and trailing runs of nines.
* Reject empty, invalid, ambiguous, colliding, or unsafe destinations before
  mutation. Never overwrite an existing file.
* Attempt rollback when a multi-file operation partially fails, then reconcile
  and report the actual state.
* Preserve disabled-format suffixes while changing the underlying mod name or
  priority.

**Verify:** Disposable fixtures cover classic and IoStore bundles, supported
priority forms, collision rejection, failure injection, rollback attempts,
unchanged unrelated files, and preserved controlled-operation identity.

## 4.3 Implement safe moves and nested folder management

* Move complete bundles between existing physical folders below the selected mod
  root.
* Create and rename nested folders, and move folders only when the planned
  source and destination remain inside the mod root.
* Reject root rename or deletion, traversal, absolute paths, collisions, moves
  into a descendant, and operations involving missing or invalid entries.
* Preserve bundle relationships and controlled-operation identity through moves.
* Reconcile after successful and partially failed operations.

**Verify:** Disposable fixtures cover nested folders, bundle moves, folder
rename and create behavior, unsafe path rejection, root protection, collision
handling, and recovery from injected failures.

## 4.4 Implement recoverable deletion

* Delete a scanner-recognized mod bundle through the Windows Recycle Bin using
  a reviewed platform boundary; do not implement a permanent-delete fallback.
* Require an explicit confirmation request; the destructive-action delay is a
  Task 4.5 UI safeguard, not backend timing policy.
* Permit incomplete IoStore bundles to delete their present recognized members;
  refuse ambiguous, missing-primary, outside-root, or game-running requests.
* Reconcile the catalog after success or failure and accurately report any
  remaining files.

**Verify:** Disposable tests prove that destructive operations never call a
permanent deletion path, rejected operations leave fixtures unchanged, and
partial or platform failures report the final state.

## 4.5 Add organization UI interactions

### 4.5.1 Clarify disabled mods

* Give intentionally disabled mods a clearly muted, non-error card treatment
  across every catalog view, with a visible Disabled badge and distinct state
  indicator.

### 4.5.2 Build mutation interaction foundations

* Add selected-mod state, a consistent action area, busy/request locking, and
  actionable mutation feedback without discarding the current library view.
* Visually verify selection, loading, duplicate-request prevention, and error
  states before adding individual mutation dialogs.

### 4.5.3 Add rename and priority controls

* Add focused rename and priority dialogs wired to the existing backend.
* Visually verify successful updates, invalid names, priority bounds, and busy
  states against a user-designated disposable library.

### 4.5.4 Add folder and move controls

* Add create, rename, and move-folder controls alongside mod move controls.
* Visually verify nested folders, collisions, and mod moves between folders
  against a user-designated disposable library.

### 4.5.5 Add recoverable deletion controls

* Add a destructive confirmation dialog with a short UI delay before sending a
  deletion request to the backend.
* Visually verify complete and incomplete bundle deletion against a
  user-designated disposable library.

### 4.5.6 Polish organization interaction resilience

* Verify empty, error, busy, and narrow-layout states; prevent conflicting
  actions and capture final interaction screenshots.

* Add focused controls and dialogs for rename, priority, move, folder
management, and deletion to the existing library UI.
* Require a clear destructive-deletion confirmation with a short delay before
  sending the backend request.
* Keep controls and confirmations consistent with discovered state, including
folder hierarchy and bundle type.
* Prevent duplicate and conflicting requests while a mutation is in progress.
* Show progress and actionable failures without freezing or discarding the
current library view.
* Reconcile only affected entries when possible; rescan when a folder-level
operation requires it.

**Verify:** The running application demonstrates successful, rejected, busy,
and confirmation states using a user-designated disposable library.

## 4.6 Validate organization safety and complete the review

* Run the canonical repository validation command and focused disposable
filesystem tests.
* Verify before-and-after filesystem state for successful, rejected, and
partially failed operations.
* Launch the application and exercise every operation against a
user-designated disposable library.
* Capture screenshots of rename, priority, move, folder, deletion confirmation,
  busy, and error states.
* Confirm no real Marvel Rivals mod files were modified during automated
  testing.
* Create `docs/reviews/phase-4-review.md` with validation results, screenshot
  paths, limitations, deferred findings, and review approval.

The review must record:

* Supported bundle, priority, and folder transitions
* Metadata-identity preservation behavior
* Recycle Bin implementation and confirmation behavior
* Collision, path-safety, game-running, rollback, and reconciliation behavior
* Commands and tests run
* Manual checks and screenshot paths
* Known limitations and deferred findings
* Review approval

**Verify:** Review approval grants permission to begin Phase 5.

## Phase 4 completion report

Report:

* What changed
* Files changed
* Validation and results
* Manual checks and screenshot paths
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 4 as one bounded implementation pass unless a material
design decision or explicit review gate is encountered. Stop before Phase 5.
