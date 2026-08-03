# Cratebug Active Tasks

**Phase:** 2 - Read-only library UI
**Status:** Not started

This file contains only the active phase. Replace it when Phase 2 is complete.

## Phase objective

Build a usable read-only interface for browsing the catalog produced by the
Phase 1 scanner.

The UI must remain independent of filesystem mutations and preserve the
scanner's read-only behavior.

Use the existing BentoMod populated library screen as the primary structural reference where applicable, while refactoring rather than blindly copying its component structure.

## Exit criteria

* Synthetic and real read-only libraries render correctly.
* Search and folder selection work without unnecessary rescans.
* Scanning does not freeze the interface.
* Migrated components are understandable and refactored.
* Running-app screenshots receive review approval.

## Out of scope

Phase 2 does not include:

* File mutations
* Enable or disable operations
* Rename, move, priority changes, or deletion
* Tags or metadata persistence
* Archive installation
* Asset conflict inspection
* Large visual redesigns

## 2.1 Define the UI boundary

* Expose the Phase 1 library result through a narrow typed application binding.
* Keep scanning and filesystem operations in Go.
* Keep React components focused on rendering and user interaction.
* Represent loading, populated, empty, and error states explicitly.
* Do not introduce mutation controls or future-phase models.

**Verify:** The binding exposes only the read-only data and operations needed by
the initial library view.

## 2.2 Migrate the initial library structure

* Selectively migrate the useful BentoMod information architecture.
* Add a header and library navigation structure.
* Add search and physical folder navigation.
* Add a mod list or grid showing name, enabled state, priority, and bundle type.
* Preserve nested folder information from discovery results.
* Refactor migrated components into understandable boundaries.
* Use the existing BentoMod populated library screen as the primary structural reference where applicable, while refactoring rather than blindly copying its component structure.

**Verify:** The UI renders a synthetic discovered library with nested folders,
disabled entries, incomplete bundles, and orphan diagnostics visible.

## 2.3 Implement read-only interaction states

* Add an explicit refresh action.
* Keep search and folder selection local to the displayed catalog.
* Avoid rescanning for filtering or navigation changes.
* Report scan errors without hiding the rest of the application shell.
* Ensure long-running scans do not freeze the interface.

**Verify:** Search, folder selection, refresh, loading, empty, populated, and
error states behave correctly with disposable fixtures.

## 2.4 Add appearance and accessibility foundations

* Support light, dark, and system appearance modes.
* Preserve the playful identity without reducing readability or usability.
* Support keyboard navigation for the initial library controls.
* Check common Windows scaling and viewport sizes.
* Keep typography, spacing, contrast, and overflow understandable at the
  standard review viewport.

**Verify:** Appearance modes and keyboard navigation work in the running
application without clipping or inaccessible controls.

## 2.5 Validate and complete the review

* Run the canonical repository validation command.
* Launch the application and navigate through the affected states.
* Capture application-window screenshots for the initial library states.
* Compare screenshots with the available reference direction.
* Fix visible layout, clipping, spacing, typography, color, and state issues.
* Create `docs/reviews/phase-2-review.md` with validation results, screenshot
  paths, limitations, and deferred findings.
* Use the existing BentoMod populated library screen as the primary structural reference where applicable, while refactoring rather than blindly copying its component structure.

The review should record:

* UI states and fixture coverage
* Binding and scanning behavior
* Search and folder-navigation behavior
* Appearance and accessibility checks
* Commands and tests run
* Screenshot paths
* Known limitations
* Deferred questions
* Review approval

**Verify:** Review approval grants permission to begin Phase 3.

## Phase 2 completion report

For each task, report:

* What changed
* Files changed
* Validation and results
* Manual checks and screenshot paths
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 2 as one bounded implementation pass unless a material design decision or explicit review gate is encountered. Stop before Phase 3.
