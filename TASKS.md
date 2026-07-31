# Cratebug Active Tasks

**Phase:** 1 - Read-only mod discovery
**Status:** Not started

This file contains only the active phase. Replace it when Phase 1 is complete.

## Phase objective

Build a deterministic, read-only Go scanner that can describe a Marvel Rivals mod library without changing any files.

The scanner must remain independent of Wails and React.

## Exit criteria

* Recursive scans recognize supported primary and sidecar formats.
* Classic, IoStore, disabled, incomplete, and orphaned entries are represented clearly.
* Nested physical folders are supported.
* Filename-based priority is parsed using verified compatibility fixtures.
* Repeated scans of the same directory return equivalent results.
* Scanning never modifies files.
* Automated tests use temporary directories and synthetic fixtures.
* Review approves the discovered behavior and terminology.

## Out of scope

Phase 1 does not include:

* Wails bindings or library UI
* File mutations
* Enable or disable operations
* Rename, move, priority changes, or deletion
* Settings or metadata persistence
* Filesystem watching
* UAssetToolRivals integration
* Archive installation
* Asset conflict inspection

## 1.1 Establish discovery fixtures

* Create synthetic fixtures representing known BentoMod-compatible layouts.
* Include:

  * Enabled `.pak`
  * `.pak_crateoff`
  * `.bak_bento`
  * `.pak_disabled`
  * Classic `.pak` without sidecars
  * `.pak`, `.utoc`, and `.ucas` IoStore bundle
  * Nested folders
  * Leading `!` priority
  * Trailing-nine priority patterns
  * Unrecognized priority names
  * Partial sidecar combinations
  * Orphaned `.utoc` and `.ucas`
  * Same stems in different folders
  * Enabled and disabled primaries with the same stem
* Record which expectations come from confirmed BentoMod behavior and which remain Cratebug decisions.
* Do not include copyrighted game or mod files.

**Verify:** Fixture contents are small, readable, deterministic, and contain no real user data.

## 1.2 Define the minimal scan result

* Define only the data needed to describe a read-only library.
* Represent:

  * Primary path
  * Relative folder path
  * Display or clean name
  * Enabled or disabled state
  * Disabled format
  * Classic or IoStore classification
  * Recognized sidecars
  * Parsed priority
  * Incomplete or unusual status
  * Read-only diagnostics
* Keep paths and bundle information independent of Wails.
* Do not add persistence IDs, tags, mutation plans, UI models, or future installation fields.
* Document any naming or classification decision that is not already settled by `SPEC.md`.

**Verify:** Types are sufficient for all Phase 1 fixtures without containing later-phase concerns.

## 1.3 Implement recursive file discovery

* Scan a supplied mod-root path recursively.
* Return a clear error when the root is missing, inaccessible, or not a directory.
* Discover only files relevant to Phase 1 classification.
* Preserve physical nested-folder information.
* Ignore unrelated files without failing the scan.
* Ensure discovery performs no writes, renames, directory creation, or metadata changes.
* Use deterministic ordering for returned files and folders.

**Verify:** Temporary-directory tests cover empty, nested, missing, inaccessible where practical, and mixed-content roots.

## 1.4 Group primary files and sidecars

* Recognize supported primary formats:

  * `.pak`
  * `.pak_crateoff`
  * `.bak_bento`
  * `.pak_disabled`
* Associate same-stem `.utoc` and `.ucas` sidecars within the same physical folder.
* Classify complete classic and IoStore bundles.
* Preserve separate entries for same-named mods in different folders.
* Do not add `.sig` bundle handling.
* Do not silently merge ambiguous primary files that share a stem.

**Verify:** Table-driven tests cover every supported primary form and representative sidecar combinations.

## 1.5 Parse state, names, and priority

* Determine enabled or disabled state from the primary filename.
* Preserve which disabled format was discovered.
* Parse verified BentoMod-compatible priority patterns.
* Support leading `!` and trailing runs of nines.
* Keep ambiguous or unrecognized filenames visible.
* Avoid using cleaned filenames as persistent mod identity.
* Do not normalize away information required to display or diagnose the original filename.

**Verify:** Compatibility tests cover normal, ambiguous, malformed, and collision-prone filename cases.

## 1.6 Report incomplete and orphaned entries

* Represent partial sidecar combinations without crashing or hiding files.
* Report `.utoc` or `.ucas` files that have no supported primary.
* Distinguish:

  * Complete classic mod
  * Complete IoStore mod
  * Incomplete bundle
  * Orphaned sidecar
  * Ambiguous primary grouping
* Keep diagnostics descriptive and read-only.
* Do not decide whether incomplete bundles may be mutated; that belongs to later phases after further investigation.

**Verify:** Tests demonstrate that every relevant discovered file appears either in a bundle or in a diagnostic result.

## 1.7 Validate rescanning and complete the review

* Test repeated scans of an unchanged temporary library.
* Modify disposable fixture directories between scans and confirm new results reflect the filesystem.
* Confirm the scanner does not require cached state to produce correct results.
* Run the canonical repository validation command.
* Optionally perform a read-only scan of a real mod directory only with explicit user permission.
* Document findings that affect later phases without implementing them.
* Create `docs/reviews/phase-1-review.md`.

The review should record:

* Fixture coverage
* Classification rules
* Priority parsing behavior
* Incomplete and orphaned behavior
* Commands and tests run
* Any real-library read-only observations
* Known limitations
* Deferred questions
* Review approval

**Verify:** Review approval grants permission to begin Phase 2.

## Phase 1 completion report

For each task, report:

* What changed
* Files changed
* Validation and results
* Manual checks performed
* Known limitations
* Deferred findings
* Suggested commit message

Do not begin the next task or phase automatically.
