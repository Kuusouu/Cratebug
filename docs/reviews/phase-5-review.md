# Phase 5 Review

**Date:** 2026-08-22
**Status:** Awaiting review

## Outcome

Phase 5 lets Cratebug persist settings, tags, and mod identity across sessions,
and keeps that persisted state safe against interrupted writes, corrupted
files, and schema drift. A new `internal/metadata` package owns the storage
format, safe writes, corrupt-file recovery, schema migration, and the
persistent mod identity that tags attach to; `app.go` wires it into the
existing Wails boundary and into the Phase 4 mutation flow so a rename,
priority change, or move re-points a mod's tags instead of orphaning them.
The frontend gained a "Tags..." context-menu action (extending the pattern
from
[0002-organize-action-pattern](../decisions/0002-organize-action-pattern.md))
and restores the last-used mod root automatically on launch.

## Storage format, schema version, and safe-write/recovery behavior

- One JSON document (`metadata.Document`) is persisted at
  `%AppData%\Cratebug\metadata.json`, carrying an explicit `schemaVersion`
  field (`internal/metadata/store.go`).
- `Store.Save` writes atomically: content goes to a temporary file in the
  same directory first, then `os.Rename` replaces the primary in one step, so
  an interrupted write never leaves the primary partially written. Before
  replacing the primary, `Save` copies its current contents to a
  `.bak` last-known-good backup.
- `Store.Load` cannot fail. A missing primary returns a fresh document. A
  primary that is unreadable, fails to parse, or declares a schema version
  newer than this build supports is quarantined (renamed to `.corrupt`,
  content untouched) rather than discarded, and `Load` falls back to `.bak`.
  If the backup is unusable too, `Load` falls back to a fresh document. A
  successful recovery is written back to the primary path so later loads do
  not repeat the recovery. `Load` returns a `Recovery{Recovered, Cause}`
  value alongside the document so callers can surface what happened.

## How persistent mod identity is derived and reconciled

- The scanner's entry ID (folder + stem + kind) changes whenever a mod is
  renamed, reprioritized, or moved, so it cannot anchor persisted metadata by
  itself. `internal/metadata/identity.go` adds a second, opaque identity
  (`mod-<hex>`) stored in `Document.Mods`, keyed independently of the current
  scanner ID. `EnsureMod` creates or looks up that identity from a scanner ID;
  same-named mods in different folders get distinct scanner IDs and therefore
  distinct persistent identities.
- `App.executeAndReconcile` (`app.go`) wraps `RenameMod`, `SetModPriority`,
  and `MoveMod`: after a successful mutation, it loads the metadata document,
  calls `ReconcileMod(result.PreviousID, result.ID)` to re-point the mod
  record at its new scanner ID, and saves. `SetModEnabled` does not need this
  wrapping because enabling or disabling a primary does not change its
  scanner ID.
- **Known limitation:** `RenameFolder` and `MoveFolder` are not wrapped, because
  their `Result` reports only the folder's own old and new paths, not a
  per-mod ID pair for every mod the folder carries. Tags on mods inside a
  renamed or moved folder are not reconciled. This mirrors the equivalent,
  already-accepted frontend limitation from
  [phase-4-review.md](phase-4-review.md#known-limitations-and-deferred-findings)
  (folder rename/move clears rather than remaps the selection) and was not
  solved here because Phase 4's `Result` type would need to change to close
  it, which is out of this task's scope.

## Tag and settings behavior, including cross-restart persistence

- `internal/metadata/tags.go` implements a tag catalog (`CreateTag`,
  `RenameTag`, `DeleteTag`, case-insensitive duplicate-name rejection) and
  per-mod assignment (`AssignTag`, `UnassignTag`, both idempotent) keyed to
  the persistent mod identity, not the current filename.
- `App` exposes `LoadMetadata`, `SetModRoot`, `CreateTag`, `RenameTag`,
  `DeleteTag`, `AssignModTag`, and `UnassignModTag`. The frontend
  (`LibraryScreen.tsx`) loads persisted metadata once on launch, restores the
  saved mod root and scans it automatically instead of requiring
  reselection, and persists the mod root again after every successful scan.
- Tags are reachable from the mod context menu's new "Tags..." item, which
  opens a checklist dialog (`ModTagDialog`) that assigns or unassigns a tag
  immediately per checkbox toggle, plus an inline "New tag" field that
  creates and assigns in one action. Assigned tags are shown on the selected
  mod's status panel.
- Manually verified end to end against a disposable fixture library
  (see Manual checks): mod root and tag assignment both survived a full
  reload (a `wails dev` browser-tab reload restarts only the frontend, not
  the long-lived Go backend, so this specifically confirms the settings and
  tags round-tripped through the real on-disk file, not an in-memory cache;
  the raw file content on disk was inspected directly and matched
  expectations before and after).

## Corrupt-data, orphaned-metadata, and migration behavior

- `Document.OrphanedMods(liveScannerIDs)` (`internal/metadata/identity.go`)
  reports mod records whose scanner ID is absent from a fresh scan, without
  removing them, so metadata is not lost if the mod reappears (for example,
  after reselecting a folder). This task did not wire a UI or App binding
  for it; see Deferred findings.
- Schema migration (`internal/metadata/migrations.go`) reads the document as
  a raw JSON map first, applies registered `migrationStep`s in sequence until
  the version reaches `CurrentSchemaVersion`, and only then decodes into
  `Document`. A migration step that fails to advance the version is treated
  as an error (guards against an infinite loop from a future migration bug).
  One migration is registered: schema version 0 (no `schemaVersion` field at
  all, e.g. a hand-edited or pre-existing file) stamps version 1; no other
  migration exists yet because version 1 is the only format Cratebug has
  ever written.
- Live-verified: corrupting the real running app's `metadata.json` to
  invalid JSON and reloading did not crash the app or block it from starting.
  It scanned the library normally, quarantined the corrupt file to
  `metadata.json.corrupt` (confirmed byte-identical to the corrupted
  content), restored `metadata.json` from `metadata.json.bak`, and surfaced
  a warning toast identifying the cause
  ("Cratebug found a problem with your saved settings and recovered them
  from a backup: parse metadata file ... invalid character 'n' ...").

## Commands and tests run

```powershell
.\check.ps1
```

This ran Go formatting, frontend format/lint/typecheck/build, `go vet`, and
the full Go test suite: 77 tests passed across `internal/metadata`,
`internal/discovery`, `internal/mutation`, and the root `main` package. Three
pre-existing `SKIP` results remain for directory-symlink tests, unrelated to
Phase 5 (documented in
[phase-4-review.md](phase-4-review.md#commands-and-tests-run); this
machine's account still lacks `SeCreateSymbolicLinkPrivilege`).

`internal/metadata` alone contributes 30 of those tests, covering: atomic
writes and backup maintenance (`store_test.go`), persistent identity and its
survival through real `mutation` package rename/priority/move calls
(`identity_test.go`), the tag catalog and assignment including a real
rename-and-move sequence (`tags_test.go`), corrupted/truncated/unsupported
files, orphan detection, and recovery not discarding unrelated valid data
(`recovery_test.go`), and schema migration including the unversioned case
and a rejected pre-migration version (`migrations_test.go`).

A real bug surfaced only through this test suite and was fixed before
merging: `ReconcileMod` originally replaced a mod's whole record on
reconciliation, silently discarding its `Tags`; `schemaVersionOf` originally
recognized only `float64` (encoding/json's numeric type) and misread a
migration's own freshly-set `int`, tripping the anti-infinite-loop guard on
every migration.

## Manual checks and screenshot paths

All manual verification used a disposable fixture library created under the
session scratchpad directory (two synthetic `.pak` files, one nested), never
a real Marvel Rivals mod directory. It and the app's own dev-mode
`metadata.json`/`.bak`/`.corrupt` files were deleted after verification.

Performed via `mise exec -c "wails dev"`, driving the app at
`http://localhost:34115` and inspecting the DOM (accessibility tree and text
content) plus the real on-disk metadata file, rather than pixel screenshots
(see Known limitations):

- Scanned the fixture library; selected a mod; opened its context menu and
  confirmed a "Tags..." item is present alongside Rename/Priority/Move/Delete.
- Opened the tag dialog on an untagged mod: confirmed the "No tags yet.
  Create one below." empty state.
- Created a tag ("Combat") via the dialog's inline field: confirmed the
  success toast, the new checked checkbox in the dialog, and the tag chip
  appearing on the selected-mod panel.
- Unchecked and rechecked the tag's checkbox: confirmed the corresponding
  "Removed"/toast and chip-disappears / chip-reappears behavior.
- Reloaded the page (restarting only the frontend, not the long-lived Go
  backend): confirmed the mod root field auto-filled, the library
  auto-scanned, and the "Combat" chip was still present on the same mod,
  reading from the real persisted file rather than in-memory state.
- Corrupted the real `metadata.json` to invalid JSON and reloaded: confirmed
  no crash, a normal scan, and the recovery warning toast described above;
  confirmed on disk that `metadata.json` was restored, `metadata.json.bak`
  was intact, and `metadata.json.corrupt` held the exact original corrupted
  bytes.
- Renamed the tagged mod, then changed its priority, then moved it to a
  different folder (three separate operations): confirmed the "Combat" chip
  remained visible immediately after each one, with no page reload. The
  first attempt at this surfaced the frontend staleness bug described above
  (the chip disappeared after rename because the frontend's cached metadata
  was not refreshed); after the fix, all three operations kept the chip
  visible without a reload.

## Known limitations and deferred findings

- **No pixel screenshots.** The available browser tooling in this session
  could not composite frames (`the Browser pane is not displayed, so the
  page is not compositing frames`), and the repository's configured
  `playwright` MCP server was not connected in this session. All UI
  verification above is DOM-level (accessibility tree, text content) and
  on-disk state, not visual. Layout, spacing, color, and typography for the
  new tag dialog and chips were not visually confirmed, only functionally
  confirmed to render the expected content. The new CSS reuses established
  classes and patterns (`.mutation-dialog`, `.mod-facts`-style pill chips)
  rather than introducing new layout, which lowers but does not eliminate
  the risk.
- Folder rename/move does not reconcile tags for the mods it carries (see
  above); this is an accepted extension of Phase 4's existing
  folder-selection limitation, not a new gap.
- `Document.OrphanedMods` has no App binding or UI surface yet; task 5.4
  built the detection capability and task 5.6 did not add a UI for it, since
  neither task's scope explicitly required one. A future task should decide
  how orphaned metadata is surfaced (for example, when a folder is
  reselected and previously-tagged mods are missing).
- No UI exists for renaming or deleting a tag from the catalog
  (`RenameTag`/`DeleteTag` exist on the backend and are tested, but
  TASKS.md's 5.6 scope named only creating, assigning, and removing tags on
  a mod).
- Batch operations, permanent deletion, archive installation, asset conflict
  inspection, BentoMod import, and general settings beyond the mod root
  remain out of scope for this phase, per TASKS.md.

## Review approval

**Decision:** Approved. See the addendum's own visual verification pass below,
which exercised this review's persistence, identity, and recovery behavior
directly in the running app in addition to the addendum's own UI.

---

# Addendum: UI overhaul (settings, tags, cards, visual identity, drag-and-drop)

**Date:** 2026-08-23
**Status:** Awaiting review

## Outcome

The Phase 5 review above was accepted functionally, but the user rejected the
frontend it shipped with as "really bad design": tags were reachable only
through a per-mod context-menu dialog, there was no settings UI despite the
backend already supporting persisted settings, and the header was a single
letter "C" in a gradient box. This addendum covers everything built in
response, using a plan drafted during that conversation (not checked into
this repository — a Claude Code session artifact, not a project file) as the
starting point. Two tracks ran beyond that plan's original scope —
an accent-color theming system and folder/mod drag-and-drop — both added
mid-stream at the user's request rather than planned upfront; this addendum
documents them alongside the planned work rather than pretending the plan was
followed to the letter. No `TASKS.md`/`ROADMAP.md`/`SPEC.md` changes were
needed, matching the plan's own assessment that SPEC.md's BentoMod-reference
mandate (sections 14 and 16) already covered this work.

Every user-visible step in this pass was shown to the user for their own look
before the next one started, per their standing hard-stop requirement — this
addendum is written after that full review cycle, not instead of it.

## Settings UI, accent-color theming, and self-hosted fonts

- `internal/metadata/store.go`'s `Settings` struct gained `Theme`,
  `DefaultViewMode`, and `AccentColor` (all `omitempty`, purely additive, no
  schema-version bump). `internal/metadata/settings.go` validates each
  server-side (`SetTheme`, `SetDefaultViewMode`, `SetAccentColor`), since
  Wails bindings are callable from devtools, not just the intended UI.
  `AccentColor` accepts a 6-digit hex string or an empty string, which
  doubles as the "reset to theme default" path. `app.go` exposes matching
  scalar methods, mirroring the existing load-mutate-save shape.
- **Deviation from the plan:** the plan specified buffered Save/Cancel for
  `SettingsDialog.tsx`, mirroring BentoMod. The user asked for immediate
  apply instead once they saw the mockup ("most of these settings should
  apply immediately"), matching `ModTagDialog`'s existing precedent. The
  shipped dialog has no Save/Cancel pair, only Close; every control (theme,
  accent) applies on click, with local optimistic state reverted only on a
  save failure.
- **Deviation from the plan:** "Default view mode" was dropped from Settings
  entirely, also per user feedback — the existing toolbar view-mode buttons
  now persist silently on every click instead, so whatever the user last
  picked is what the catalog opens to next time.
- Theme is a 3-icon picker (System/Light/Dark), not BentoMod's cycling
  toggle — the user's own preference, "like how discord has it."
- Accent color (not in the original plan, added after the wordmark
  discussion below) is a 5-preset-plus-custom-hex picker
  (`frontend/src/library/accentColor.ts`), applied via an inline
  `--accent`/`--accent-ink` override on `.app-shell` so it composes with
  existing theme tokens rather than replacing them. `contrastingInk()`
  computes a readable ink color for arbitrary accents via a perceived-
  brightness threshold, since the app's own `--accent-ink` is only tuned for
  each theme's specific default. Typing a hex applies live on every
  keystroke but persists on a 400ms debounce, so it isn't writing to disk on
  every character. The cream "Crate" preset became the actual theme default
  for both light and dark (`App.css`'s `.app-shell` token blocks), not just
  a selectable option, at the user's request.
- Inter and Bungee are both self-hosted now (`@font-face` in `style.css`),
  where Inter previously had none at all — every user was silently getting
  their OS default sans-serif. Both ship as raw `.ttf`, not `.woff2`,
  documented in the bug below.

## Tag filter and catalog management

- `frontend/src/library/TagMenu.tsx`: a toolbar popover next to the search
  input, listing the tag catalog with per-tag filter toggling (OR semantics),
  inline rename/delete, and a create-tag form. `RenameTag`/`DeleteTag`
  existed on the backend since Phase 5 with zero frontend callers; this
  wires them up. Deleting a tag currently in the active filter also clears
  it from the filter.
- `usePositionedPopover.ts` extracts `ContextMenu.tsx`'s viewport-clamped
  positioning and outside-pointerdown/Escape/scroll/resize dismissal into a
  shared hook, used by both components instead of a second copy.
- `LibraryScreen.tsx` gained `tagFilterIDs` state and a `tagsByEntryID` memo
  (one pass over every mod record per metadata change, resolved against the
  tag catalog once) — a deliberate perf choice distinct from the existing
  `tagIDsForScannerID` helper, which is fine for the single-selection panel
  but would be O(entries × mods) if reused per card on every render.

## Card redesign

Tags-on-cards was planned; the rest of the redesign below was not — it came
from the user's direct reaction to a screenshot of the first cut ("the top
cards are bigger still... looks disproportionate").

- `ModCatalog.tsx` cards show up to 4 tag chips with a "+N" overflow
  indicator, each chip removable inline (`UnassignModTag`, immediate, not
  confirmed — unassigning only edits Cratebug's own metadata, never mod
  files).
- The status-dot + "DISABLED" badge + Enable/Disable button trio was
  replaced with a single `role="switch"` toggle — the three were saying the
  same thing three ways. Rename/priority/move/delete stay context-menu-only,
  unchanged.
- A `.mod-thumbnail` placeholder (a generic package glyph in a bordered
  square) reserves the layout space a real mod thumbnail would occupy later;
  `internal/discovery` has no icon-scanning capability today, so this is
  reserved space for a feature that doesn't exist yet, not a hidden one.
- The `.brand-mark` header icon became an inline crate SVG (rounded square,
  lid line, corner bracing) in a neutral, non-accent-tinted treatment (muted
  border + subtle `color-mix()` background) rather than the old orange
  gradient box. The same mark, filled and rendered at each standard Windows
  icon size, became the actual app icon (`build/appicon.png`,
  `build/windows/icon.ico` — both gitignored, generated via a hand-rolled
  multi-resolution ICO writer since Wails only regenerates `icon.ico` when
  the file is absent and has a known bug where it doesn't always propagate a
  replaced `appicon.png` even then).

## Folder and mod drag-and-drop

Not in the original plan at all — added afterward at the user's request,
after confirming the existing `MoveFolder`/`MoveMod` mutation operations
already did the necessary work and only needed a new frontend trigger.

- `FolderNavigation.tsx`: sidebar folders are draggable onto other folders
  (including onto "All mods" to move to the library root), gated behind the
  existing cycle check (a folder cannot be dropped into itself or its own
  descendant).
- `ModCatalog.tsx`: mod cards are draggable onto sidebar folders, gated
  behind the existing `canOrganizeMod` predicate (the same one already
  guarding Rename/Priority/Move in the context menu).
- The drag state (`DraggedItem`, a `{type: "folder"|"mod", ...}` union) and
  its validity check (`isValidDropTarget`) both live in
  `frontend/src/library/libraryTypes.ts`, shared by `LibraryScreen` (which
  owns the drag and decides whether to execute the move), `FolderNavigation`
  (the drop target), and `ModCatalog` (a second drag source) — the state had
  to move up from a first draft that kept it local to `FolderNavigation`,
  once mods needed to be draggable from a sibling component.

## Real bugs found and fixed

- **WOFF2 rendering bug in WebView2/DirectWrite.** The Bungee wordmark's
  woff2 build reported `status: "loaded"` via the CSS Font Loading API and
  laid out with correct metrics, but silently painted a different font's
  glyphs — confirmed by measuring the real element's shrink-wrapped width
  against controlled references before concluding it was genuinely
  mis-rendering, not a caching artifact. The identical file rendered
  correctly in a non-Windows test browser. Root cause narrowed to Windows'
  DirectWrite text rasterizer specifically mishandling this file's woff2
  encoding; the raw `.ttf` source doesn't have the problem. Both bundled
  fonts ship as `.ttf` because of this, not for file-size reasons. Two
  smaller, real (if less severe) findings surfaced during the same
  investigation and are documented in code comments where they were fixed:
  `<h1>`'s default bold weight combined with `font-synthesis: none` silently
  falls through to the next font in the stack when the custom font doesn't
  register that weight (`App.css`'s `.wordmark` rule), and this codebase's
  test sandbox has its own unrelated, non-standard font-fallback-list
  resolution quirk that was initially and incorrectly suspected as the root
  cause before the woff2 theory was confirmed against the user's real
  environment.
- **CSS Grid stretch-space redistribution.** When `.mod-grid` stretched every
  card in a row to match the tallest one (a card carrying tags), a card's
  own leftover vertical space was distributed across its internal grid
  tracks in a way that pushed the enable/disable toggle away from the tag
  row with an inconsistent gap, and in one case let tag chips render past
  the card's own border into the row below. Fixed by switching `.mod-card`
  from `display: grid` to `display: flex; flex-direction: column`, whose
  leftover-space rule is simple and consistent across browsers: unclaimed
  space goes after the last child, not redistributed across siblings.
- **React memo subtree cascade.** `FolderTreeItem`'s memo comparator checked
  only whether a row's own exact path matched the current drag state. Since
  the component also recursively renders its own children, a parent that
  judged itself "unchanged" skipped re-rendering its whole subtree —
  including a child whose drag-over highlight genuinely needed to update.
  This made dragging over subfolders feel unresponsive while top-level
  folders worked fine, since top-level folders have no such parent to get
  blocked behind. Fixed by making the comparator check the whole subtree a
  node roots, the same pattern `selectionTouchesBranch` already used
  correctly for the selected-folder highlight.

## Commands and tests run

```powershell
.\check.ps1
```

86 Go tests pass (up from 77 at the Phase 5 baseline — the 9 new tests are
`internal/metadata/settings_test.go`'s theme/view-mode/accent-color
validation and persistence-round-trip cases), the same 3 pre-existing `SKIP`
results for directory-symlink tests remain (unrelated, documented above).
`bun run check` (Biome format/lint, `tsc --noEmit`, `vite build`) is clean.

## Manual checks

Performed via `mise exec -c "wails dev"` against `C:\ModsFixtures` (the
repo's standing fixture library, per the updated `AGENTS.md` guidance this
addendum's work also added — a small varied set of Classic/IoStore bundles
across nested folders, replacing the old per-task disposable-fixture-only
guidance since this addendum's testing needed a folder hierarchy to
exercise). As in the Phase 5 review, this session's Browser tooling could
not composite frames, so verification is DOM-level (accessibility tree,
computed styles, measured element geometry) and on-disk state, not pixel
screenshots — with one exception: raster icon previews
(`build/appicon.png` and a scaled-up 16×16 render) were inspected visually
via the Read tool's image support, since those are static generated files
rather than live browser-rendered UI.

Notably, most of the deepest verification in this pass was adversarial
measurement rather than passive observation — confirming the WOFF2 bug and
the memo cascade bug both required comparing a real rendered element's
geometry against deliberately-constructed reference elements, because
surface-level checks (`document.fonts.check()`, the presence of a CSS class)
reported success in both cases despite the underlying behavior being wrong.

Every UI-visible change was shown to the user directly in the running app
between steps, per their hard-stop requirement; several rounds of this
surfaced real regressions before they shipped (the card-height issue above,
the settings-dialog spacing, the font weight, all fixed in the same pass
they were found).

## Known limitations and deferred findings

- The original plan's exclusion table mostly still holds (self-update,
  launcher picker, processing-speed settings, hero icons/backgrounds, help
  tour, bypass-game-running-lock — all still excluded for the reasons
  already stated there). Two entries changed status: the **accent color
  picker**, listed as "Deferred" in the original table, was built in this
  pass at the user's request; **hold-to-delete** and **"Auto Detect" game
  path** remain deferred, unchanged.
- The Classic/IoStore badge two-tint treatment, listed as optional/low-
  priority in the original plan, was also built (`bundle-format-badge`
  classes in `ModCatalog.tsx`/`App.css`).
- Screenshots still were not captured for the same environment reason as the
  Phase 5 review; see Manual checks above for how this was mitigated.
- The `wails dev` session in this addendum exhibited HMR staleness several
  times (stale CSS/JS not propagating to the native WebView2 window even
  after a hard refresh, requiring a full process restart) — none of it was
  a real regression once verified against a clean reload, but it repeatedly
  cost verification time and is worth knowing about for future sessions
  rather than immediately trusting a "still broken" report without checking
  whether a restart (not just a refresh) resolves it first.

## Visual verification pass (screenshots)

**Date:** 2026-08-25

The prior passes above could not capture pixel screenshots (no compositing
browser). This pass fixed that: the repo's `playwright` MCP server had no
browser binary installed (`~/AppData/Local/ms-playwright` did not exist), so
`mise exec -- bunx @playwright/mcp@latest install-browser chrome-for-testing`
was run once and the server reconnected on session restart. All screenshots
below are real WebView2/Wails-served renders at `http://localhost:34115`,
driven against `C:\ModsFixtures` (the running `wails dev` instance was
temporarily repointed from the user's real Marvel Rivals folder to fixtures
for this pass, with the user's explicit approval, and repointed back
afterward). Screenshots are in `docs/screenshots/phase-5/`.

- `task-5.6-library-baseline.png` — compact-view catalog on fixtures.
- `task-5.6-settings-default.png`, `task-5.6-settings-dark-teal.png` —
  Settings dialog; confirmed System/Light/Dark and all six accent options
  (five presets plus custom hex) apply immediately and live, including to
  the dialog's own chrome.
- `task-5.6-tagmenu-empty.png`, `task-5.6-tagmenu-created.png` — toolbar tag
  catalog popover empty state and after creating two tags ("Favorites",
  "WIP"), including the success toast.
- `task-5.6-mod-context-menu.png`, `task-5.6-tagdialog-checklist.png` —
  mod context menu showing "Tags..." alongside Rename/Priority/Move/Delete,
  and the resulting checklist dialog.
- `task-5.6-card-tag-chip.png` — assigned "Favorites" tag rendering as a
  removable chip on both the card and the selected-mod panel.
- `task-5.6-tag-filter-active.png` — toolbar tag filter narrowing the
  catalog to the one tagged mod, with the active filter count badge.
- `task-5.6-dragdrop-mod-result.png` — a mod card dragged onto a sidebar
  folder, confirmed via the resulting toast and updated folder counts.
- `task-5.6-dragdrop-folder-result.png` — a folder dragged onto another
  folder (nesting), confirmed via toast and the sidebar tree collapsing to
  the new structure.
- `task-5.6-dragdrop-tag-reconcile.png` — the tagged mod dragged to a new
  folder; the "Favorites" chip survived the move. This specifically
  exercises `App.executeAndReconcile` through the new drag-and-drop trigger
  added in this addendum, which the addendum's own manual checks had only
  exercised through the context-menu Rename/Priority/Move dialogs, not drag.
- `task-5.4-corrupt-recovery-toast.png` — corrupting the real
  `metadata.json` and reloading: no crash, the same recovery toast text
  documented above, and the library scanned normally.
- `task-5.6-large-view.png`, `task-5.6-list-view.png` — the other two
  catalog view modes, for layout consistency with the compact view above.

**Finding, not a regression:** the corrupt/recovery test above was run after
two prior drag-and-drop moves in the same session. The restored `.bak` was
one save generation behind the live state (by design — `Store.Save` copies
the *previous* primary to `.bak` before writing the new one), so it still
described the tagged mod at its pre-move scanner ID. `App.executeAndReconcile`
only re-points a mod record when a mutation call is made through it
(Rename/Priority/Move); a plain load-then-scan after a backup restore has no
equivalent step. The result: the tag silently disappeared from the UI (not
from the underlying data — it remains in `metadata.json` under the old
scanner ID) with no indication to the user. This is not a new gap: it is a
concrete, reproduced instance of the limitation already disclosed above
("`Document.OrphanedMods` has no App binding or UI surface yet"). It does not
violate the Phase 5 exit criteria (metadata is retained, not discarded), but
it's worth recording that this specific failure mode — a recovery-restored
backup racing an already-applied filesystem mutation — is how that gap
actually surfaces to a user, as a tag that appears to vanish rather than an
explicit "orphaned" state. A future task closing the `OrphanedMods` UI gap
should use this as a concrete repro case.

## Follow-up fix: surface orphaned tagged mods

**Date:** 2026-08-25

Closes the finding above: a tagged mod record whose scanner ID no longer
matches anything in a fresh scan (whether from a stale backup restore, a
folder-rename/move gap, or otherwise) previously vanished from the UI with
no indication, even though `internal/metadata/identity.go`'s
`Document.OrphanedMods` already detected exactly this case — task 5.4 built
the detection but no task wired it to the frontend.

- No Go changes. `MetadataState`/`LoadMetadata` already return the full
  `Document`, including every mod record's `scannerID` and `tags`, and a
  scan's `discovery.Library.entries` already carry each entry's `id`
  (`json:"id"`, `internal/discovery/scan.go`). Recomputing the orphan set
  from data the frontend already holds avoids a second filesystem scan or a
  new Wails binding, so `internal/metadata/identity.go`'s existing
  `OrphanedMods` logic is duplicated in TypeScript rather than called
  directly — the two are intentionally the same one-line membership check.
- `frontend/src/library/LibraryScreen.tsx`: `orphanedTaggedModCount` counts
  mod records that carry at least one tag but whose `scannerID` is absent
  from the current scan's entry IDs (records with no tags are excluded,
  since nothing user-visible would be missing for those). A `useEffect`
  depending on `[library, metadataDocument]` recomputes this on every scan
  or metadata change and shows a warning toast through the existing
  `showMutationFeedback` path when the count changes to something nonzero.
  A ref (`lastOrphanedTagNoticeCountRef`) suppresses re-showing the same
  count, so routine metadata reloads (assigning a tag, for example) do not
  repeat the notice, and resets on an empty scan so a later scan can notify
  again.
- **First draft had a bug, caught before landing:** the effect's initial
  version called `orphanedTaggedModCount` directly inside `scan()`, which
  closes over `metadataDocument` from the render that created that specific
  `scan` closure. On the automatic first-launch scan, that closure was
  created before the mount effect's `setMetadataDocument` call resolved, so
  it always saw `metadataDocument` as `null` and never fired. Moving the
  check into its own `useEffect` over `[library, metadataDocument]` fixes
  this regardless of which of the two resolves first.
- **Two grammar bugs in the first live checks, also fixed:** the singular
  case initially read "1 tagged mod record no longer match anything"
  (subject-verb disagreement) from one pluralized template covering both
  cases, and the plural case's second sentence initially read "...in case
  the mod reappears" (singular noun for a message about multiple records).
  Split into an explicit singular/plural pair of full sentences — "Its tags
  are kept in case the mod reappears" versus "Their tags are kept in case
  the mods reappear" — instead of trying to interpolate one template both
  ways.

**Verify:** `.\check.ps1` passed (Go tests, `go vet`, Biome format/lint,
`tsc`, `vite build`) after each change. Manually reproduced against
`C:\ModsFixtures`: reused the same orphaned "Favorites" tag record from the
finding above (still present in `metadata.json` at its stale pre-move
scanner ID), reloaded the app, and confirmed the toast "1 tagged mod record
no longer matches anything in this scan. Its tags are kept in case the mod
reappears." appears
(`docs/screenshots/phase-5/task-5.4-orphan-notice.png`). Clicking "Scan
library" again with no state change did not re-show the toast, confirming
the dedup ref. No fixture files were modified; the underlying orphaned
record was left in place afterward since it is pre-existing test debris
from the finding above, not new data this fix created.

## Review approval

**Decision:** Approved.
