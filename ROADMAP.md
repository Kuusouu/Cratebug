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
- Worker distributed as a versioned release artifact pinned directly from upstream UAssetToolRivals (`XzantGaming/UassetToolRivals`), which already publishes an actively maintained, official prebuilt release for this purpose; normal Cratebug development and builds do not require the .NET toolchain unless the worker is explicitly rebuilt from source. Cratebug owning its own fork remains an option if a concrete need to diverge or control the release pipeline ever arises, but is not adopted speculatively.
- FFI comparison only when a concrete performance, packaging, or operational reason exists
- Crash, packaging, performance, and complexity validation for the selected boundary
- Evaluate bounded parallel archive inspection and archive-tool actions using representative mod libraries; adopt concurrency only when measurements show it improves responsiveness without weakening cancellation, progress reporting, deterministic results, or filesystem safety
- A Cratebug-owned, lightweight layer for determining mod type, built instead of routing through UAssetToolRivals's full archive-extraction actions; benchmarked on its own terms for where and how much parallelization actually helps, rather than assuming the general archive-operation findings transfer unchanged
- Hero and skin name resolution from the same internal path listings, sourced from a fetched and cached character-ID table, as a separate later step built on top of the type-determination layer
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

## Phase 7 - UAssetToolRivals UI integration

**Outcome:** Users can see each mod's determined type/category in the library UI, backed by the Cratebug-owned classification layer built in Phase 6.

**Includes:**

- Wire `internal/uassettool` and `internal/modtype` into the application layer (`app.go`), including launching and supervising the worker for the lifetime of a session
- Expose mod type/category and hero/skin name through the Wails-bound API to the frontend
- Render category, hero/skin name, and hero portrait thumbnails in the library UI
- A caching strategy for classification results so repeated scans do not repeatedly re-invoke the worker; BentoMod's mtime-keyed in-memory cache is a reference pattern, not an architectural template
- Apply the entry-count-tiered concurrency policy from `docs/decisions/0003-uassettoolrivals-boundary.md` (`WorkerPoolSizeForLibrary`) when classifying a full library
- Progress and loading states for classification, since it runs after the initial fast filesystem scan rather than blocking it
- Graceful, clearly-labeled "unknown" presentation for mods `Determine`/`DetermineIdentity` cannot classify (encrypted IoStore containers, incomplete bundles)
- Package the pinned worker binary and its third-party notices into the production build and installer, deferred from Phase 6 since nothing invoked the worker until this phase wires it in

**Excludes:** Installation, asset conflict inspection, VFX updating, and exposing any UAssetToolRivals surface beyond what Phase 6 already scoped.

**Exit criteria:**

- Mod type/category renders correctly for representative real and disposable-fixture libraries.
- Classification does not block the initial library render; the catalog appears immediately and categories populate progressively.
- Encrypted or otherwise undeterminable mods show a clear "unknown" state, never an error or a crash.
- Caching avoids redundant worker calls for mods unchanged since the last classification.
- Production packaging and the installer include the pinned worker binary and its third-party notices.
- Review approves the UI presentation and responsiveness against a large real library.

## Phase 8 - Installation and archive safety

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

## Phase 9 - Asset conflict inspection

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

## Phase 10 - Automatic updates, remote mod downloads, and release hardening

**Outcome:** Cratebug has a real, CalVer-tagged GitHub release pipeline; an installed build can check for, download, and silently apply a new release in place and relaunch; a mod can be installed from a URL instead of only a local file; and the app is ready for its first public release.

Phase 11 folded into this phase: the update/apply flow needs a real release to test against, so building the release pipeline and hardening the release itself happen together instead of in sequence.

**Includes:**

- `CHANGELOG.md` plus a tag-triggered GitHub Actions release workflow that builds the installer and publishes a GitHub release, prerelease-tagged for `-rcN` builds
- Update check against the published GitHub release, silent in-place apply through the existing per-user NSIS installer, and an automatic relaunch
- A "what's new" changelog UI shown on an available update and once after an applied update
- Downloading a mod archive from a user-provided URL into the existing Phase 8 staged-install pipeline, treated as untrusted input the same as a local archive
- Installer branding, upgrade-in-place behavior, and uninstall correctness
- License and third-party notices, clean-machine Windows 10/11 install/upgrade/uninstall testing
- Accessibility, scaling, performance, security, and recovery review across the app
- User documentation and reproducible release builds

**Excludes:** Silent or forced background updates without user confirmation, browser-extension or deep-link intake, and cross-platform packaging.

**Exit criteria:**

- An older installed build detects and offers a newer published release, applies it without losing user settings or metadata, and relaunches showing the changelog.
- A URL-sourced download goes through the same staging, validation, and preview as a local archive install.
- Update and download failures report clearly and never leave a partially-applied install.
- Clean systems can install, run, upgrade, and uninstall.
- No known critical data-loss issue remains.
- Release artifacts are reproducible from the tag-triggered workflow.
- Review approves the update, remote-download, and release flows.

## Phase 12 - Provider-based library auto-detection (post-release)

**Outcome:** Cratebug can find the Marvel Rivals mod library itself instead of requiring a pasted path. Detection is built around a small per-store provider seam, with Steam implemented first and Epic Games added once it can be verified against a real Epic installation. When the game install is found but the `~mods` library does not exist, the user is offered a confirmed, single-folder creation.

**Includes:**

- `internal/gamedetect`: minimal provider interface and registry, Steam provider first
- Steam detection via the Windows registry, `libraryfolders.vdf` parsing, and install-shape validation
- Wails-bound detect and confirm-to-create methods; persisted provider setting defaulting to Steam
- Library toolbar detect control showing the active store's logo, and a provider selector in Settings
- Store-logo assets and trademark notice

**Excludes:** Epic Games provider implementation (Phase 13), any other deferred roadmap item.

**Exit criteria:**

- Detection is read-only; the only write outside a configured mod root is the user-confirmed creation of an empty `~mods` directory inside a provider-verified game install.
- An existing Steam library is detected and applied; a missing `~mods` offers creation; no Steam install reports clearly.
- The provider setting persists and the toolbar control reflects the active store.
- The provider seam lets the Epic provider land without changing call sites.
- Canonical checks pass; running-app states are screenshotted and reviewed.

## Phase 13 - Epic Games library detection

**Status:** Complete. Review approved 2026-09-02; see `docs/reviews/phase-13-review.md`.

**Outcome:** Cratebug detects a Marvel Rivals mod library from the Epic Games launcher through the Phase 12 provider seam, verified against a real Epic installation.

**Includes:**

- `EpicProvider` in `internal/gamedetect`, reading `%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests\*.item`
- Match Marvel Rivals by verified `CatalogNamespace` or `DisplayName`, skip incomplete installs and DLC, validate `{InstallLocation}\MarvelGame\Marvel\Content\Paks`
- Register `epic` in the provider registry; enable the Settings selector and detect-dialog logo
- Disposable `.item` fixture tests; live read-only detect against the maintainer's Epic install

**Excludes:** Other stores, silent re-detection, BentoMod import, and creating `~mods` in the real Epic Paks without an explicit yes at verification time.

**Exit criteria:**

- An Epic install with `~mods` is detected and scanned; without `~mods`, the create-library dialog appears with the Epic logo.
- No Epic install reports clearly and offers no creation.
- Detection never writes; confirm-to-create remains the only write outside a configured mod root.
- Tests use injectable manifests under `t.TempDir`, never ProgramData or the real game.
- Canonical checks pass; running-app Epic states are screenshotted and reviewed.

## Deferred post-release work

Potential later work includes BentoMod/Repak-X state migration, batch operations, filesystem watching, full backup and restore, browser intake, game launching, crash monitoring, character data updates, recompression, VFX updating, virtual collections, permanent deletion, and advanced external-rename reconciliation.

These require separate specification and roadmap decisions.
