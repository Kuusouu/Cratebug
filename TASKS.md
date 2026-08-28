# Cratebug Active Tasks

**Phase:** 12 - Provider-based library auto-detection (post-release)
**Status:** Complete. Review approved 2026-08-28; see `docs/reviews/phase-12-review.md`.

This file contains only the active phase. Replace it when Phase 12 is complete.

## Phase objective

Let Cratebug find the Marvel Rivals mod library itself instead of requiring a pasted path. The library toolbar gains a detect control showing the active store's logo; detection locates the game installation through a small per-store provider seam and points the library at its `~mods` folder. When the game install is found but `~mods` does not exist, a dialog offers to create it. Steam ships first and is the default provider; Epic Games follows as the second provider once it can be verified against a real Epic installation.

BentoMod's `find_marvel_rivals`/`get_steam_library_paths`/`get_steam_path_from_registry` (`archive/BentoMod/bentomod/src/utils.rs:385-490`) and `auto_detect_game_path` (`main_tauri.rs:865-897`) are behavioral references for registry resolution, `libraryfolders.vdf` parsing, and install-shape validation. Two deliberate differences: BentoMod has no Epic detection, and its auto-detect silently creates `~mods`; Cratebug requires explicit confirmation for creation. BentoMod also shells out to `reg.exe`; Cratebug reads the registry directly via `golang.org/x/sys/windows/registry`, which is already a direct dependency.

## Design decisions (user-approved 2026-08-28)

* **Provider seam, Steam first.** `internal/gamedetect` defines a minimal provider interface and a small registry of providers. Steam is the only implementation this phase; the Epic Games provider lands in a follow-up once a real Epic installation is available for path verification and live testing, without changing call sites.
* **Detection is read-only.** Detecting never writes. The one permitted write outside a configured mod root is creating an empty `~mods` directory inside a provider-verified game installation, and only after the user confirms it in a dialog.
* **Three-state detection result.** Game install found with an existing `~mods` (library path returned), game install found without `~mods` (install path returned so creation can target it), game install not found. Each state gets its own frontend presentation.
* **Creation flow.** The dialog reads "No mod library was found for <Steam/Epic>" and offers to create it. `CreateLibrary` re-runs detection server-side rather than accepting a path argument from the frontend, so nothing untrusted crosses the trust boundary and creation can only ever target a provider-verified install.
* **Applying a detected library.** With no mod root configured, a found library is applied directly. When a root is already configured, applying a different detected path requires confirmation first.
* **Steam is the default provider.** The persisted setting is empty-means-Steam. The Settings provider selector lists Epic as visible-but-unavailable until its provider ships, and the toolbar copy names Steam until then.
* **Trademarks.** Steam and Epic logos appear as store indicators; NOTICE.md notes that Valve and Epic own their respective marks.

## Exit criteria

* A Steam installation with an existing `~mods` is detected and the library scans from it.
* A Steam installation without `~mods` produces the create-library dialog; confirming creates exactly that folder and nothing else; declining changes nothing.
* No Steam installation reports clearly and offers no creation.
* Detection never writes; confirm-to-create is the only write outside a configured mod root.
* The provider setting persists across restarts and the toolbar control reflects the active store.
* Provider tests use disposable fixtures with injectable registry and vdf paths, never a real install.
* `check.ps1` passes; running-app states are screenshotted and reviewed.

## Out of scope

* Epic Games provider implementation (follow-up, gated on a real Epic installation)
* BentoMod settings or game-path import (the SPEC section 14 importer remains separate)
* Silent, automatic, or periodic re-detection; watching the game install for changes
* Anything else in ROADMAP.md's deferred post-release list

## 12.1 Backend: `internal/gamedetect`

* Pure Go, independent of Wails and React, following the existing package-boundary pattern.
* Minimal provider interface (name plus detect) and a small provider registry; no plugin machinery.
* Steam provider:
  * Steam root from the registry with ordinary default-path fallbacks, as BentoMod does.
  * Parse `steamapps/libraryfolders.vdf` `"path"` entries, handling `\\` unescaping.
  * A library counts when `<lib>\steamapps\common\MarvelRivals\MarvelGame\Marvel\Content\Paks` exists; first match wins in deterministic order.
* Registry and vdf lookups go through injectable seams so tests fabricate installs under `t.TempDir` with fixture vdf files.
* Typed three-state result; specific errors, not generic failures.

## 12.2 Backend: App bindings and settings

* `DetectLibrary(provider)` returning the three-state result; `CreateLibrary(provider)` re-detecting server-side and creating `~mods` only when the install is verified and the folder is missing.
* `Settings.LibraryProvider` in the versioned metadata store, empty-means-Steam, additive-field persistence like `LastSeenVersion`.
* Unit tests for provider dispatch, all three detection outcomes, create/refuse paths (unverified install, folder already present), and setting round-trip.

## 12.3 Frontend: detect control, settings selector, dialogs

* Library toolbar: detect button beside the mod-root field, labeled around "Automatically detect mods in Steam installation", showing the active store's logo; busy state matches existing conventions.
* Settings dialog: provider selector section; Epic visible but unavailable until implemented.
* Dialog flow per detection state: apply or switch confirmation when a root is already configured, the create-library offer when `~mods` is missing, a clear not-found message otherwise. Applying triggers the existing scan flow.
* Empty mod-root state makes detection the primary affordance; the pasted-path field remains as fallback.

## 12.4 Assets and notices

* Steam and Epic SVG logo assets that hold up in light and dark themes.
* NOTICE.md trademark line for the Valve and Epic marks.

## 12.5 Validate and review

* Run `check.ps1` and the full Go and frontend test suites.
* Drive the running app: scan behavior against `C:\ModsFixtures`, detection against the real Steam install (read-only), and the create-library dialog. Explicit approval is required at the time before any `~mods` is created inside the real install.
* Screenshot the toolbar detect control, each dialog state, and the Settings selector; save under `docs/screenshots/phase-12/`.
* Update `docs/USER_GUIDE.md` and `docs/TROUBLESHOOTING.md` for detection and the create-library prompt.
* Create `docs/reviews/phase-12-review.md`.

**Verify:** Review approval closes the phase; the Epic Games provider follow-up is then scoped against the installed Epic copy.

## Follow-up (not this phase)

* Epic Games provider: verify the install layout under a real Epic `InstallLocation`, implement the second provider behind the existing seam, enable the selector option, and test against the live Epic installation.
