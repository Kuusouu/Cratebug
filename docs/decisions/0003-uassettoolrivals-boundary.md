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
- Task 6.6's concurrency policy is decided below, not deferred: cap any worker pool at 4, and prefer no pooling at all for routine per-mod operations. See "Concurrency policy (task 6.6 evidence)."

## Concurrency policy (task 6.6 evidence)

Task 6.6 asks for measured evidence before adopting any concurrency policy rather than assuming more workers is better. Two prototypes ran a 1/4/8/16/32-worker pooled sweep against the same representative library — the user's real, live `~mods` folder (60 mods, read-only source) — using the same pooled-worker model: N long-lived `UAssetTool.exe` processes, each reused across its share of the 60 mods, matching what an actual Cratebug worker pool would look like rather than a fresh process per mod.

**Heavy operation** (`extract_iostore` + `detect_type` per extracted asset — full Zen-to-legacy conversion, exercising the `FZenPackageContext` reload described above): 1 worker took 3m27s; 4 workers took 1m23s (2.51x); 8 workers took 1m14s (2.81x); 16 workers took 1m11s (2.90x, the peak); 32 workers took 1m14s (2.81x, worse than 16). Real work overlaps here, so concurrency genuinely helps, but returns are clearly diminishing well before 32 cores and 32 is measurably worse than 16.

**Cheap operation** (`is_iostore_encrypted` + `list_pak` on the companion `.pak` — the only two actions a separate investigation confirmed BentoMod's own "instant" type/conflict UI actually calls into UAssetTool for; everything else in that UI is BentoMod's own native `.utoc` parsing and filename heuristics, not a tool call at all): 1 worker took 136ms for the entire 60-mod library; 4 workers took 122ms (1.12x); 8 workers took 165ms (0.83x, worse than 1); 16 workers took 266ms (0.51x); 32 workers took 565ms (0.24x, over 4x worse than 1 worker). Per-call work here is a few milliseconds, so process-spawn overhead dominates as soon as the pool grows past a handful of workers, and concurrency actively hurts.

**Decided policy: cap any worker pool at 4, and default to no pooling (1 worker) for routine per-mod operations like the cheap case above.** 4 is not the single best number for the heavy case (16 measured higher), but the gain from 4 to 16 is modest (2.51x to 2.90x) against real added cost — more concurrent self-contained .NET processes competing for memory and disk I/O — and 4 is safely inside the range where the cheap case still wins outright over 1 worker rather than losing to it. A policy that has to pick different pool sizes per operation weight is more moving parts than task 6.6's scope needs; one small bound that never regresses either measured case is preferable to chasing the heavy case's peak.

Practical implication for whichever task ends up calling these actions (most plausibly Phase 8's conflict UI): routine, cheap per-mod checks should not be parallelized at all — a single reused worker handling the whole library is both simpler and faster than any pool size measured. A bounded pool is only worth reaching for on genuinely heavy operations, and even then capped at 4, not scaled to `runtime.NumCPU()`.

This benchmarking used disposable scratch Go harnesses (pooled-worker NDJSON clients against the released `UAssetTool.exe`, not checked into this repository), run against the user's real mod library with their explicit permission, read-only throughout. Any extracted output was written to per-run temp directories and deleted after each pass.

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
