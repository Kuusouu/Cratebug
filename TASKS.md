# Cratebug Active Tasks

**Phase:** 13 - Epic Games library detection
**Status:** Complete. Review approved 2026-09-02; see `docs/reviews/phase-13-review.md`.
**Branch:** `feat/epic-gamedetect`

This file contains only the active phase. Replace it when starting the next phase.

## Phase objective

Let Cratebug detect a Marvel Rivals mod library from the Epic Games launcher the same way it already does for Steam. The Phase 12 provider seam, Wails bindings, create-library confirmation, and Settings selector stay in place. This phase adds the Epic provider, enables the selector option, and verifies detection against a real Epic install.

Live layout verified 2026-09-02 on the maintainer's machine (do not hardcode the install folder name):

* Manifests: `%ProgramData%\Epic\EpicGamesLauncher\Data\Manifests\*.item`
* `DisplayName`: `Marvel Rivals`
* `CatalogNamespace`: `38e211ced4e448a5a653a8d1e13fef18`
* `InstallLocation`: `C:\Program Files\Epic Games\MarvelRivalsjKtnW` (Epic appends a random suffix)
* Paks: `{InstallLocation}\MarvelGame\Marvel\Content\Paks`
* `~mods` was not present; a live detect should return `installFound`

Steam's extra `steamapps\common\MarvelRivals\` prefix is not present under Epic. `AppName` is an opaque hash and is not a match key.

## Design decisions (user-approved 2026-09-02)

* **Same seam, second provider.** `internal/gamedetect` already has `Provider`, `Registry`, `DetectLibrary`, `CreateLibrary`, and `Settings.LibraryProvider`. Register `ProviderEpic = "epic"` in `providerNames` and `NewDefaultRegistry` together. No new Wails methods.
* **Manifests are the source of truth.** Parse launcher `.item` JSON. Custom install paths are already in `InstallLocation`. Do not walk `Program Files` and do not add Steam-style hardcoded fallback roots.
* **Match rules.** Skip `bIsIncompleteInstall`. Skip DLC (`MainGameAppName` non-empty). Match when `CatalogNamespace` equals the verified constant **or** `DisplayName` equals `Marvel Rivals` (case-insensitive). First matching install whose Paks path is a directory wins, in sorted filename order.
* **Injectable manifests dir.** Tests write fixture `.item` files under `t.TempDir` and never touch ProgramData or the real game.
* **Steam remains the default.** Empty `LibraryProvider` still means Steam. Users with only Epic pick it in Settings.
* **Create-library is unchanged.** `CreateLibrary` already re-detects by provider name and creates only `Paks\~mods`. Do not create `~mods` inside the real Epic Paks without an explicit yes at verification time.
* **Frontend uses the existing maps.** Add `"epic"` to `libraryProviders` / labels / `providerLogos`. Remove the hardcoded disabled Epic control. `DetectLibraryDialog` must use `providerLogos[provider]` instead of a hardcoded Steam logo.

## Exit criteria

* An Epic installation with an existing `~mods` is detected and the library scans from it.
* An Epic installation without `~mods` produces the create-library dialog with the Epic logo; confirming creates exactly that folder and nothing else; declining changes nothing.
* No Epic installation reports clearly and offers no creation.
* Detection never writes; confirm-to-create remains the only write outside a configured mod root.
* Settings lists Epic as a selectable provider; the toolbar detect control shows the Epic logo when Epic is active.
* Provider tests use disposable `.item` fixtures with an injectable manifests dir, never the real ProgramData or game install.
* `check.ps1` passes; running-app Epic states are screenshotted and reviewed.

## Out of scope

* Default window size and list-view scrollbar work (remaining-todos TODO 2)
* Other stores, silent re-detection, BentoMod import
* Writing into the real Epic `~mods` without a separate yes
* Anything else in ROADMAP.md's deferred post-release list

## 13.1 Backend: `EpicProvider`

* New `internal/gamedetect/epic.go`, independent of Wails and React, following `steam.go`.
* Production manifests dir: `filepath.Join(os.Getenv("ProgramData"), "Epic", "EpicGamesLauncher", "Data", "Manifests")`.
* Read `*.item` (case-insensitive), skip unreadable or invalid JSON, process in sorted filename order.
* Paks relative path: `MarvelGame\Marvel\Content\Paks`.
* Same three-state `Detection` as Steam. A Paks path that exists as a file is skipped.
* Injectable `manifestsDir` so tests never touch the real launcher data.

**Verify:** `go test ./internal/gamedetect/ -count=1` covers the cases in 13.2.

## 13.2 Backend: register and tests

* Add `ProviderEpic = "epic"` to `providerNames` and `NewDefaultRegistry`.
* Flip `TestValidProvider`: `"epic"` is now valid.
* `TestSetLibraryProviderRejectsAnUnknownProvider` currently uses `"epic"` as the unknown. Change it to `"egs"` (same sentinel `app_test.go` already uses). Accept `"epic"` as a registered provider.
* New fixture tests: library found; install found without `~mods`; not found (empty manifests, wrong game, incomplete, DLC); Paks-as-file; first matching `.item` in sorted filename order; DisplayName match when namespace differs; malformed JSON skipped so a later valid `.item` still counts.

**Verify:** `go test ./internal/gamedetect/ ./internal/metadata/ -count=1`

## 13.3 Frontend: enable Epic in the selector and dialogs

* `libraryTypes.ts`: add `"epic"` to `libraryProviders` and `libraryProviderLabels`.
* `StoreLogos.tsx`: `providerLogos.epic = EpicGamesLogo`.
* `SettingsDialog.tsx`: delete the hardcoded disabled Epic button and the "coming soon" hint.
* `DetectLibraryDialog.tsx`: render `providerLogos[provider]`, not a hardcoded `SteamLogo`.
* Toolbar copy in `LibraryScreen.tsx` is already provider-aware; leave it.

**Verify:** `bun run check` from `frontend/`. Settings shows Epic as selectable. An Epic detect dialog shows the Epic logo.

## 13.4 Docs

* `docs/USER_GUIDE.md`: Epic is selectable, not "coming".
* `docs/TROUBLESHOOTING.md`: Epic looks at launcher manifests, not Steam libraries. Paste `Paks\~mods` if the launcher has no record.

**Verify:** The two docs name both Steam and Epic, and the paste-path fallback is still documented.

## 13.5 Validate and review

* Run `check.ps1` and the full Go and frontend test suites.
* Drive the running app with Settings on Epic, detect against the real Epic install (read-only). Expect `installFound` and the create-library dialog. Explicit approval is required at the time before any `~mods` is created inside the real Epic Paks.
* Screenshot the Settings selector with Epic selected, the toolbar detect control showing the Epic logo, and the Epic create-library dialog; save under `docs/screenshots/phase-13/`.
* Create `docs/reviews/phase-13-review.md`.

**Verify:** Review approved 2026-09-02. Phase 13 is complete.

## Follow-up (not this phase)

* remaining-todos TODO 2: default window size and list-view scrollbar at 1080p / 100% scale
