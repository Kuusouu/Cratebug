# 0003: UAssetToolRivals integration boundary

- Status: Accepted
- Date: 2026-08-25

## Context

Phase 6 needs a reviewed, testable integration for the narrow subset of Unreal archive operations Cratebug actually needs, behind a typed adapter around a pinned, prebuilt UAssetToolRivals worker. ROADMAP.md already states a default: a supervised helper process, with FFI considered only if a concrete performance, packaging, or operational reason surfaces. This decision records the investigation behind that default before any adapter code is written.

The investigation used the UAssetToolRivals submodule already initialized at `$HOME/archive/BentoMod/UAssetToolRivals`, which BentoMod tracks via `.gitmodules` as `mewclouds/UAssetToolRivals` (branch `aot`) — a fork of `XzantGaming/UassetToolRivals`. Two things came out of reading the actual source rather than assuming from the roadmap wording alone:

**Both transports already exist upstream, sharing one JSON contract.** `src/UAssetTool/UAssetTool.csproj` builds a self-contained CLI that, run with no arguments, becomes a newline-delimited JSON stdin/stdout loop (`RunInteractiveMode`, `Program.cs:2619`) — documented in the tool's own README as a "GUI Backend... JSON stdin/stdout API for frontend integration." `src/UAssetTool/UAssetToolNative.csproj` builds the same codebase as a NativeAOT library exporting one C function, `uat_invoke` (`NativeExports.cs:29`), whose own doc comment states it "reuses the exact `UAssetRequest`/`UAssetResponse` contract as the stdin interactive mode — only the transport changes." This is not a build-vs-don't-build decision: the work exists either way, and an adapter written against the shared JSON request/response shape is largely transport-agnostic by the upstream tool's own design.

**BentoMod, the closest real precedent, currently uses FFI, but that is not what its own docs say.** `uasset_toolkit/uasset_app/src/lib.rs:1-24` loads `UAssetTool.dll` via Rust's `libloading` and calls `uat_invoke` directly in-process, with an explicit code comment: "No child process, no stdin/stdout pipe... Only the transport changed." `uasset_toolkit/README.md` still describes the earlier child-process design and does not mention this migration — the crate's docs and its code disagree, and only the code was trusted here. The local BentoMod checkout is a single squashed commit ("initial commit: BentoMod v1.0.0 Rebrand"), so no commit history exists to recover *why* that migration happened; it is evidence that FFI works for BentoMod's Rust/Tauri stack, not evidence of a reason that would apply to Cratebug's Go/Wails stack.

**Only the CLI has an official prebuilt release today.** `gh release list --repo XzantGaming/UassetToolRivals` shows an actively maintained release train (latest `v1.5.6`, published the day before this investigation). `.github/workflows/release.yml` on the tracked fork publishes only the self-contained CLI for `win-x64` and `linux-x64` (`dotnet publish ... --self-contained -p:PublishSingleFile=true`); it does not build `UAssetToolNative.csproj`. `gh release view v1.5.6` confirms the Windows asset: `UAssetTool-win-x64.zip`, 31,781,186 bytes (~30.3 MB). BentoMod's own `uasset_toolkit/uasset_app/build.rs` builds the NativeAOT library from source locally (`dotnet publish UAssetToolNative.csproj`, requiring `vswhere.exe`/MSVC on `PATH` for the native link step) precisely because no prebuilt release of it exists anywhere.

**FFI would not require cgo on Cratebug's side, correcting an initial assumption.** `uat_invoke`/`uat_free` are `[UnmanagedCallersOnly]` exports — plain C-ABI exports of the kind Go already calls via `syscall.NewLazyDLL`/`NewProc` (the same mechanism used for ordinary Win32 API calls), with no cgo and no C toolchain needed in Cratebug's own build. This is not the deciding factor below, but it means the FFI option was not rejected for a reason that turned out to be false.

**Upstream's own error handling favors the process model's safety story.** `ProcessRequest` (`Program.cs:2661`) wraps its entire action dispatch in one `try`/`catch`, so a bad request or an internal exception always degrades to a normal `{success:false, message:...}` response and never crashes the interactive loop. Combined with an OS process boundary, a worker crash under the supervised-process model is a detectable exit code Cratebug can restart from. Under FFI, that same per-request exception handling still applies for managed exceptions, but there is no process boundary underneath it: a native crash inside `uat_invoke` takes the whole Cratebug process down, and a hung call cannot be safely killed the way a subprocess can. Phase 6's own exit criterion — "representative failures do not corrupt or crash Cratebug unexpectedly" — favors the model with an actual boundary.

`UAssetTool --version` / `--version` / `-v` (`Program.cs:89`) is a normal CLI argument, separate from interactive mode, giving a clean way to validate the pinned worker's version with a one-shot invocation before spawning the long-lived interactive process.

## Decision

Cratebug's UAssetToolRivals adapter uses a **supervised helper process** speaking newline-delimited JSON over stdin/stdout, driving the officially released self-contained CLI build. This matches the roadmap's stated default. No reason was found strong enough to invoke the roadmap's own override clause ("FFI comparison only when a concrete performance, packaging, or operational reason exists"): Phase 6 needs representative read-only operations, not a proven throughput bottleneck, and every packaging/build-cost consideration favors the process model today (an existing, actively maintained release pipeline versus none for the NativeAOT library).

**Alternative considered:** in-process FFI via the NativeAOT library (`uat_invoke`/`uat_free`), following BentoMod's current architecture. Not selected because it removes process-boundary crash isolation (the strongest factor against it, directly opposing Phase 6's crash-safety exit criterion), has no official prebuilt release to pin against yet, and would introduce Cratebug's first unsafe-pointer/manual-marshaling Go code where the process model needs only `os/exec` and a line-buffered reader. The lower per-call latency FFI offers is real but unneeded at Phase 6's scope.

Because both transports share one JSON contract by upstream's own design, the Go adapter (task 6.3) should still be structured behind a small interface — request in, `UAssetResponse`-shaped result out — so that if a future phase (most plausibly Phase 8's bulk conflict scanning across many mods) produces an actual measured need for lower latency, adding an FFI implementation is a transport swap, not a rewrite of call sites. That measurement, if it ever happens, is what would change this decision.

## Consequences

- Task 6.2 pins the officially released self-contained `win-x64` CLI build (currently `v1.5.6`, ~30.3 MB zipped) as the worker artifact, with its version, source revision, and checksum recorded there. No new release pipeline needs to be built for this phase.
- Task 6.3's adapter package owns process lifecycle (`os/exec`), NDJSON framing, and version verification via a one-shot `--version` call before trusting the long-lived interactive worker.
- Task 6.4's crash/hang handling can rely on normal process supervision (exit code, timeout, kill, restart) rather than needing to guard against an in-process native fault.
- The adapter's request/response types should mirror `UAssetRequest`/`UAssetResponse` closely enough that swapping in an FFI implementation later does not require changing call sites, in case Phase 8 or later produces the concrete performance reason this decision did not find.
- `uasset_toolkit/README.md` in the BentoMod archive is known to be stale relative to its own code; any future reference to that crate for precedent should re-read `lib.rs` directly rather than trust its README.
- Task 6.6's concurrency policy is decided below, not deferred: task 6.6 itself capped any worker pool at 4; task 6.7's later, wider benchmark refined this into an entry-count-tiered cap (4/8/16). See "Concurrency policy (task 6.6 evidence)."

## Concurrency policy (task 6.6 evidence)

Task 6.6 asks for measured evidence before adopting any concurrency policy rather than assuming more workers is better. Two prototypes ran a 1/4/8/16/32-worker pooled sweep against the same representative library — the user's real, live `~mods` folder (60 mods, read-only source) — using the same pooled-worker model: N long-lived `UAssetTool.exe` processes, each reused across its share of the 60 mods, matching what an actual Cratebug worker pool would look like rather than a fresh process per mod.

| Workers | Heavy op (`extract_iostore`+`detect_type`) | Speedup | Cheap op (`is_iostore_encrypted`+`list_pak`) | Speedup |
|---|---|---|---|---|
| 1 | 3m27s | 1.00x | 136ms | 1.00x |
| 4 | 1m23s | 2.51x | 122ms | 1.12x |
| 8 | 1m14s | 2.81x | 165ms | 0.83x |
| 16 | 1m11s | **2.90x (peak)** | 266ms | 0.51x |
| 32 | 1m14s | 2.81x | 565ms | 0.24x |

The heavy operation (full Zen-to-legacy conversion, exercising the `FZenPackageContext` reload described above) genuinely benefits from concurrency, peaking at 16 workers. The cheap operation gets steadily worse past 4 workers — at 60 mods, per-call work is only a few milliseconds, so process-spawn overhead dominates almost immediately.

**Correction, found while investigating task 6.7:** the cheap operation's "only two actions BentoMod calls" framing was incomplete. BentoMod's own IoStore type-display path also calls `list_iostore_files` (via `bentomod/src/utoc_utils.rs`'s `read_utoc`), not just `is_iostore_encrypted`/`list_pak` — `list_iostore_files` was never actually included in this sweep. Task 6.7's own benchmark below fills that gap.

**Decided policy at the time (task 6.6): cap any worker pool at 4.** This held up as the right call for the library size tested (60 mods) — see the entry-count-tiered policy below for how this was later refined once task 6.7 measured the same shape of operation across a much wider range of library sizes.

This benchmarking used disposable scratch Go harnesses (pooled-worker NDJSON clients against the released `UAssetTool.exe`, not checked into this repository), run against the user's real mod library with their explicit permission, read-only throughout. Any extracted output was written to per-run temp directories and deleted after each pass.

### Task 6.7 evidence: Cratebug's own type-classification layer, at four library sizes

Task 6.7 built `internal/modtype`, a Cratebug-owned category classifier (`Classify`, pure Go, no worker calls) plus an orchestration function (`Determine`) that resolves one mod's bundle format, calls `uassettool.ListPak` or (`IsIoStoreEncrypted` then `ListIoStoreFiles`) for its internal path listing, and runs `Classify` over the result. Per task 6.7's own instruction, this was benchmarked on its own terms rather than assuming the numbers above transfer, since this is a different, lighter operation shape and now genuinely exercises `list_iostore_files`, which the sweep above did not.

**A first pass at 72 entries (the real `~mods` folder) found a "no pooling" conclusion, which turned out to be a methodology artifact, not a real result.** Pool size 1 ran first in that sweep and so uniquely absorbed the one-time cost of the OS file cache being cold for every archive file being touched for the first time; every larger pool size inherited an already-warm cache from pool 1's own run, making the comparison unfair. The fix — a throwaway warmup pass at the largest pool size before any timed measurement — was applied to a full rerun across four real library sizes (the user built three synthetic-but-real-format libraries by duplicating real mod bundles, specifically to test this): the original 72-entry `~mods` folder, and three new libraries at 504, 864, and 2592 entries. Two timed passes per pool size per library; passes agreed within noise once warmup was in place, confirming the fix worked.

**Speedup vs. 1 worker, `Determine` (the actual 6.7 operation: bundle-format check, cheap listing call, `Classify`):**

| Workers | 72 entries | 504 entries | 864 entries | 2592 entries |
|---|---|---|---|---|
| 1 | 1.00x | 1.00x | 1.00x | 1.00x |
| 2 | 1.27x | 1.42x | 1.98x | 1.78x |
| 4 | **1.76x (peak)** | 2.14x | 2.98x | 3.03x |
| 8 | 1.36x | **2.36x (peak)** | 3.28x | 4.37x |
| 16 | 1.14x | 2.32x | **3.40x (peak)** | **4.68x (peak)** |
| 32 | 0.97x | 1.69x | 2.31x | 3.84x |

**Speedup vs. 1 worker, `ListPak`/`ListIoStoreFiles` alone (no classification):**

| Workers | 72 entries | 504 entries | 864 entries | 2592 entries |
|---|---|---|---|---|
| 1 | 1.00x | 1.00x | 1.00x | 1.00x |
| 2 | 1.26x | 1.69x | 1.63x | 1.88x |
| 4 | **1.63x (peak)** | **2.24x (peak)** | 2.15x | 3.28x |
| 8 | 1.41x | 2.06x | 2.32x | 4.55x |
| 16 | 0.97x | 1.76x | **2.53x (peak)** | **4.77x (peak)** |
| 32 | 0.73x | 1.28x | 1.86x | 3.64x |

**Speedup vs. 1 worker, `IsIoStoreEncrypted` alone (the cheapest of the three calls):**

| Workers | 72 entries | 504 entries | 864 entries | 2592 entries |
|---|---|---|---|---|
| 1 | 1.00x | 1.00x | 1.00x | 1.00x |
| 2 | 1.07x | 1.68x | 1.54x | 1.61x |
| 4 | **1.24x (peak)** | **1.95x (peak)** | **1.98x (peak)** | 2.26x |
| 8 | 1.20x | 1.52x | 1.36x | **2.81x (peak)** |
| 16 | 0.84x | 1.32x | 1.83x | 1.93x |
| 32 | 0.76x | 1.05x | 1.14x | 1.84x |

Absolute 1-worker baselines, for scale: `Determine` took 176ms/331ms/555ms/1196ms sequentially at 72/504/864/2592 entries respectively; `discovery.Scan` itself (the plain filesystem walk, no worker involved) took 2ms/15ms/27ms/80ms at the same sizes and is not a bottleneck at any size tested. Category output was identical across every pool size once warmup was applied, confirming pooling changes only timing, not results; the small "per-entry error" counts at each size are expected `ErrCannotDetermineType` results for encrypted IoStore mods, not failures — confirmed by the `IsIoStoreEncrypted` table above showing zero errors for the same entries, since checking whether something is encrypted always succeeds.

**Two findings that reshape the task 6.6 policy:**

1. Pooling genuinely helps even at 72 entries once the cold-cache confound is removed — task 6.6's cheap-operation sweep and task 6.7's first 72-entry pass were both measuring against an unfair pool-1 baseline. It just doesn't matter much there: the gain is real but shaves well under 100ms off an already-imperceptible operation.
2. The pool size that actually peaks grows with library size — 4 is enough at 72-504 entries, but 864-2592 entries want 8-16 workers to capture the full benefit. 32 workers never wins at any size or operation measured; the ceiling sits somewhere between 16 and 32 at every scale tested.

**Decided policy: entry-count-tiered pool sizing**, replacing the fixed cap of 4:

| Entry count | Pool cap | Basis |
|---|---|---|
| < 700 | 4 | Peak observed at 72 and 504 entries across all three operations |
| 700 – 1,499 | 8 | 864-entry peak was split between 8 and 16 depending on operation; 8 is the safer floor of that range |
| ≥ 1,500 | 16 | Peak observed at 2592 entries across all three operations; 32 regressed at every size tested |

Implemented as `internal/uassettool.WorkerPoolSizeForLibrary(availableCores, entryCount)` / `DefaultWorkerPoolSizeForLibrary(entryCount)`, layered on the same core-aware halving logic `WorkerPoolSize`/`DefaultWorkerPoolSize` already used for task 6.6's fixed cap of 4 (which those two functions keep doing, unchanged, for callers that don't know their entry count up front). The threshold boundaries (700, 1,500) are interpolated between the four measured sizes, not measured exactly; a wider spread of measured library sizes could refine them further. `internal/modtype` still ships no pooling code of its own — callers with a large library should use `WorkerPoolSizeForLibrary` to size their own worker pool around `Determine`.

This benchmark used a disposable scratch Go harness, temporarily placed inside the Cratebug module tree (Go's `internal/` import visibility rule requires this — a harness outside the module cannot import `internal/discovery`, `internal/uassettool`, or `internal/modtype` regardless of `replace` directives), run once per configuration, deleted immediately after, read-only throughout against the user's real and duplicated-real-format mod libraries with their explicit permission.

## Sources

- `UAssetToolRivals/src/UAssetTool/Program.cs` (interactive mode loop, version handling, `ProcessRequest` exception boundary)
- `UAssetToolRivals/src/UAssetTool/NativeExports.cs` (FFI surface and its own doc comment describing the transport-only change)
- `UAssetToolRivals/src/UAssetTool/UAssetTool.csproj` and `UAssetToolNative.csproj` (self-contained CLI vs. NativeAOT library build settings)
- `UAssetToolRivals/.github/workflows/release.yml` (official release scope)
- `UAssetToolRivals/README.md` ("Interactive JSON Mode" section)
- `uasset_toolkit/README.md` and `uasset_toolkit/uasset_app/src/lib.rs` (BentoMod's current FFI implementation and its stale README)
- `uasset_toolkit/uasset_app/build.rs` (local NativeAOT build requirements)
- `.gitmodules` (submodule fork and tracked branch)
- `gh release list --repo XzantGaming/UassetToolRivals` and `gh release view v1.5.6 --repo XzantGaming/UassetToolRivals` (release cadence and Windows asset size, checked 2026-08-25)
