# Cratebug Active Tasks

**Phase:** 7 - UAssetToolRivals UI integration
**Status:** Complete. Approved.

This file contains only the active phase. Replace it when Phase 7 is complete.

## Phase objective

Give Cratebug's library UI a working presentation of each mod's determined
type/category and, where resolvable, its hero and skin name — backed by the
`internal/uassettool` and `internal/modtype` packages Phase 6 built but did
not wire into the application. This phase launches and supervises the
pinned worker for the lifetime of a session, exposes classification through
the Wails-bound API, caches results so repeated scans stay cheap, and
packages the worker binary into production builds for the first time.

Classification must never block the library's initial render, and must
never turn into a visible error or crash: anything Cratebug cannot
classify (worker unavailable, encrypted container, incomplete bundle)
degrades to a clearly labeled "Unknown" state instead.

## Exit criteria

* Mod type/category renders correctly for representative real and
  disposable-fixture libraries.
* Classification does not block the initial library render; the catalog
  appears immediately and categories populate progressively.
* Encrypted or otherwise undeterminable mods show a clear "unknown" state,
  never an error or a crash.
* Caching avoids redundant worker calls for mods unchanged since the last
  classification.
* Production packaging and the installer include the pinned worker binary
  and its third-party notices.
* Review approves the UI presentation and responsiveness against a large
  real library.

## Out of scope

Phase 7 does not include:

* Full installation (Phase 8)
* Asset conflict inspection (Phase 9)
* VFX updating
* Exposing any UAssetToolRivals surface beyond what Phase 6 already scoped
* Batch operations, filesystem watching, or anything else in ROADMAP.md's
  "Deferred post-release work"

## 7.1 Wire the worker pool's per-worker lifecycle into app.go

* Add a production pinned-revision constant (for example
  `uassettool.PinnedSourceRevision`) instead of duplicating the value that
  currently only exists in test files, `fetch-uassettool.ps1`, and docs —
  keep it in sync with `docs/decisions/0004-pin-uassettool-worker.md`.
* Resolve the worker executable path with a small helper checked in this
  order: (1) an env var override, for example `CRATEBUG_UASSETTOOL_PATH`,
  useful for tests and for a dev/prod override without recompiling; (2)
  `<directory containing os.Executable()>/uassettool/UAssetTool.exe` (the
  production/installed layout task 7.6 creates); (3)
  `build/uassettool/UAssetTool.exe` relative to the working directory (the
  development layout `fetch-uassettool.ps1` produces, and `wails dev` runs
  from the repo root, so a relative path works there). Return a clear,
  specific error if none exist — 7.2 turns that into "Unknown," not a
  crash.
* This task covers **one worker's** lifecycle only: launch via
  `NewWorker`, detect death via `Alive()`, close via `Close()`. Task 7.2
  covers how many of these App keeps alive at once and how work is
  distributed across them — do not decide pool size here.
* Add `OnShutdown` to `main.go`'s `wails.Run` options and close every
  worker the pool (7.2) is holding, so no process is ever left running
  after Cratebug exits.
* Any worker-launch failure (missing executable, version mismatch, crash)
  must degrade the affected entries to category `Unknown`, never surface
  as an error to the frontend.

**Verify:** Deleting or renaming the pinned worker executable still lets
Cratebug scan and browse a library normally; classification for that
session simply comes back `Unknown` instead of failing the scan.

## 7.2 Build the classification cache and a session-lived worker pool

* Add an in-memory cache (not persisted — this is derived data, not user
  data like `metadata.Store`) keyed by entry ID plus the primary file's
  mtime. `discovery.Entry` does not carry an mtime field (checked
  `internal/discovery/scan.go` — the only `os.Stat` call there is for the
  mod root, not per-entry files), so this means one extra `os.Stat` per
  entry per classify call, on the entry's resolved absolute path. Resolve
  that path the same way `internal/modtype/determine.go`'s own (unexported,
  not importable) `absPath` does —
  `filepath.Join(root, filepath.FromSlash(entry.PrimaryPath))` — `app.go`
  needs its own small equivalent, not a reuse of that one. Cheap, but not
  something already sitting on the entry for free. Note this key is *not*
  stable across rename, move, or priority
  changes: `app.go`'s `executeAndReconcile` comment confirms those
  mutations assign a new scanner ID to the affected mod(s) — and for
  `RenameFolder`/`MoveFolder` specifically, to *every* mod inside that
  folder, not just one, per that same comment ("a folder rename or move
  changes the scanner ID of every mod it contains"). Affected mods will
  miss the cache once and be reclassified; that is fine (still bounded by
  the folder's contents, not the whole library, and each call is cheap
  per 0003's own benchmark) and does not need solving — do not build a
  more elaborate content-stable key for this. See 7.4 for the separate,
  more important case of `RenameMod`/`SetModPriority`, which change an
  entry's ID *without* triggering a rescan at all.
* Keep the pool alive for the session, not spun up and torn down per
  call. `ClassifyLibrary` is called after every `ScanLibrary`, and
  `ScanLibrary` runs more often than just initial load — `LibraryScreen.tsx`'s
  `reloadLibrary()` also calls it after `MoveMod`, `DeleteMod`,
  `CreateFolder`, `RenameFolder`, and `MoveFolder` (checked directly in
  the frontend code, not assumed). Even so, these are still explicit,
  human-paced user actions, not a hot loop — a warm session-lived pool
  plus the cache above (which keeps a mutation-triggered reload cheap,
  since only the mutated entry misses) is still clearly better than
  spinning up a fresh pool of worker processes on every one of them, which
  would repeatedly pay the process-spawn overhead the 0003 benchmark found
  dominates small workloads.
* Size the pool by the **cache-miss count for that call**, not the total
  entries requested: `uassettool.WorkerPoolSizeForLibrary(runtime.NumCPU(),
  len(misses))`. On the first `ClassifyLibrary` call of a session the
  cache is empty, so misses equal total entries and this is just the
  library's full size — but on a later call after one mutation-triggered
  reload (see the bullet above), most of a large library is still cached,
  and only the mutated mod(s) actually miss. Sizing by total entries there
  would resize a perfectly good warm pool down to a handful of workers (or
  up to 16 for one changed mod), discarding it for no reason and undercutting
  the whole point of keeping the pool alive. If a call's miss count lands
  in a different size tier than the pool's current size (see
  `internal/uassettool/concurrency.go`'s thresholds), close the pool and
  relaunch it at the new size rather than trying to grow/shrink it live —
  resizing is rare enough that simplicity wins here.
* Structure the pool as a job queue, not a fixed 1:1 assignment: a
  buffered channel of per-entry jobs, N goroutines each owning exactly one
  `*uassettool.Worker` (never shared — `Worker.Call` is documented as
  unsafe for concurrent use by multiple goroutines) pulling jobs and
  writing results to a results channel.
* Do not try to make two truly concurrent, overlapping `ClassifyLibrary`
  calls safe against each other on the Go side — a resize (previous
  bullet) closing the `jobs` channel while another call is still sending
  to it panics ("send on closed channel"), a hard crash. Prevent the
  overlap at the source instead: guard the frontend call site the same
  way this codebase already guards every mutation
  (`isMutationLocked`/`mutatingEntryIDs` in `LibraryScreen.tsx` — see
  7.3) so a second classify request cannot fire while one is still in
  flight. Keep a mutex around pool creation/resize in Go anyway, as cheap
  defense-in-depth, but the frontend guard is what actually prevents the
  hazard, not the mutex alone.
* On a worker's `Alive() == false`, replace just that one worker in the
  pool before it takes its next job — do not tear down the whole pool for
  one crashed process.
* Classify only cache misses; merge cache hits and freshly classified
  results into the returned map.
* Any per-entry error (encrypted container, missing sidecar, worker
  failure) degrades that one entry to `Category: "Unknown"` with empty
  hero/skin names; it must not fail the whole batch.

**Verify:** Classifying the same unchanged library twice in one session
only invokes the worker for entries not already cached — a unit test with
a fake `caller` (no live worker) can assert call counts directly. Run the
pool's own tests with `go test -race` to confirm the cache map and pool
state have no data races *within* one call's fan-out/fan-in (goroutines
writing results concurrently) — this does not require, and should not
require, two full `ClassifyLibrary` calls to run safely against each
other, since 7.3's frontend guard is what prevents that from happening in
practice.

## 7.3 Expose classification through the Wails-bound API

* Add an `App` method the frontend calls after `ScanLibrary` resolves, for
  example `ClassifyLibrary(modRoot string, entries []discovery.Entry)
  (map[string]modtype.Identity, error)`, keyed by entry ID.
* Load the character table once per session
  (`modtype.LoadCharacterTable`, `modtype.DefaultCharacterTableCachePath`),
  not on every call.
* Guard the frontend call site against firing a second `ClassifyLibrary`
  while one is still in flight, the same way `LibraryScreen.tsx` already
  guards every mutation (`isMutationLocked`/`mutatingEntryIDs`) — see
  7.2's pool-resize hazard this prevents.
* Run `mise exec -c "wails dev"` after adding the method so
  `frontend/wailsjs/go/main/App.d.ts` picks up the new binding before 7.4
  tries to call it.

**Verify:** The generated TypeScript binding for the new method appears in
`frontend/wailsjs/go/main/App.d.ts`, and calling it from the running dev
app against a real or fixture library returns a populated result map.

## 7.4 Render category and hero/skin name in the library UI

* Call the new classification method from `LibraryScreen.tsx` right after
  a successful `ScanLibrary`, without blocking the catalog's initial
  render — the scan result renders first, classification results merge in
  once they arrive.
* Render the resolved category (and hero/skin name when present) on each
  mod entry across every view mode (`compact`, `large`, `list` — see
  `frontend/src/library/libraryTypes.ts`), following the same
  presentation-helper pattern as `entryPresentation.ts` rather than
  inlining the logic into `ModCatalog.tsx`.
* Keep the classification map's keys in sync with entry ID changes that
  do **not** go through a rescan: `RenameMod` and `SetModPriority` both
  call `LibraryScreen.tsx`'s `updateMutatedEntry`, which swaps an entry's
  `id` from `result.previousID` to `result.id` locally, with no
  `ScanLibrary` call afterward. Without a matching fix, a renamed or
  reprioritized mod's category/hero silently disappears from the UI until
  the next full rescan, since the classification map is still keyed by
  the old ID. The mod's archive contents did not change, so the fix is
  cheap: re-key that one classification result from `previousID` to `id`
  locally, the same way `metadata`'s tag reconciliation
  (`app.go`'s `executeAndReconcile`) already does for tags — do not
  re-invoke the worker for this case.

**Verify:** Screenshot each view mode against a real or fixture library
per `AGENTS.md`'s UI verification steps, comparing against BentoMod's
equivalent display as a reference, not a template. Rename a mod and
change its priority in the running app; its category/hero name must
still be visible immediately after, not blank until the next rescan.

## 7.5 Hero thumbnail asset pipeline and UI rendering

* Include `characterID` in `modtype.Identity` (and `ResolveCharacter` / `DetermineIdentity`) so the frontend receives the resolved 4-digit Hero ID alongside the hero/skin names.
* Source/bundle hero portrait assets (in `frontend/src/assets/heroes/<id>.png`) and provide a reproducible script (`scripts/fetch-hero-portraits.ps1` or similar) to document/automate how hero portrait assets are obtained or updated.
* Render the resolved hero portrait image inside `.mod-thumbnail` on mod cards when `identity.characterID` is available and has a corresponding asset, falling back to the package icon for non-hero or unclassified mods.
* Keep the hover tooltip working seamlessly on top of the hero image thumbnail.

**Verify:** Mod cards for character mods display their corresponding hero portrait icon in the thumbnail area; non-character mods and unclassified mods cleanly fall back to the package icon.

## 7.6 Add progressive loading and a graceful "unknown" presentation

* Show a lightweight loading indication (for example a skeleton or muted
  placeholder) on each mod's category/hero area between the catalog's
  initial render and the classification call resolving.
* Present `Category: "Unknown"` (and empty hero/skin) as a clearly
  labeled, non-alarming state — never an error banner, never a crash.

**Verify:** A library containing an encrypted IoStore mod (or any entry
`Determine`/`DetermineIdentity` cannot resolve) shows that mod as
"Unknown" without any error state appearing elsewhere in the UI.

## 7.7 Package the pinned worker binary and its notices into production builds

* Decide where the installed worker lives relative to `Cratebug.exe` (a
  `uassettool/` subfolder next to the installed executable is the natural
  choice, mirroring `build/uassettool/`'s dev layout) and make 7.1's path
  resolution agree with it.
* Extend the Windows build/install step (`wails build -nsis ...`, see
  `README.md`'s "Windows installer" section) so the pinned
  `UAssetTool.exe` and `THIRD_PARTY_NOTICES.md` are included in both the
  unpackaged build output and the NSIS installer. Wails supports a custom
  `build/windows/installer/project.nsi` template for adding extra
  installed files; none exists in this repo yet, so this task creates it.
* Ensure all Git LFS assets in general (hero and skin portrait images, fonts,
  and binary assets) are properly bundled and resolved into the executable/build
  and packaging pipeline so production builds and standalone artifacts are self-contained.
* Do not commit the worker binary — the extracted `UAssetTool.exe` is
  ~73 MB (per `docs/decisions/0004-pin-uassettool-worker.md`; do not
  confuse this with the ~30 MB *zip download* size mentioned there, a
  different number for a different file). The packaging step should pull
  the extracted `.exe` from wherever `fetch-uassettool.ps1` already places
  it (`build/uassettool/`, already gitignored) — CI or a release script
  needs to run that script before packaging.

**Verify:** A clean `wails build -nsis` run (after `fetch-uassettool.ps1`)
produces an installer that, once installed, has `UAssetTool.exe` and
`THIRD_PARTY_NOTICES.md` present next to the installed `Cratebug.exe` —
confirmed by inspecting the installed directory, not just the installer's
file list.

## 7.8 Validate the integration and complete the review

* Run the canonical repository validation command (`check.ps1`).
* Launch the app (`mise exec -c "wails dev"`), scan a real or fixture
  library, and confirm categories and hero/skin names render correctly and
  progressively, per `AGENTS.md`'s UI verification steps.
* Confirm the packaged installer includes the worker and its notices
  (7.7's Verify).
* Create `docs/reviews/phase-7-review.md` following the format of
  `docs/reviews/phase-6-review.md`.

The review must record:

* The worker lifecycle design (lazy launch, crash recovery, shutdown) and
  where the executable path is resolved for dev versus production
* The classification cache's design and eviction/invalidation behavior
* The Wails-bound API surface added and its shape
* How progressive loading and the "unknown" state were implemented and how
  they were verified
* Packaging changes and how the installer's contents were confirmed
* Commands and tests run, including manual UI checks and screenshot paths
* Known limitations and deferred findings
* Review approval

**Verify:** Review approval grants permission to begin Phase 8.

## Phase 7 completion report

Report:

* What changed
* Files changed
* Validation and results
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 7 as one bounded implementation pass unless a
material design decision or explicit review gate is encountered. Stop
before Phase 8.
