# Phase 7 Review

**Date:** 2026-08-25
**Status:** Approved

## Outcome

Phase 7 completes the end-to-end integration of the UAssetToolRivals worker, classification layer, and character/skin metadata into Cratebug's Go backend, Wails bridge, and React/TypeScript frontend. The backend manages a session-lived, mtime-keyed in-memory classification cache and a dynamic, miss-based worker pool that supervises pinned `UAssetTool.exe` child processes with automatic crash recovery. The library UI exposes asynchronous background classification, rendering format badges, category pills, hero/skin names, and hero/skin thumbnail portraits with accessible tooltips across compact, large, and list views. Progressive skeleton placeholders give immediate visual feedback during classification without shifting layout, and unresolvable or encrypted entries display a graceful, neutral "Unknown" state without error banners. Packaging is completed with a custom NSIS installer script bundling the pinned worker and third-party notices, Git LFS asset tracking, and automated CI installer validation.

---

## Worker lifecycle design and executable path resolution

`internal/uassettool` and `internal/modtype` manage worker processes with a lazy, resilient lifecycle:

1. **Path Resolution (`ResolveExecutablePath`):**
   - **Environment Override:** `CRATEBUG_UASSETTOOL_PATH` for custom development setups.
   - **Installed Production Layout:** `<executable-dir>/uassettool/UAssetTool.exe` adjacent to `Cratebug.exe`.
   - **Development Layout:** `<working-directory>/build/uassettool/UAssetTool.exe`.
2. **Lazy Pool Initialization:** Workers are never spawned on application startup; the worker pool starts only when `App.ClassifyLibrary` encounters cache misses.
3. **Crash Recovery & Worker Replacement:** If a worker process terminates unexpectedly mid-call, `Worker.Call` catches the error, tears down the faulty instance, launches a fresh supervisor process, and resumes processing the remaining queue without dropping requests or crashing the app.
4. **Graceful Application Shutdown:** `App.Shutdown` hooks into Wails's `OnShutdown` lifecycle event in `main.go`, closing the pool's job queue and cleanly terminating all running worker subprocesses.

---

## Classification cache design and invalidation behavior

`internal/modtype.Cache` (`internal/modtype/cache.go`):

- **Keying:** In-memory map keyed by entry ID and archive file modification time (`mtime`), ensuring modified archives are automatically invalidated and re-classified while untouched mods hit cache instantly.
- **Miss-Based Pool Sizing:** `ClassifyLibrary` checks the cache first. If all entries hit cache, no workers are launched. On cache misses, the worker pool dynamically sizes to the number of missing entries using the tiered concurrency policy from Phase 6 (`< 700 → 4`, `700–1,499 → 8`, `≥ 1,500 → 16`).
- **Mutation Re-keying:** Local metadata mutations (`RenameMod`, `SetModPriority`) update entry IDs in place via `updateMutatedEntry` in `LibraryScreen.tsx`, re-keying cached identities from `previousID` to `id` without requiring an archive rescan or worker call.

---

## Wails-bound API surface and data contracts

The Wails bridge exposes two core methods:

1. **`App.ClassifyLibrary(modRoot string, entries []discovery.Entry) (map[string]modtype.Identity, error)`**
   - Accepts scanned library entries, resolves cache hits, executes worker pool jobs for misses, updates the cache, and returns a mapped dictionary of `modtype.Identity`.
2. **`modtype.Identity` Schema:**
   ```go
   type Identity struct {
       Category      Category `json:"category"`
       CharacterID   string   `json:"characterID"`
       CharacterName string   `json:"characterName"`
       SkinID        string   `json:"skinID"`
       SkinName      string   `json:"skinName"`
   }
   ```
   - TypeScript bindings are generated in `frontend/wailsjs/go/models.ts` and `App.d.ts`.

---

## Character & skin resolution improvements and collision prevention

During Phase 7 integration, character table parsing and resolution were refined:

1. **Active Roster Bounding (`1011..1066`):**
   - `internal/modtype/character_data.go` strictly filters playable heroes to the active roster range `1011` (Hulk) through `1066` (The Hood).
   - Stale duplicate rows, `(Old)` suffixes, and dev test IDs in the upstream community table are filtered out, fixing ID collisions where Cyclops (`1063`) was misattributed to `Nightcrawler (Old)` and Devil Dinosaur (`1062`) to `Beast (Old)`.
2. **Deterministic Multi-Skin Handling:**
   - Single-character mods containing multiple skin IDs return `(characterID, characterName, "", "")`, treating skin resolution as ambiguous and deterministic while preserving hero attribution.

---

## Asset pipeline, branding, and Git LFS tracking

1. **Hero & Skin Sourcing Pipeline (`fetch-hero-portraits.ps1`):**
   - Downloads 53 base hero avatars and 590 skin icons from Rivalskins directly into `frontend/src/assets/heroes/<id>.png`.
   - Includes built-in self-validation asserting that all 53 hero PNGs and downloaded skin PNGs exist and have non-zero file sizes.
2. **Git LFS Integration:**
   - Configured `.gitattributes` to track `*.png`, `*.ico`, `*.zip`, and `*.ttf` via Git LFS.
   - Updated `.github/workflows/ci.yml` with `with: lfs: true` for checkout.
   - Documented Git LFS prerequisites and clone instructions in `README.md`.
3. **Neutral Crate Brand-Mark Application Icon:**
   - Rendered the header brand-mark crate logo (dark rounded squircle `#111827`, border `#374151`, white crate geometry) into multi-resolution `.ico` (16..256) and 512x512 `.png`.
   - Used unified Signed Distance Field (SDF) rasterization with 4x super-sampling, geometrically clipping inner lines to the outer rounded rectangle envelope to prevent line cap protrusion past corner curves.
   - Configured canonical build locations in `build/windows/icon.ico` and `build/appicon.png`, and source copies in `frontend/src/assets/`.

---

## Progressive loading and graceful "Unknown" presentation

1. **Progressive Skeleton Badges:**
   - Mod cards render a pulsing `.category-skeleton` badge during background classification.
   - Holds layout space smoothly without shifting or jank.
   - Marked with `aria-hidden="true"` to prevent redundant screen reader announcements on initial load.
   - Includes `@media (prefers-reduced-motion: reduce)` support disabling animation and rendering a static 50% opacity badge.
2. **Selected Mod Actions Panel:**
   - Displays " · Classifying..." in place of the category when an entry is selected while classification is pending.
3. **Graceful "Unknown" State:**
   - Unresolvable or encrypted mods display `Category: "Unknown"` with muted slate styling (`.category-unknown`), non-alarmingly without error banners or crashes.
   - Hero thumbnail fallback renders the package icon (`<Package />`) cleanly when hero portraits are unavailable.

---

## Production packaging and NSIS installer

1. **Custom NSIS Template (`build/windows/installer/project.nsi`):**
   - Installs `Cratebug.exe` and `THIRD_PARTY_NOTICES.md` to `$INSTDIR`.
   - Installs pinned `UAssetTool.exe` to `$INSTDIR\uassettool\UAssetTool.exe`.
   - Sets up Start Menu and Desktop shortcuts and uninstaller.
   - Recursively removes `$INSTDIR` and shortcuts upon uninstallation.
2. **Toolchain & CI:**
   - Configured `mise.toml` `[env] _.path` with NSIS directories for automatic `makensis` resolution.
   - Updated `.github/workflows/ci.yml` to run `fetch-uassettool.ps1` and build/assert both `build/bin/Cratebug.exe` and `build/bin/Cratebug-amd64-installer.exe`.
3. **Local Packaging Verification:**
   - Built `build/bin/Cratebug-amd64-installer.exe` (87.45 MB) and `build/bin/Cratebug.exe` (61.65 MB).
   - Inspected archive contents with `7z l`: confirmed `Cratebug.exe` (61.65 MB), `uassettool\UAssetTool.exe` (72.98 MB), `THIRD_PARTY_NOTICES.md` (5.56 KB), and `uninstall.exe` are bundled.

---

## Commands and tests run

```powershell
.\check.ps1
```

Validation output:
- **Go Formatting (`gofmt`):** Passed with 0 changes.
- **Frontend Checks:** Biome format check (22 files checked, 0 fixes), Biome lint (0 errors, 0 warnings), TypeScript typecheck (`tsc --noEmit`), and Vite bundle build (`bun run build`) passed clean.
- **Go Vet (`go vet ./...`):** Passed with 0 warnings.
- **Go Test Suite (`go test ./...`):** 168 Go tests passed (0 failures, 3 pre-existing symlink skips).
- **Frontend Test Suite (`bun test`):** 23 frontend unit tests passed across `entryPresentation.test.ts`.

---

## Manual checks

1. **Live App Verification (`wails dev`):**
   - Scanned fixture libraries; verified that format badges (`IoStore`/`Classic`) render immediately, followed by progressive skeleton pulsing pills, and smoothly transition to resolved category badges (`Mesh`, `VFX`, `Audio`, `UI`, `Texture`, etc.) and hero portrait thumbnails with hover tooltips.
   - **Progressive Classifying State:** `docs/screenshots/phase-7/task-7.6-classifying.png` captures the intermediate scan state with pulsing category skeleton badges.
   - **Fully Classified State:** `docs/screenshots/phase-7/task-7.8-classified.png` captures the fully resolved catalog displaying category badges, format pills, and corresponding hero portraits (Cloak & Dagger, Cyclops, Daredevil, Devil Dinosaur, Jeff the Land Shark, Deadpool) alongside graceful package icon fallbacks.
2. **Installer Verification:**
   - Generated `Cratebug-amd64-installer.exe` with Wails/NSIS, inspected archive contents with `7z l`, extracted to temporary directory, and verified that `uassettool\UAssetTool.exe` and `THIRD_PARTY_NOTICES.md` are present at the expected locations.

---

## Known limitations and deferred findings

1. **Character ID Table Concurrency Tiers (700/1,500):** Interpolated from Phase 6 benchmark measurements across 72, 504, 864, and 2,592 entries.
2. **Directory Symlink Skips:** The 3 pre-existing directory-symlink test skips remain (environment lacks `SeCreateSymbolicLinkPrivilege`, documented since Phase 4).
3. **Installer Signing:** Phase 7 packages remain unsigned; code signing and release distribution are scheduled for release hardening.

---

## Review approval

**Decision:** Approved. All Phase 7 tasks (7.1 through 7.8) and exit criteria are complete, verified, and ready for Phase 8.
