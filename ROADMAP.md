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
- Worker distributed as a versioned self-contained release artifact pinned from the managed UAssetToolRivals fork (`mewclouds/UAssetToolRivals`, currently `v1.5.9`). The fork tracks upstream `XzantGaming/UassetToolRivals` and publishes the self-contained win-x64 CLI Cratebug fetches; normal Cratebug development and builds do not require the .NET toolchain unless the worker is explicitly rebuilt from source.
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

## Phase 14 - Batch actions and in-place encryption

**Status:** Complete. Review approved 2026-09-05; see `docs/reviews/phase-14-review.md`.

**Outcome:** Users can check many mods and run the same action on the set from one Actions menu, including encrypting or decrypting complete IoStore bundles in place.

**Includes:**

- Viewing vs checked selection, Ctrl/Shift range, Select all / Clear
- Catalog-header Actions dropdown (Enable, Disable, Move, Tags, Encrypt/Decrypt, Delete)
- Batch organize by looping current one-mod mutations, with partial-success reporting
- Hardcoded Marvel Rivals AES key in Go, `Identity.encrypted`, and listing encrypted IoStore with that key
- In-place IoStore encrypt/decrypt through UAssetToolRivals (`extract_iostore` + `create_mod_iostore` `obfuscate`) and a lock mark on cards
- Mixed-state and ineligible-format disable rules, plus confirm copy that encryption is a rebuild
- Game-running lock, temp+replace+rollback, progress, and cancellation
- Decision 0005 for the key, the write-surface expansion, and the selection/menu pattern

**Excludes:** Install-time obfuscation, classic-PAK encryption, converting classic mods to IoStore, a cluttered persistent Bento toolbar, new batch Wails methods for enable/move/tags/delete, exposing the AES key to the frontend, VFX/recompress, and BentoMod changes.

**Exit criteria:**

- Click, Ctrl+click, Shift-range, Select all, and Clear work in compact, large, and list.
- The Actions menu operates on the checked set. An empty set disables it.
- Batch enable/disable/move/tags/delete report partial success and never claim full success on a mixed result.
- Encrypted IoStore mods classify (not Unknown) and show a lock.
- Encrypt and Decrypt succeed on disposable IoStore fixtures. Mixed or classic selections stay disabled.
- Failed or cancelled encrypt leaves no partial bundle presented as the new mod.
- Canonical checks pass. Running-app states are screenshotted and reviewed.

## Phase 15 - Unsupported companion PAK cleanup and required encryption

**Status:** Complete. Review approved 2026-09-06; see `docs/reviews/phase-15-review.md`.

**Outcome:** Cratebug warns when installed or incoming mods still contain `chunknames` / `patched_files` companion PAK entries (anti-cheat crash as of 3 September 2026), and rewrites only those `.pak` files one at a time. After classify, it offers encryption for complete unencrypted IoStore mods that touch files outside `/Game/Marvel/Characters`, and runs those rebuilds through a write-worker pool.

**Includes:**

- Detect via `list_pak` (not filesystem scan): any primary whose listing contains `chunknames` or `patched_files`
- First populated library load per root per session offers a Yes/No rewrite
- Install preview warns and Apply rewrites the staged `.pak` before it lands in the library
- Sequential Go mutation: extract remaining files, `create_pak` without the metadata names, replace only the `.pak`, leave `.utoc` / `.ucas` untouched
- Game-running lock, park+replace+rollback, progress, cancellation
- Detect required encryption from classify path listings (outside `/Game/Marvel/Characters`)
- First-load Encrypt / Not now after classify, queued behind the companion dialog
- Encrypt/decrypt writes use a `NewWriteWorker` pool sized by `WorkerPoolSizeForLibrary`, not the classify processes
- Decision 0006 and the 0005 addendum

**Excludes:** Rebuilding IoStore containers for companion cleanup, install-time obfuscation, classic-to-IoStore conversion, BentoMod changes, and a persistent "never ask again" setting.

**Exit criteria:**

- A disposable companion PAK that only contains those names is rewritten without them (one harmless stub entry, because the worker cannot write an empty PAK). Hybrid companions keep their other files.
- First library load of a dirty library shows the companion warning once per session. Refresh does not re-prompt.
- Install preview names the affected mods and Apply writes a clean `.pak`.
- Failed or cancelled rewrite leaves that `.pak` as it was.
- A complete unencrypted IoStore whose listing leaves Characters is offered for encryption once per session after classify.
- App encrypt launches write workers using the library pool-size policy.
- Canonical checks pass.

## Phase 16 - Nexus Mods integration (BYOK)

**Outcome:** Users install Marvel Rivals mods from Nexus Mods with their own API key. Premium accounts download through the API. Free accounts start the download from the Nexus website via an `nxm://` link that Cratebug can own. The generic "Install from URL" path is removed.

**Supersedes:** Phase 10's user-typed URL download (`App.InstallFromURL`, `InstallFromUrlDialog`, `internal/install/download.go`). Streaming, stall, and progress mechanics move into `internal/nexus`. No code path accepts an arbitrary user-typed download URL.

**Includes:**

- Bring-your-own Nexus API key stored locally with DPAPI, never in `metadata.json` or the WebView
- Nexus client: validate, mod and file metadata, download links, header-driven rate limits
- Premium API download and free-user `nxm://` download; signed `key` / `expires` stay in Go
- Runtime-owned `nxm://` registration: silent if free, prompt if taken, restore on unregister
- Single-instance handoff of cold and warm `nxm://` links
- Settings for key paste, account state, and the handler toggle
- Install UI: paste a Nexus URL, file picker, `nxm://` confirm, download progress
- Persist Nexus mod, file, and version IDs on installed mods
- Uninstall cleanup of the handler and `nexus.key`
- Decision 0007

**Excludes:** In-app Nexus browsing, search, endorsements, tracked mods, update checking, browser-extension intake, automating the free-user website click, Nexus application registration, silent takeover of another app's `nxm://` handler, and arbitrary-URL remote install.

**Exit criteria:**

- A connected premium account can resolve a Nexus URL, download, preview, and install through the existing staged pipeline.
- A free account is sent to the Nexus page, and an `nxm://` link completes the same install.
- Another app's handler is never taken without a named confirmation. Unregister restores it.
- The API key and signed `nxm` parameters never appear in Wails payloads, `metadata.json`, or error text.
- Generic URL install is gone.
- Canonical checks pass. Running-app states are screenshotted and reviewed.

## Phase 17 - Linux distribution

**Outcome:** Cratebug runs on Linux as a first-class build. Marvel Rivals is playable on Linux through Proton with its anti-cheat working, so the mods a Linux player manages are the same Windows bundles in the same Steam library layout. The work is making Cratebug itself portable, not changing what a mod is.

**Includes:**

- Portable Go core. Four packages block a Linux build today, each already following the repository's `_windows.go` / `_other.go` split convention used by `internal/secret` and `internal/urlscheme`:
  - `internal/gamedetect` - Steam detection reads the Windows registry. Linux resolves the library from `steamapps/libraryfolders.vdf` under the user's Steam root. Epic has no native Linux launcher; decide between Heroic/Legendary support and declaring Epic Windows-only.
  - `internal/mutation` - Recycle Bin deletion and the running-game check are Win32. Linux needs an XDG trash implementation and a process check that never silently degrades to permanent deletion.
  - `internal/update` - the updater writes and runs a `.bat` script around an NSIS installer. AppImage updates are a different mechanism entirely.
  - `internal/uassettool` - `syscall.SysProcAttr{HideWindow}` does not compile off Windows, and `WorkerExecutableName` is hardcoded to `UAssetTool.exe`.
- Secret storage. `internal/secret` compiles on Linux but its `protect_other.go` stub returns an error, so the Nexus API key has no at-rest protection. Linux needs a real backend (Secret Service / libsecret, kwallet) or an explicitly documented weaker fallback. Storing the key in plaintext silently is not acceptable.
- Worker on Linux. The pinned release already publishes `UAssetTool-linux-x64.tar.gz` alongside the Windows zip, self-contained and multi-file since `v1.5.9`. Needs: a portable fetch script (`fetch-uassettool.ps1` is PowerShell and zip-only), the executable-bit set on extract, platform-aware worker path resolution, and confirmation that the tool's runtime Oodle download resolves the Linux library rather than `oo2core_9_win64.dll`.
- `nxm://` handling through a `.desktop` file and `xdg-mime` instead of the Windows registry, including restoring a previous handler on unregister.
- Build and packaging. Wails needs GTK3 and WebKit2GTK at build and run time, with the `webkit2_41` build tag on modern distros and `webkit2_40` on older ones. Publish an AppImage as the primary artifact so one download works everywhere.
- Documented per-distro setup: `libgtk-3-dev` + `libwebkit2gtk-4.1-dev` (Debian/Ubuntu), `gtk3-devel` + `webkit2gtk4.1-devel` (Fedora), `gtk3` + `webkit2gtk-4.1` (Arch).
- CI builds the Linux artifact alongside the Windows installer.

**Excludes:** macOS, ARM builds, Flatpak and Snap, native distro packages (`.deb`, `.rpm`, AUR), Steam Deck / gamescope-specific work, and any attempt to make Cratebug manage mods for a Proton prefix differently from a normal Steam library.

**Test matrix:** Pop!_OS (Ubuntu-based), Fedora, CachyOS (Arch-based). A virtual machine per distro is sufficient.

**Exit criteria:**

- `go build ./...` and the full test suite pass with `GOOS=linux`.
- The AppImage launches on all three distros and detects a real Steam library.
- Enable, disable, rename, move, tag, install, delete, conflict detection, and encrypt/decrypt all work on each, with the worker running.
- Deletion reaches the desktop trash and is restorable. It never falls back to permanent deletion.
- The Nexus API key is either protected at rest or its fallback is documented in the UI and the user guide.
- Windows behavior is unchanged: canonical checks still pass and the installer still builds.
- Setup and publishing steps are documented well enough to follow from a clean install of each distro.
- Screenshots from each distro are reviewed against the Windows build.

## Deferred post-release work

Potential later work includes BentoMod/Repak-X state migration, install-time obfuscation, filesystem watching, full backup and restore, game launching, crash monitoring, character data updates, recompression, VFX updating, virtual collections, permanent deletion, and advanced external-rename reconciliation.

These require separate specification and roadmap decisions.
