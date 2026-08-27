# Phase 9 Review

**Date:** 2026-08-27
**Status:** Approved

## Outcome

Phase 9 gives Cratebug the ability to detect and present overlapping internal Unreal asset paths across enabled mods, on demand, without resolving or merging anything automatically. UAssetToolRivals continues to supply only archive facts (which internal paths a mod's archive contains); a new `internal/conflict` package owns every conflict decision: enabled/disabled filtering, priority-tier comparison, transitive grouping of overlapping mods, and same-priority ("duplicate priority") versus cross-priority classification. The application layer reuses the internal path listing Phase 7's classification pass already fetched instead of re-listing every mod's archive contents a second time, and the frontend presents results grouped by resolved character with hero thumbnails, a priority +/- switcher per participant, and each participant's specific overlapping files collapsed behind a per-row disclosure toggle.

---

## Retaining internal asset paths from classification (9.1)

`internal/modtype/cache.go`'s `CachedClassification` bundles each mod's `Identity` with the `Paths` that produced it, keyed and invalidated the same way as before (entry ID + primary file mtime, in `Cache.Get`/`Put`). `SessionClassifier.PathsForEntry` (`internal/modtype/session.go:130`) resolves a cache hit into its retained path list without any new worker call. `Identity` itself (`internal/modtype/identity.go:8-14`) carries no path field, preserving the existing Wails-bound boundary — paths never leave the Go backend. `ListInternalPaths` (`internal/modtype/determine.go:55`) is the exported, reusable entry point for mods not yet classified this session. `Determine`/`DetermineIdentity`'s existing `ErrCannotDetermineType` behavior for encrypted or otherwise unlistable IoStore containers is unchanged, and `DetectConflicts` treats a missing cache entry as unavailable rather than an error (see 9.3).

## `internal/conflict` conflict domain logic (9.2)

`internal/conflict/detect.go` is a new, pure-Go package independent of Wails and React, following the existing `internal/discovery`/`internal/install` boundary pattern. Given enabled entries and their resolved path lists, `Detect` finds internal paths shared by more than one mod, forms transitively-connected groups (a chain of pairwise overlaps lands in one group, not several), and classifies each group's relationship by comparing `discovery.Priority`'s `Kind` and `Value` together — `Value` alone is not enough, since a leading-bang priority and no-priority-markup entry both leave `Value` at zero. Disabled mods are excluded before overlap detection ever runs. This logic stays distinct from `internal/install`'s destination-collision detection and from `discovery.Issue` (invalid/incomplete bundle) — neither concept is touched by this package. Caching is handled one layer up, by the existing mtime-keyed classification cache that supplies `Detect`'s path input (see 9.1); `Detect` itself is a pure function over its inputs and needs no cache of its own.

## Application-layer wiring (9.3)

`app.go:101` (`DetectConflicts`) classifies entries first — a no-op for anything already classified this session, since `SessionClassifier`'s own mtime-keyed cache short-circuits the corresponding UAssetToolRivals call — then reuses each enabled mod's retained path listing via `PathsForEntry` rather than resolving it again. Any enabled mod without a resolved path listing (tool failure, encrypted/undeterminable container) is simply left out of the `paths` map passed to `conflict.Detect`, which reports it in `Result.Unavailable` instead of crashing or silently omitting it from a group it might otherwise belong to. Progress and cancellation deliberately match the existing classification pattern exactly: `DetectConflicts` is a single synchronous call, and the frontend's `isCheckingConflicts` state is the only busy indicator, matching `isClassifying`'s existing precedent — there is no backend progress-event stream or true cancellation for conflict detection, the same as classification today. This was a scoped decision recorded in `TASKS.md` 9.3, not a gap: a finer-grained progress protocol would be new surface Phase 9 didn't set out to add, and the underlying scan (in-memory grouping over already-cached path lists) is fast enough in practice that the busy indicator is rarely visible for more than an instant.

## Conflict-check trigger and catalog indicator (9.4)

`LibraryScreen.tsx`'s "Check for conflicts" header button runs `DetectConflicts` on demand; it never runs automatically and never blocks other actions. `ModCatalog.tsx` renders a lightweight conflict badge on affected mods once a scan has run, so a conflict is visible before opening the details view.

## Conflict details UI (9.5)

`ConflictDetailsDialog` groups results by resolved character via `groupByCharacter`, reusing `internal/modtype`'s existing `CharacterID`/`CharacterName`/hero-portrait resolution and the same thumbnail presentation `ModCatalog.tsx` already uses — not a raw numeric character-ID grouping. Each character section (`ConflictCharacterHeading`) shows the hero thumbnail and resolved name; each group (`ConflictGroupCard`) shows a same-priority/cross-priority badge and the total overlapping file count. Within a group, `ConflictParticipantRow` lists each conflicting mod with a priority +/- switcher wired to the existing Phase 4 `SetModPriority` mechanism, and the specific overlapping files that mod is responsible for. That file list is collapsed behind a per-row "N overlapping files" disclosure toggle by default — added after an explicit request during this phase, since most conflicts are resolved by adjusting priority alone and the full file list previously took space every user had to scroll past whether or not they needed it. No resolution action beyond the priority switcher exists: no overwrite, merge, or auto-reprioritize.

---

## Commands and tests run

```powershell
.\check.ps1
```

```powershell
mise exec -c "go test ./... -count=1 -v"
```

```powershell
mise exec -c "bun test"
```

Validation output:

- **Go formatting (`gofmt`):** Passed with 0 changes.
- **Frontend checks:** Biome format check, Biome lint (0 errors, 0 warnings), TypeScript typecheck (`tsc --noEmit`), and the Vite production build all passed clean.
- **Go vet (`go vet ./...`):** Passed with 0 warnings.
- **Go test suite (`go test ./... -count=1`):** 216 tests passed across the repo (uncached, fresh run). `internal/conflict` (11 tests) covers same-priority and cross-priority grouping, the leading-bang-vs-no-priority Value-collision case, disabled-mod exclusion, orphaned-sidecar exclusion, an unavailable (encrypted/undeterminable) mod mixed into an otherwise-conflicting set without hiding a real overlap, and transitive grouping across a three-mod chain. Root-package tests (15) include `TestDetectConflictsFindsSamePriorityGroupAndReusesCache` (confirms a second scan of an unchanged library issues no new worker calls) and `TestDetectConflictsReportsUnavailableWithoutALiveWorker`.
- **Frontend test suite (`bun test`):** 48 tests passed across 2 files, including `characterHeroPortraitUrl`'s null-safety coverage for a missing `characterID`.

## Manual checks

1. **Live app verification (`C:\ModsFixtures`, `wails dev`):** Drove the running app in a real browser tab at the Wails-bound dev URL (`http://localhost:34115`, confirmed with a working `window.go` bridge — "Connected to backend" in console) against the full `C:\ModsFixtures` library (72 mods). Clicked "Check for conflicts" and got a real, mixed result: 4 duplicate-priority groups, 1 cross-priority group, and 2 unavailable mods, with the "2 enabled mods could not be scanned (encrypted or unreadable) and are excluded from these results" notice rendered correctly. Confirmed the Invisible Woman group's `JIRA_INVIS_SilicaSound!PrismParade_V1_Kuru` participant renders collapsed as "21 overlapping files" by default, and that clicking the toggle expands it to the individual file tags (`MI_1050308_Body.uasset`, etc.) while the sibling participant row and other groups stay independently collapsed.
2. **Responsiveness:** The scan against the full 72-mod fixture library returned and rendered the dialog with no visible delay; matches the "remains responsive" exit criterion under the synchronous-call design confirmed in 9.3.
3. **Screenshots:**
   - `docs/screenshots/phase-9/task-9.5-initial.png`
   - `docs/screenshots/phase-9/task-9.5-conflicts-collapsed.png` — dialog open, duplicate-priority and cross-priority groups visible, participant file lists collapsed.
   - `docs/screenshots/phase-9/task-9.5-conflicts-expanded.png` — same dialog with one participant's file list expanded via the disclosure toggle.

## Known limitations and deferred findings

1. **No true cancellation.** As recorded in 9.3, `DetectConflicts` is a single synchronous call with no backend progress stream or cancel endpoint, matching classification's existing precedent rather than introducing new infrastructure. This is an intentional scope boundary, not an oversight, and was verified fast enough in practice on the available fixture scale (72 mods) not to need one. A future phase revisiting classification's progress model should revisit this at the same time, since the two are wired identically.
2. **Fixture scale.** `C:\ModsFixtures` (72 mods) is the largest library available for manual verification in this environment; a substantially larger real library's responsiveness under `Detect`'s in-memory transitive-grouping pass was not separately measured, though the algorithm's complexity is linear in overlap-pair count rather than library size, so this is not expected to be a concern.

## Review approval

**Decision:** Approved. All Phase 9 tasks (9.1 through 9.5) and exit criteria are complete, verified against the `internal/conflict` unit suite, the app-layer integration tests, and the live native-bridge application against the full `C:\ModsFixtures` library, and ready for Phase 10.
