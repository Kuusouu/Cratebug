# Cratebug Active Tasks

**Phase:** 3 - Safe enable and disable
**Status:** Not started

This file contains only the active phase. Replace it when Phase 3 is complete.

## Phase objective

Add the first safe filesystem mutation to Cratebug: enabling and disabling discovered mods.

Cratebug must write its native `.pak_crateoff` disabled format while remaining compatible with legacy `.bak_bento` and `.pak_disabled` files.

All mutation policy and filesystem operations belong in Go. React only requests an operation and displays its result.

## Exit criteria

* Enabled mods can be disabled safely using `.pak_crateoff`.
* Cratebug-disabled and supported legacy-disabled mods can be enabled.
* `.utoc` and `.ucas` sidecars remain unchanged.
* Destination collisions and invalid states fail without modifying files.
* Game-running safety is enforced by the backend.
* Mutation behavior is covered by disposable filesystem tests.
* The UI reflects successful and failed operations correctly.
* Running-app screenshots receive review approval.

## Out of scope

Phase 3 does not include:

* Priority changes
* General rename or move operations
* Folder mutations
* Deletion
* Tags or metadata persistence
* Archive installation
* Asset conflict inspection
* Batch enable or disable
* Permanent migration of legacy disabled filenames

## 3.1 Define the mutation boundary

* Add a narrow Go operation for enabling or disabling a discovered mod.
* Identify mods using scanner-produced data rather than arbitrary frontend paths.
* Validate the requested transition against the current filesystem state.
* Keep filesystem mutation details out of Wails bindings and React components.
* Return specific, actionable errors for rejected operations.

**Verify:** The application layer exposes only the enable/disable operation needed by the UI and does not expose arbitrary rename functionality.

## 3.2 Implement safe disable behavior

* Disable an enabled `.pak` by renaming only the primary file to `.pak_crateoff`.
* Leave `.utoc` and `.ucas` sidecars unchanged.
* Refuse the operation if the expected source is missing or the destination already exists.
* Do not overwrite or remove existing files.
* Preserve the original filename apart from the disabled suffix.

Example:

`Example_9999999_P.pak` -> `Example_9999999_P.pak_crateoff`

**Verify:** Disposable fixtures prove successful disable behavior, unchanged sidecars, collision handling, and no unintended filesystem changes.

## 3.3 Implement safe enable behavior

* Enable `.pak_crateoff` by restoring the primary `.pak` filename.
* Support enabling legacy `.bak_bento` and `.pak_disabled` files.
* Legacy formats are read-compatible only; future disables use `.pak_crateoff`.
* Refuse ambiguous or colliding destinations rather than choosing or overwriting automatically.
* Leave `.utoc` and `.ucas` sidecars unchanged.

**Verify:** Tests cover all supported disabled formats, destination collisions, missing files, and unchanged sidecars.

## 3.4 Enforce game-running safety

* Detect whether Marvel Rivals is running using the established executable identity.
* Enforce the restriction in Go before performing a mutation.
* Do not rely on disabled UI controls as the safety boundary.
* Return a clear error when an operation is blocked because the game is running.

**Verify:** Backend tests prove the mutation is rejected before filesystem changes occur when game-running protection is active.

## 3.5 Add the UI interaction

* Add enable/disable controls to the existing library UI.
* Keep the control state consistent with the discovered mod state.
* Show operation progress without freezing the interface.
* Prevent accidental duplicate requests while an operation is in progress.
* Surface failures without discarding the current library view.
* Refresh or update the catalog after a successful mutation so the displayed state matches disk.

**Verify:** Enabled, Cratebug-disabled, and legacy-disabled entries transition correctly in the running application, and failures remain understandable.

## 3.6 Validate mutation safety and complete the review

* Run the canonical repository validation command.
* Run focused filesystem mutation tests against disposable fixtures.
* Verify before-and-after filesystem state for successful and rejected operations.
* Launch the application and exercise enable/disable behavior using disposable test data.
* Capture screenshots of the relevant enabled, disabled, busy, and error states.
* Confirm no real Marvel Rivals mod files were modified during automated testing.
* Create `docs/reviews/phase-3-review.md` with validation results, screenshot paths, limitations, and deferred findings.

The review should record:

* Supported state transitions
* Legacy disabled-format behavior
* Collision and invalid-state behavior
* Sidecar preservation
* Game-running protection
* Commands and tests run
* Manual checks and screenshot paths
* Known limitations
* Deferred findings
* Review approval

**Verify:** Review approval grants permission to begin Phase 4.

## Phase 3 completion report

Report:

* What changed
* Files changed
* Validation and results
* Manual checks and screenshot paths
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 3 as one bounded implementation pass unless a material design decision or explicit review gate is encountered. Stop before Phase 4.
