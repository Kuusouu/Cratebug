# Cratebug Active Tasks

**Phase:** 9 - Asset conflict inspection
**Status:** Complete. Approved.

This file contains only the active phase. Replace it when Phase 9 is complete.

## Phase objective

Give Cratebug the ability to inspect overlapping internal Unreal asset paths across enabled mods. UAssetToolRivals supplies archive facts such as internal paths; Cratebug owns enabled/disabled filtering, priority comparison, overlap rules, caching and invalidation, and user-facing conflict results. UAssetToolRivals does not decide whether two mods conflict.

Phase 7 already resolves every mod's internal asset path listing through the UAssetTool bridge (`internal/modtype`'s `listInternalPaths`, used by `Determine`/`DetermineIdentity`) as part of classification, but discards the path list once `Category`/`Identity` is derived from it. Phase 9 must reuse that already-fetched data rather than re-listing every mod's archive contents a second time.

## Exit criteria

* Same-priority conflicts and cross-priority overlaps are both detected and clearly distinguished. SPEC.md calls the same-priority case "duplicate priority" to set it apart from the broader asset-conflict idea; it is not a separate no-overlap check, since SPEC.md section 9 explicitly allows unrelated mods to share a priority with no overlap.
* Asset conflict, destination collision, invalid bundle, and duplicate priority (the same-priority-conflict case above) remain clearly distinct concepts in code and in the UI.
* Disabled mods are excluded from conflict detection but handled without crashing or misleading output.
* Synthetic fixtures with deliberately overlapping internal paths produce expected, deterministic results.
* UAssetToolRivals failures (including encrypted/undeterminable mods) produce a clear unavailable or partial state, never a crash or a silently wrong result.
* A full-library conflict scan remains responsive and supports progress and cancellation.
* A "Check for Conflicts" trigger runs the scan on demand; it does not enforce or auto-resolve anything, only reports.
* Conflict results are presented in a details UI grouped by resolved character (thumbnail and name, not a raw character ID), with no automatic resolution.
* Review covers success and major failure paths against representative fixtures.

## Out of scope

Phase 9 does not include:

* Automatic conflict resolution
* Content merging
* Automatic priority rewriting
* Any new UAssetToolRivals action beyond what Phase 6 already scoped (`list_pak`, `is_iostore_encrypted`, `list_iostore_files`)

## 9.1 Retain internal asset paths from the existing classification pass

* Extend `modtype.Cache`/`SessionClassifier` to retain each classified mod's resolved internal path list alongside its `Identity`, keyed and invalidated the same way (entry ID + primary file mtime), so a conflict scan run after classification needs no new worker calls for mods already classified this session.
* Export the internal path listing currently private to `internal/modtype` (`listInternalPaths`) as a reusable entry point for resolving paths on demand, for mods not yet classified.
* Internal path lists stay backend-only: do not add them to `modtype.Identity`, since that type is Wails-bound and already ships to the frontend for every mod.
* Preserve `Determine`/`DetermineIdentity`'s existing `ErrCannotDetermineType` behavior for encrypted or otherwise unlistable IoStore containers; conflict detection must treat these the same way, as unknown rather than an error.

## 9.2 Build `internal/conflict` conflict domain logic

* New package, pure Go, independent of Wails and React, following the existing `internal/discovery`/`internal/install` boundary pattern.
* Given a `discovery.Library`'s enabled entries and their resolved internal path lists, detect internal asset paths shared by more than one mod.
* Classify each overlap as a same-priority conflict ("duplicate priority" in SPEC.md's terms) or a cross-priority overlap, comparing `discovery.Entry.Priority`'s Kind and Value together — Value alone is not enough, since `discovery.Priority` leaves Value at zero for both the leading-bang form and an entry with no recognized priority markup at all.
* Exclude disabled mods from conflict detection.
* Keep this logic distinct from `internal/install`'s destination-collision detection and from `discovery.Issue` (invalid/incomplete bundle) — do not merge these concepts.
* Cache conflict results with the same mtime-based invalidation approach used elsewhere, and invalidate correctly on rescan.

## 9.3 Wire conflict detection into the application layer

* Add Wails-bound method(s) on `App` exposing a full-library conflict scan: which mods overlap, the specific overlapping asset paths, and same-priority (duplicate-priority) vs. cross-priority classification.
* Surface a clear per-mod "unavailable" state for tool failures and undeterminable (encrypted) mods, matching the existing classification progress/failure presentation.
* Progress and cancellation for a full-library scan match the existing classification pattern exactly, which is a single synchronous call with a frontend-only busy indicator (`isClassifying` in `LibraryScreen.tsx`) — there is no backend progress-event stream or true cancellation for classification today, so 9.4's "Check for Conflicts" trigger should present the same simple busy state rather than implying finer-grained progress than the backend actually reports.

## 9.4 Wire a conflict-check trigger into the UI

* Add a "Check for Conflicts" button to the library header, matching BentoMod's `handleCheckClashes`/header-button pattern (`archive/BentoMod/bentomod/src/App.tsx:3889-3904`) as a reference for the interaction, not the visuals: an explicit, user-initiated, non-enforcing scan, never automatic and never blocking other actions.
* Add a lightweight conflict indicator to affected mods in the library catalog (`ModCatalog.tsx`) so a conflict is visible before opening the details view.
* Optionally add a per-mod "check conflicts" entry to the existing mod context menu, scoped to that one mod, matching BentoMod's `onCheckConflicts` pattern.
* This task wires the trigger and signals only; the results view itself is 9.5.

## 9.5 Build the conflict details UI

* Build a dedicated conflict details view (opened by the 9.4 trigger) that groups results by resolved character using `internal/modtype`'s existing `CharacterID`/`CharacterName`/hero-portrait resolution, reusing the same thumbnail presentation already used in `ModCatalog.tsx` — not BentoMod's raw `"Characters: 1020"` numeric-ID grouping (`archive/BentoMod/bentomod/src/main_tauri.rs:6613`, `ClashPanel.tsx`), which is a reference for what to avoid here, not a template.
* Each character group shows: hero thumbnail, resolved character name, the priority number in conflict, and the total overlapping file count for that group.
* Within a group, list each conflicting mod with a priority +/- switcher (adjusting that mod's filename-based priority through the existing Phase 4 priority mechanism) and the specific overlapping file(s) that mod is responsible for, so the user can see exactly what to change before adjusting priority.
* Present same-priority conflicts ("duplicate priority") distinctly from cross-priority overlaps.
* No resolution actions beyond the priority switcher itself (no overwrite, merge, or auto-reprioritize) — detection, presentation, and manual priority adjustment only.

## 9.6 Validate and review

* Run the canonical repository validation command (`check.ps1`).
* Build synthetic fixtures with deliberately overlapping internal paths (reusing the disposable-fixture pattern from `internal/discovery`/`internal/install` tests) covering same-priority conflicts, cross-priority overlaps, and an encrypted/undeterminable mod mixed into an otherwise conflicting set.
* Launch the app and verify the conflict details UI against a real or `C:\ModsFixtures`-scale library.
* Confirm a full-library scan remains responsive and that cancellation works.
* Create `docs/reviews/phase-9-review.md` covering all new workflows.

**Verify:** Review approval grants permission to begin Phase 10.

## Phase 9 completion report

**What changed**

Cratebug can now detect and present overlapping internal Unreal asset paths across enabled mods, on demand via a "Check for conflicts" trigger. `internal/modtype`'s classification cache now retains each mod's resolved internal path list alongside its `Identity`, so a conflict scan reuses Phase 7's already-fetched UAssetToolRivals listing instead of re-listing every mod's archive contents. A new, pure-Go `internal/conflict` package owns every conflict decision: enabled/disabled filtering, transitive grouping of overlapping mods, and same-priority ("duplicate priority") versus cross-priority classification, comparing `discovery.Priority`'s `Kind` and `Value` together. `app.go`'s `DetectConflicts` wires this into a Wails-bound call, treating tool failures and undeterminable (encrypted) mods as a clear "unavailable" result rather than a crash or a silently wrong one. The frontend gained a conflict badge in the catalog, a per-mod "check conflicts" context-menu entry, and a `ConflictDetailsDialog` that groups results by resolved character (hero thumbnail, name), shows a priority +/- switcher per participant wired to the existing Phase 4 priority mechanism, and lists each participant's specific overlapping files collapsed behind a disclosure toggle by default (added on request, since most conflicts are resolved by priority alone).

**Files changed**

Backend: `internal/conflict/` (new package: `detect.go`, `detect_test.go`, `doc.go`), `internal/modtype/cache.go` (`CachedClassification.Paths`), `internal/modtype/session.go` (`PathsForEntry`), `internal/modtype/determine.go` (exported `ListInternalPaths`), `app.go` (`DetectConflicts`, `ConflictType`), `app_test.go`. Frontend: `frontend/src/library/LibraryScreen.tsx` (`ConflictDetailsDialog` and related components), `frontend/src/library/ModCatalog.tsx` (conflict badge, context-menu entry), `frontend/src/library/entryPresentation.ts`/`.test.ts` (`characterHeroPortraitUrl`), `frontend/src/App.css`, generated Wails bindings. Docs: `docs/screenshots/phase-9/`, `docs/reviews/phase-9-review.md`, this file.

**Validation and results**

`check.ps1` passes clean (gofmt, Biome format/lint, `tsc`, vite build, `go vet`, `go test ./...`). A fresh, uncached `go test ./... -count=1` run passed all 216 tests, including 11 in `internal/conflict` (same-priority and cross-priority grouping, the leading-bang-vs-no-priority `Value`-collision case, disabled and orphaned-sidecar exclusion, an unavailable mod mixed into an otherwise-conflicting set, transitive grouping across a three-mod chain) and app-layer tests confirming a second scan of an unchanged library issues no new worker calls. `bun test` passed all 48 frontend tests. See `docs/reviews/phase-9-review.md` for the full write-up.

Manually verified against the full `C:\ModsFixtures` library (72 mods) driving the running app: a real scan returned 4 duplicate-priority groups, 1 cross-priority group, and 2 unavailable mods, with the collapsed-by-default file-count toggle expanding correctly per participant and other rows staying independently collapsed. The scan returned with no visible delay.

**Known limitations**

`DetectConflicts` is a single synchronous call with no backend progress stream or cancel endpoint, matching classification's existing precedent exactly rather than introducing new infrastructure — an intentional scope boundary recorded in this file's 9.3 section, not an oversight. A substantially larger real library's responsiveness was not separately measured beyond the 72-mod fixture scale, though `Detect`'s complexity is linear in overlap-pair count rather than library size.

**Deferred findings**

None beyond the known limitations above.

**Suggested commit message**

`chore: finalize phase 9`
