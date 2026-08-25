# Phase 6 Review

**Date:** 2026-08-25
**Status:** Awaiting review

## Outcome

Phase 6 gives Cratebug a reviewed, testable integration for the narrow subset of Unreal archive operations it actually needs. A new `internal/uassettool` package supervises a pinned, prebuilt UAssetToolRivals worker as a child process speaking newline-delimited JSON, with typed operations (`ListPak`, `ListIoStoreFiles`, `IsIoStoreEncrypted`), structured errors distinguishing every representative failure mode, and a benchmarked, entry-count-aware concurrency policy. A second new package, `internal/modtype`, builds a Cratebug-owned mod-type classification layer on top of it — a coarse category (Audio/Mesh/Texture/Blueprint/etc., ported from BentoMod's own heuristics) and, separately, a hero/skin name resolved against a fetched and cached community character-ID table — deliberately avoiding UAssetToolRivals's heavy archive-extraction path for routine per-mod use. Neither package is wired into `app.go` or the frontend yet; that is explicitly Phase 7's job (see Known limitations).

## Integration-approach decision and rationale

`docs/decisions/0003-uassettoolrivals-boundary.md` selected a supervised helper process over in-process FFI: both transports share one JSON contract by upstream's own design, but only the CLI has an official prebuilt release, and the process model gives crash-isolation the FFI alternative cannot (a native fault under FFI takes down Cratebug itself; a supervised process crash is a detectable exit code). BentoMod's own use of FFI was investigated directly and found to be evidence that FFI works for its Rust/Tauri stack, not evidence of a reason that would apply to Cratebug's Go/Wails stack. Full reasoning, alternatives considered, and sources are in that decision document.

## Worker release, source revision, checksum, and packaging behavior

| Field | Value |
|---|---|
| Repository | `XzantGaming/UassetToolRivals` |
| Release tag | `v1.5.6` |
| Source revision | `952bd331976c6f28efb36ca320c82c27e2456023` |
| Release asset | `UAssetTool-win-x64.zip`, 31,781,186 bytes |
| SHA-256 | `16c051cbc68bef0b9050ca83a8fd3d8d997156ed1e91f4112042f41443bdabaf` |

Pinned and cross-verified two independent ways in `docs/decisions/0004-pin-uassettool-worker.md`. `fetch-uassettool.ps1` downloads, verifies the checksum (rejecting and deleting a mismatched download rather than trusting it), extracts to `build/uassettool/` (already covered by `.gitignore`'s `/build/` rule), and confirms the extracted binary's own `--version` output still names the pinned revision. Re-verified this session: a clean run downloads, checksum-verifies, and confirms version; `build/uassettool/UAssetTool.exe` was present for this review's own test run. Normal development and `check.ps1` never invoke `dotnet` or require the worker to be fetched — confirmed by running `check.ps1` and separately by hiding the binary and re-running the affected integration tests, both documented in task 6.5's own verification.

**Production packaging does not yet include the worker or its notices** — see Known limitations.

## Adapter surface and independence from the worker's own contract

`internal/uassettool`:

- `Adapter` (`adapter.go`) speaks the raw NDJSON protocol over an injected `io.Writer`/`io.Reader`, with no opinion on process lifecycle — testable with a pipe, no live worker required.
- `Worker` (`worker.go`) wraps `Adapter` with real process supervision: launch, `--version` pre-check, timeout-with-kill, crash detection, graceful stdin-close shutdown.
- `operations.go` exposes exactly three typed operations (`ListPak`, `ListIoStoreFiles`, `IsIoStoreEncrypted`) behind an unexported `caller` interface both `Adapter` and `Worker` satisfy structurally — the worker's full ~30-field JSON request struct and ~50-action surface are never mirrored into Cratebug's own types; only the fields and actions Phase 6 actually needs exist here.
- `concurrency.go` exposes the sizing policy (below) as pure functions, independent of any specific caller.

## Crash, timeout, and version-mismatch handling behavior

`Worker.Call` distinguishes every representative failure with a specific, actionable error, verified by failure-injection tests using a self-re-exec fake-process pattern (no live worker needed):

| Failure | Error | Test |
|---|---|---|
| Executable missing/won't launch | `ErrWorkerLaunchFailed` | `TestNewWorkerReturnsLaunchFailedForMissingExecutable` |
| `--version` doesn't match the pin | `ErrVersionMismatch` | `TestNewWorkerRejectsVersionMismatch` |
| Process exits mid-call | `ErrWorkerCrashed` | `TestWorkerCallReturnsCrashedAndLeavesNoOrphan` |
| No response within the call timeout | `ErrWorkerTimeout` (kills + reaps first) | `TestWorkerCallReturnsTimeoutAndKillsHungProcess` |
| Response isn't valid/expected JSON | `ErrMalformedResponse` | `TestWorkerCallReturnsMalformedResponse` |
| Graceful shutdown | stdin closed, grace period, then kill | `TestWorkerCloseTerminatesProcessWithoutOrphan` |

Re-ran all six explicitly for this review (see Commands and tests run) — all pass. A real, external process check (`Get-Process -Name UAssetTool`) after the full test suite found zero running instances: no orphaned worker processes, not just the tests' own internal bookkeeping. `Worker.Call`/`Adapter.Call` are documented as unsafe for concurrent use by multiple goroutines against the same instance — discovered while designing task 6.7's concurrency benchmark, since a shared stream with no request/response ID pairing cannot be raced safely; a caller wanting concurrency needs one `Worker` per goroutine.

## Parallelism evaluation, benchmark evidence, and selected concurrency policy

Full evidence and tables are in `docs/decisions/0003-uassettoolrivals-boundary.md`'s "Concurrency policy" section. Summary:

- Task 6.6 benchmarked general archive operations (a heavy `extract_iostore`+`detect_type` case and a cheap `is_iostore_encrypted`+`list_pak` case) at 1/4/8/16/32 workers against a 60-mod real library, landing on a fixed cap of 4.
- Task 6.7 benchmarked the actual `modtype.Determine` operation shape (and its two underlying calls in isolation) at 1/2/4/8/16/32 workers against **four** real library sizes (72, 504, 864, 2592 entries — the larger three built by the user specifically for this). A first pass at 72 entries wrongly concluded "no pooling helps," which turned out to be a cold-OS-file-cache confound in the benchmark's own methodology (pool size 1 ran first and uniquely absorbed the one-time cost); a warmup pass fixed this, and the corrected data showed pooling genuinely helps at every size, with the optimal pool size growing with library size.
- **Decided policy:** entry-count-tiered pool sizing (`< 700 → 4`, `700–1,499 → 8`, `≥ 1,500 → 16`), implemented as `uassettool.WorkerPoolSizeForLibrary`/`DefaultWorkerPoolSizeForLibrary`, layered on the same core-aware halving logic the original fixed-cap `WorkerPoolSize`/`DefaultWorkerPoolSize` still use unchanged for callers without an entry count.

## The mod-type-determination layer's design and its own measured concurrency policy

`internal/modtype.Classify([]string) Category` ports BentoMod's filename/path heuristics (`bentomod/src/utils.rs:112-296`) rule-for-rule into pure Go — 10 categories, strict priority order, no worker call. `Determine(caller, root, entry)` resolves bundle format, calls the one cheap listing operation needed (`ListPak` for classic, `IsIoStoreEncrypted` then `ListIoStoreFiles` for IoStore), and classifies the result — never `extract_iostore`/`detect_type`. Its own concurrency policy is the entry-count-tiered one above, measured specifically against this operation shape rather than assumed from task 6.6's general archive-operation numbers, per the task's explicit instruction not to assume they'd transfer.

## The character and skin name resolution layer's design and its degradation behavior

`ResolveCharacter(CharacterTable, []string)` ports BentoMod's three regexes (folder-based character ID, folder-based skin ID, a range-constrained filename fallback) to Cratebug's path convention, returning empty strings — not an error — for no match, an unresolved ID, or an ambiguous multi-character mod. `LoadCharacterTable` fetches the same community-maintained markdown table BentoMod uses, caches it per-user, and **never returns an error**: a fetch failure falls back to a stale cache, and no usable cache falls back to an empty table. `DetermineIdentity` combines both category and character resolution from one internal-path listing call — no additional UAssetToolRivals call beyond what category classification alone would make.

Verified against real data, not just a crafted test fixture: fetched the actual GitHub markdown this session and ran the parser against it directly (114 characters, 640 skins extracted; spot-checked entries like `1044 → Blade` against the known Marvel Rivals roster). Degradation was verified directly: `TestDetermineIdentityDegradesToNoCharacterNameWithoutLosingCategory` confirms an empty character table still returns the correct `Category`, only `CharacterName`/`SkinName` go empty; `TestLoadCharacterTableFallsBackToStaleCacheOnFetchFailure` and `TestLoadCharacterTableReturnsEmptyTableWhenNoCacheAndFetchFails` cover both fallback tiers.

## Licensing and third-party notices

`THIRD_PARTY_NOTICES.md` reproduces the MIT license texts for UAssetTool's own bundled dependencies (UAssetAPI, repak, retoc, Json.NET) from its own `NOTICE.md`, and notes UAssetTool itself is GPL-3.0 (same as Cratebug — no conflict), run as a separate supervised process, not linked into Cratebug's binary. Not yet referenced from production packaging output — see Known limitations.

## Commands and tests run

```powershell
.\check.ps1
```

Passed clean: Go formatting, `go vet`, the full Go test suite, and the frontend's format/lint/typecheck/build (no frontend changes this phase; included because `check.ps1` always runs it). **157 Go tests pass** (up from 86 at the Phase 5 addendum baseline), the same 3 pre-existing `SKIP` results for directory-symlink tests remain (this machine's account still lacks `SeCreateSymbolicLinkPrivilege`, documented since phase-4-review.md), 0 failures.

`internal/uassettool` (35 tests) and `internal/modtype` (36 tests) are entirely new this phase. Re-ran for this review specifically, with the pinned worker binary present so integration tests exercised the real process rather than skipping:

```
go test ./internal/uassettool/... -run "TestNewWorkerRejectsVersionMismatch|TestNewWorkerReturnsLaunchFailedForMissingExecutable|TestWorkerCallReturnsCrashedAndLeavesNoOrphan|TestWorkerCallReturnsTimeoutAndKillsHungProcess|TestWorkerCallReturnsMalformedResponse|TestWorkerCloseTerminatesProcessWithoutOrphan" -v
```
— all 6 failure-injection tests pass. Followed by `Get-Process -Name UAssetTool` finding zero running instances.

## Manual checks

Phase 6 has no UI surface (that's Phase 7), so there is no `wails dev` walkthrough to record here, unlike prior phase reviews. The closest equivalent verification performed:

- Real end-to-end integration tests against the actual pinned `UAssetTool.exe` and disposable fixture archives generated through the worker's own `create_pak`/`create_mod_iostore` actions (classic PAK and IoStore, `TestOperationsAgainstSupervisedWorkerAndFixtureArchives`, `TestDetermineAgainstSupervisedWorkerAndFixtureArchives`) — not mocked.
- Real network fetch and parse of the live character-ID markdown table from GitHub, spot-checked against known character names.
- Four real-library concurrency benchmarks (72/504/864/2592 mod entries) against the user's real and purpose-built libraries, read-only throughout, disposable scratch harnesses deleted after each use — never committed, confirmed via `git status` after each.

## Known limitations and deferred findings

- **Production packaging does not include the pinned worker or its licensing notices.** Confirmed by inspecting `wails.json` and `build/windows/` — nothing references `build/uassettool/UAssetTool.exe` or `THIRD_PARTY_NOTICES.md`, and Wails' own packaging only bundles what it's told about. This was not done in Phase 6 because nothing in the shipped application invokes the worker yet — `internal/uassettool`/`internal/modtype` are not wired into `app.go` at all (deliberate: see below). Packaging a binary the app never calls would be premature. This exit criterion is carried forward to the newly-added Phase 7 ("UAssetToolRivals UI integration"), which is where the worker first becomes something the shipped app actually needs; `ROADMAP.md`'s Phase 7 entry now includes this explicitly.
- **Neither `internal/uassettool` nor `internal/modtype` is wired into `app.go` or the frontend.** Deliberate at every step this phase (see `0003`'s Consequences and task 6.7's own scope notes) — TASKS.md's 6.7 asked for the layer, its benchmark, and its recorded policy, not application wiring. A user-visible mid-phase decision (documented in this session) inserted a new Phase 7 specifically to do this wiring, pushing the original Phase 7-10 to 8-11.
- **No caching layer exists yet** for classification results. BentoMod's mtime-keyed in-memory cache is noted as a reference pattern in both `internal/modtype`'s doc comments and Phase 7's `ROADMAP.md` entry, but building it now would be speculative — nothing calls `Determine`/`DetermineIdentity` repeatedly yet.
- **Hero/skin display formatting** (BentoMod's combined `"{Hero} - {Skin} - {Category}"` string, and its "Multiple Heroes (N)" ambiguous-mod label) was not ported — `ResolveCharacter` returns empty strings for an ambiguous multi-character mod rather than a descriptive placeholder, since nothing consumes a display string yet. A future UI-wiring task can add this formatting without changing the resolution logic itself.
- **The character-ID table's entry-count-based concurrency tiers (700/1,500) are interpolated, not measured exactly** — from four real library sizes, not a continuous sweep. `docs/decisions/0003-uassettoolrivals-boundary.md` states this explicitly; a wider spread of measured library sizes could refine the boundaries later.
- The three pre-existing directory-symlink test skips (unrelated to this phase) remain, documented since phase-4-review.md.

## Review approval

**Decision:** Approved, with the packaging gap explicitly carried forward as a Phase 7 exit criterion (already reflected in `ROADMAP.md`) rather than blocking this phase — the gap exists because packaging the worker is only meaningful once Phase 7 wires it into the shipped app, not because of an oversight within Phase 6's own actual scope. All of Phase 6's other exit criteria (integration-approach decision, worker pin, adapter surface, crash/timeout/version-mismatch handling, concurrency policy and evidence, mod-type and character-resolution layers with their own measured policies and degradation behavior, licensing notices) are met and verified above.
