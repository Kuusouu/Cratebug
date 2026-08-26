# Cratebug Active Tasks

**Phase:** 8 - Installation and archive safety
**Status:** In Progress

This file contains only the active phase. Replace it when Phase 8 is complete.

## Phase objective

Give Cratebug the ability to safely install supported local mod archives and direct `.pak` files into the mod library. Cratebug must handle single and multiple mod installations, accepting input via UI drag-and-drop or an explicit "Install" button.

Users must be presented with an installation preview UI before changes are made. This preview will show data about each mod being installed, allow users to set a custom name for each (or use a default), and choose the destination folder within their library. The destination should default to their current navigation context in the UI (e.g., library root, or a specific subfolder).

Under the hood, all archive handling (zip, rar, 7z, etc.) must be unified using `github.com/mholt/archives`. Installation must be safe: it must use staged extraction, protect against path-traversals and unsafe links, validate bundles, handle collision decisions, and behave like a transaction (apply, cleanup, and reconcile) with progress and cancellation.

## Exit criteria

* The UI accepts file inputs via drag-and-drop and an "Install" button.
* Cratebug successfully unpacks archives using `github.com/mholt/archives` or processes direct `.pak` selections.
* An installation preview UI shows mod details, destination, and name choices prior to execution.
* The destination correctly defaults to the current library navigation context.
* Malicious archives (path traversal, unsafe links) are rejected safely.
* Existing mods are never silently overwritten without a collision decision.
* Failed or cancelled installations remove staging data and do not leave partial bundles.
* Multiple mod installation processes seamlessly and securely.
* Review covers success and major failure paths against representative classic and IoStore fixtures.

## Out of scope

Phase 8 does not include:

* Remote downloads and deep links
* Automatic updates
* Asset conflict inspection (Phase 9)
* Full backup and restore

## 8.1 Unify archive handling with `mholt/archives`

* Introduce `github.com/mholt/archives` as the single unified library for extracting and inspecting supported archives.
* Replace or build backend utilities for staged extraction that use this library, supporting common formats.
* Implement path-traversal and unsafe-link protection during extraction.

## 8.2 Build backend installation pipeline

* Implement backend endpoints to receive file paths (archives or individual `.pak` files).
* Implement staged extraction to a temporary directory before modifying the actual mod library.
* Implement bundle discovery and validation on the staged files (identifying `.pak`, sidecars, etc.).
* Implement collision detection with existing mods.
* Implement a transaction-like `apply` mechanism that moves validated, staged files to their final destination, cleans up staging areas, and reconciles the library state.
* Support progress reporting and cancellation logic that safely aborts without leaving partial state in the library.

## 8.3 Build the installation preview UI

* Create a new installation preview modal/screen that receives the pending installation jobs.
* Display relevant data about each mod being prepared.
* Provide inputs for the user to assign a specific name to the mod (defaulting to a sensible derived name).
* Provide a folder selection dropdown or path input for the destination. Default this to the user's current view context (e.g., if they are in `root/Characters/Hulk`, default the destination to that).
* Present warnings for any detected collisions and allow the user to decide how to proceed (e.g., overwrite, rename, cancel).

## 8.4 Implement Drag-and-Drop and Install triggers

* Wire the main application window to accept drag-and-drop events of external files.
* Add an "Install Mod" button in the library header/UI that opens a file picker capable of selecting multiple files/archives.
* Route both triggers to the installation preview UI built in 8.3.

## 8.5 Validate and review

* Run the canonical repository validation command (`check.ps1`).
* Launch the app and perform installations using various fixtures (zip, rar, bare `.pak`, multiple files).
* Verify that drag-and-drop triggers the flow.
* Confirm path traversal protections and cancellation cleanly aborts.
* Reference BentoMod in `C:\Users\mew\archive\BentoMod` to see how they handle installations for ideas, but do not treat it as the final truth.
* Create `docs/reviews/phase-8-review.md` covering all new workflows.

**Verify:** Review approval grants permission to begin Phase 9.

## Phase 8 completion report

Report:

* What changed
* Files changed
* Validation and results
* Known limitations
* Deferred findings
* Suggested commit message
