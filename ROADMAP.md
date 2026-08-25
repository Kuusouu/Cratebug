# Cratebug Roadmap

**Status:** Draft v0.1

This roadmap defines implementation order. Detailed work belongs in `TASKS.md`, which contains only the active phase.

## Working rules

- Each phase delivers one coherent capability.
- Later-phase work does not enter early phases "just in case."
- Automated evidence and project review are both required where appropriate.
- A phase ends in a working, reviewable repository state.
- Compilation alone is not completion.

## Phase 0 - Repository and toolchain

**Outcome:** A fresh, reproducible Cratebug repository that develops, builds, installs, launches, and uninstalls on Windows.

**Includes:**

- Go, Wails v2, React, TypeScript, Vite 8, Bun, and Biome
- Pinned stable versions
- Minimal Wails application shell
- Canonical validation commands
- CI
- Production Windows build
- NSIS installer and uninstaller
- Setup documentation

**Excludes:** Mod logic, BentoMod UI migration, persistence, and UAssetToolRivals.

**Exit criteria:**

- Development app launches.
- All Go and frontend checks pass.
- Production build succeeds.
- Installer installs, launches, and uninstalls.
- A clean checkout reproduces the workflow.
- Review approves the foundation.

## Phase 1 - Read-only mod discovery

**Outcome:** Cratebug can scan and describe a mod library without changing files.

**Includes:**

- Recursive folder discovery
- `.pak`, `.pak_crateoff`, `.bak_bento`, and `.pak_disabled`
- `.utoc` and `.ucas` association
- Classic and IoStore classification
- Priority parsing
- Incomplete and orphaned file reporting
- Synthetic fixtures and temporary-directory tests
- Manual refresh

**Excludes:** Mutations, metadata, installation, file watching, and UAssetToolRivals inspection.

**Exit criteria:**

- Representative libraries are classified deterministically.
- Scanning never mutates files.
- Nested folders and legacy disabled forms work.
- Unusual bundles remain visible instead of crashing.
- Scanner code is independent of Wails and React.
- Review approves terminology and fixtures.

## Phase 2 - Read-only library UI

**Outcome:** Users can browse the discovered catalog in a usable Cratebug interface.

**Includes:**

- Selective BentoMod frontend migration
- Header, search, folder navigation, and mod list or grid
- Enabled state, priority, and bundle type
- Empty, loading, populated, and error states
- Light, dark, and system themes
- Initial screenshot workflow

**Excludes:** Mutations, tags, installation, conflicts, and large visual redesigns.

**Exit criteria:**

- Synthetic and real read-only libraries render correctly.
- Search and folder selection work without unnecessary rescans.
- Scanning does not freeze the interface.
- Migrated components are understandable and refactored.
- Running-app screenshots receive review approval.

## Phase 3 - Safe enable and disable

**Outcome:** Cratebug can safely toggle mods using `.pak_crateoff`.

**Includes:**

- Operation planning and path validation
- Collision checks
- Game-running detection enforced in Go
- Disable `.pak` to `.pak_crateoff`
- Enable `.pak_crateoff`, `.bak_bento`, and `.pak_disabled`
- Rollback attempts and post-operation reconciliation
- Failure-injection tests

**Excludes:** Rename, priority, folders, batches, installation, and advanced safety overrides.

**Exit criteria:**

- Expected filename changes are verified with disposable fixtures.
- Sidecars remain associated without being renamed.
- Unsafe paths and collisions fail before mutation.
- Failures never produce false success.
- Final filesystem state is reported accurately.
- Review approves success and failure flows.

## Phase 4 - Organization and recoverable deletion

**Outcome:** Users can rename, prioritize, move, organize, and recoverably delete mods.

**Includes:**

- Rename and filename-based priority
- Move mods between physical folders
- Create, rename, and organize nested folders
- Bundle-aware mutation plans
- Metadata-preserving controlled moves
- Recycle Bin deletion
- Destructive confirmation delay
- Rollback and reconciliation

**Excludes:** Batch operations, permanent deletion, installation, conflicts, and BentoMod import.

**Exit criteria:**

- Primaries and sidecars remain synchronized.
- Priority behavior matches compatibility fixtures.
- Folder operations remain inside the mod root.
- Deletion is recoverable and never silently becomes permanent.
- Partial failures are reconciled and reported.
- Review covers destructive edge cases.

## Phase 5 - Metadata and persistence

**Outcome:** Settings, tags, and mod metadata survive safely across sessions and controlled operations.

**Includes:**

- Versioned persistence
- Safe writes and last-known-good recovery
- Persistent internal mod identity
- Tags and settings
- Schema migration tests
- Corrupt and orphaned metadata handling

**Excludes:** BentoMod import, cloud sync, and perfect external-rename matching.

**Exit criteria:**

- Same-named mods can hold separate metadata.
- Tags survive controlled rename and move.
- Corrupt data does not destroy the library.
- Failed writes preserve valid prior state.
- Review approves stored data and recovery behavior.

## Phase 6 - UAssetToolRivals boundary

**Outcome:** Cratebug has a reviewed, testable integration for the small subset of Unreal archive operations it actually needs, behind a narrow typed boundary around a pinned prebuilt UAssetToolRivals worker release tied to a known source revision.

**Includes:**

- Pinned prebuilt UAssetToolRivals worker release tied to a known source revision
- Narrow typed archive-tool adapter
- Supervised helper-process prototype as the default integration direction
- Worker distributed as a versioned release artifact from the maintained UAssetToolRivals fork; normal Cratebug development and builds do not require the .NET toolchain unless the worker is explicitly rebuilt from source
- FFI comparison only when a concrete performance, packaging, or operational reason exists
- Crash, packaging, performance, and complexity validation for the selected boundary
- Evaluate bounded parallel archive inspection and archive-tool actions using representative mod libraries; adopt concurrency only when measurements show it improves responsiveness without weakening cancellation, progress reporting, deterministic results, or filesystem safety
- A Cratebug-owned, lightweight layer for determining mod type, built instead of routing through UAssetToolRivals's full archive-extraction actions; benchmarked on its own terms for where and how much parallelization actually helps, rather than assuming the general archive-operation findings transfer unchanged
- Version checks, structured errors, logging, and test doubles
- Representative read-only archive operations from the Cratebug subset

**Excludes:** Full installation, VFX updating, exposing the complete UAssetToolRivals surface, and making the UAssetToolRivals JSON contract part of Cratebug's domain API.

**Exit criteria:**

- A written decision selects a supervised helper process by default, or documents the concrete reason to pursue FFI instead.
- Worker release version, source revision, and checksum are pinned and documented.
- Representative failures do not corrupt or crash Cratebug unexpectedly.
- The review records the parallelism evaluation, benchmark evidence, selected concurrency policy, and any decision to defer concurrency.
- Production packaging works.
- Licensing and notices are documented.
- Review approves the boundary before archive mutation begins.

## Phase 7 - Installation and archive safety

**Outcome:** Cratebug can safely install supported local mod archives.

**Includes:**

- Staged extraction
- Path-traversal and unsafe-link protection
- Bundle discovery and validation
- Installation preview
- Collision decisions
- Transaction-like apply, cleanup, and reconciliation
- Progress and cancellation

**Excludes:** Remote downloads, deep links, automatic updates, and full backup/restore.

**Exit criteria:**

- Malicious archives are rejected.
- Cancellation removes only Cratebug staging data.
- Existing mods are never overwritten without a decision.
- Failed installs do not leave partial bundles presented as installed.
- Representative classic and IoStore fixtures work.
- Review covers success and major failure paths.

## Phase 8 - Asset conflict inspection

**Outcome:** Users can inspect overlapping internal Unreal asset paths.

**Includes:**

- Same-priority conflict detection
- Cross-priority overlap inspection
- UAssetToolRivals supplies archive facts such as internal paths; Cratebug owns enabled/disabled filtering, priority comparison, overlap rules, caching and invalidation, and user-facing conflict results.
- UAssetToolRivals does not decide whether two mods conflict.
- Clear distinction among asset conflict, destination collision, invalid bundle, and duplicate priority
- Conflict details UI
- Progress, cancellation, and justified caching

**Excludes:** Automatic conflict resolution, content merging, and automatic priority rewriting.

**Exit criteria:**

- Synthetic fixtures produce expected results.
- Disabled mods are handled appropriately.
- Tool failures produce clear unavailable or partial states.
- Large scans remain responsive.
- Review approves terminology and presentation.

## Phase 9 - BentoMod migration

**Outcome:** Users can optionally import selected BentoMod state without modifying BentoMod.

**Includes:**

- Read-only state detection and parsing
- Import preview and confirmation
- Selected game path, tags, tag catalog, and approved appearance preferences
- Ambiguity reporting and repeatable behavior

**Excludes:** Mod-file mutation, automatic import, unsafe bypass settings, launcher state, updater state, and crash-monitor state.

**Exit criteria:**

- BentoMod files remain unchanged.
- Malformed state fails safely.
- Ambiguous filename-based tags are surfaced.
- Repeated imports do not duplicate data unpredictably.
- Review uses disposable copies of representative real state.

## Phase 10 - Release hardening

**Outcome:** Cratebug is ready for its first public release.

**Includes:**

- Installer branding, upgrades, and uninstall behavior
- Versioning and release notes
- License and third-party notices
- Clean-machine Windows 10 and 11 testing
- Accessibility, scaling, performance, security, and recovery review
- User documentation and reproducible release builds

**Excludes:** Automatic updates, cross-platform packages, and unrelated convenience features.

**Exit criteria:**

- Clean systems can install, run, upgrade, and uninstall.
- Core workflows pass automated and manual testing.
- No known critical data-loss issue remains.
- User-data retention matches documentation.
- Release artifacts are reproducible.
- Principal UI states receive final screenshot approval.

## Deferred post-release work

Potential later work includes batch operations, filesystem watching, full backup and restore, automatic updates, remote downloads, browser intake, game launching, crash monitoring, character data updates, recompression, VFX updating, virtual collections, permanent deletion, and advanced external-rename reconciliation.

These require separate specification and roadmap decisions.
