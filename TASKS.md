# Cratebug Active Tasks

**Phase:** 6 - UAssetToolRivals boundary
**Status:** Not started

This file contains only the active phase. Replace it when Phase 6 is complete.

## Phase objective

Give Cratebug a reviewed, testable integration for the narrow subset of
Unreal archive operations it actually needs, behind a typed adapter around a
pinned, prebuilt UAssetToolRivals worker release tied to a known source
revision. The default integration direction is a supervised helper process;
FFI is only pursued if a concrete performance, packaging, or operational
reason makes it necessary, and that choice is recorded as a written decision
before implementation goes further.

The worker ships as a versioned release artifact from the maintained
UAssetToolRivals fork. Normal Cratebug development and builds do not require
the .NET toolchain unless the worker itself is explicitly rebuilt from
source. Cratebug's domain API stays independent of the tool's own JSON
contract: the adapter translates worker output into Cratebug's own types, it
does not re-export the worker's contract.

This phase covers only representative read-only archive operations from the
Cratebug subset. It does not perform installation, VFX updates, or expose
the tool's complete surface.

## Exit criteria

* A written decision selects a supervised helper process by default, or
  documents the concrete reason to pursue FFI instead.
* Worker release version, source revision, and checksum are pinned and
  documented.
* Representative failures (crash, missing worker, version mismatch,
  malformed output) do not corrupt or crash Cratebug unexpectedly.
* The review records the bounded-parallelism evaluation, benchmark evidence,
  selected concurrency policy, and any decision to defer concurrency.
* Mod type is determined through a Cratebug-owned layer rather than the
  heavy archive-extraction path, with its own measured concurrency policy
  rather than one assumed from the general archive-operation benchmark.
* Production packaging works and includes the worker's licensing and
  third-party notices.
* Review approves the boundary before archive mutation (Phase 7) begins.

## Out of scope

Phase 6 does not include:

* Full installation (Phase 7)
* VFX updating
* Exposing the complete UAssetToolRivals surface through Cratebug's API
* Making the UAssetToolRivals JSON contract part of Cratebug's domain API
* Asset conflict inspection (Phase 8)
* Adopting concurrency without measured evidence that it helps

## 6.1 Decide the integration approach

* Prototype a supervised helper-process integration against the pinned
  worker: process lifecycle (start, health check, timeout, kill), structured
  request/response over its interface, and crash isolation from the Cratebug
  process.
* Compare it against FFI only if a concrete performance, packaging, or
  operational reason surfaces during prototyping; do not build an FFI path
  speculatively.
* Record the decision in `docs/decisions/0003-uassettoolrivals-boundary.md`:
  which approach was selected, why, and what would change the answer.

**Verify:** The decision document names the selected approach, the
alternative considered, and the concrete evidence (not just preference)
behind the choice.

## 6.2 Pin the worker release

* Select and pin a specific UAssetToolRivals worker release: version, source
  revision (commit hash), and a checksum verified at Cratebug build or first
  run.
* Document how to reproduce the pinned build from the maintained fork, and
  what changes require re-pinning.
* Confirm normal Cratebug development and `check.ps1` do not require the
  .NET toolchain; only an explicit worker rebuild does.
* Record licensing and third-party notices for the worker and its own
  dependencies.

**Verify:** A clean checkout can fetch and verify the pinned worker without
installing .NET, and the checksum check rejects a tampered or mismatched
binary.

## 6.3 Build the narrow typed archive-tool adapter

* Add a Go package that owns all communication with the worker: version
  checks, request construction, structured errors, and logging.
* Expose only the operations this phase needs; do not mirror the worker's
  full JSON contract into Cratebug's domain types.
* Provide test doubles so higher-level code and future phases can be tested
  without a live worker process.
* Keep the adapter independent of Wails and React, matching the existing
  scanner and mutation boundaries.

**Verify:** Unit tests exercise the adapter against a test double covering
success, malformed output, and a version mismatch, with no dependency on a
running worker process.

## 6.4 Implement the supervised helper-process integration

* Launch, monitor, and cleanly terminate the worker process from Go,
  including on Cratebug's own shutdown.
* Detect and recover from a crashed, hung, or unresponsive worker without
  crashing Cratebug or leaving an orphaned process.
* Enforce a version check against the pinned release before trusting worker
  output.
* Return specific, actionable errors distinguishing worker-launch failure,
  timeout, crash, version mismatch, and malformed response.

**Verify:** Failure-injection tests (killed process, hung process, wrong
version, malformed output) each produce a specific reported error and leave
no orphaned worker process.

## 6.5 Implement representative read-only archive operations

* Implement the small, representative set of read-only archive operations
  this phase needs (for example, internal asset path listing) through the
  adapter from 6.3.
* Validate inputs and outputs at the adapter boundary; do not pass
  unvalidated worker output further into Cratebug's domain layer.

**Verify:** Disposable fixture archives (classic and IoStore) produce
expected results through the full path: Go caller to adapter to supervised
worker and back.

## 6.6 Evaluate bounded parallel archive operations

* Benchmark sequential versus bounded-parallel archive inspection and
  archive-tool actions against representative mod libraries.
* Adopt concurrency only where measurements show it improves responsiveness
  without weakening cancellation, progress reporting, deterministic results,
  or filesystem safety; otherwise defer it and record why.
* Document the selected concurrency policy (or the decision to defer) and
  the benchmark evidence behind it.

**Verify:** Benchmark results and the resulting policy are recorded in the
Phase 6 review with enough detail (library size, timings, concurrency bound)
for the decision to be checked later.

## 6.7 Determine mod type through Cratebug's own layer

* Build a lightweight, Cratebug-owned layer for determining a mod's type
  (for example texture, mesh, blueprint, or a coarser UI-facing category)
  instead of routing it through UAssetToolRivals's full archive-parse
  actions. `docs/decisions/0003-uassettoolrivals-boundary.md`'s benchmark
  found `extract_iostore` + `detect_type` unnecessarily heavy for this, and
  that BentoMod itself avoids that path entirely for its own instant type
  display, using its own native `.utoc` header parsing and filename
  heuristics instead of calling into UAssetToolRivals for it.
* This layer may still call the adapter from 6.3 for specific cheap,
  header-only facts it needs (for example `is_iostore_encrypted`), but must
  not depend on the heavy Zen-to-legacy conversion path for routine type
  determination.
* Benchmark this layer on its own terms, the same evidence-before-policy way
  6.6 benchmarked archive operations generally: sequential versus a bounded
  worker pool, against a representative real mod library. Do not assume
  6.6's archive-operation numbers transfer unchanged — a lighter or
  pure-Go implementation may have a different concurrency profile than
  either of 6.6's two measured cases, including possibly needing no worker
  pool at all.
* Record the resulting policy (worker count, or no pooling) and the
  benchmark evidence behind it.

**Verify:** Disposable and real-library benchmarks show this layer's actual
concurrency curve, and the selected worker count (or the decision not to
pool at all) is backed by that curve rather than assumed from 6.6's
archive-operation numbers.

## 6.8 Validate the boundary and complete the review

* Run the canonical repository validation command and the adapter's
  failure-injection tests.
* Exercise the supervised worker against representative fixture archives and
  confirm crash, timeout, and version-mismatch handling does not corrupt or
  crash Cratebug.
* Confirm production packaging includes the pinned worker and its licensing
  and third-party notices.
* Create `docs/reviews/phase-6-review.md` with validation results, benchmark
  evidence, limitations, deferred findings, and review approval.

The review must record:

* The integration-approach decision and its rationale
* Worker release version, source revision, checksum, and packaging behavior
* Adapter surface and how it stays independent of the worker's own contract
* Crash, timeout, and version-mismatch handling behavior
* The parallelism evaluation, benchmark evidence, and selected concurrency
  policy
* The mod-type-determination layer's design and its own measured
  concurrency policy
* Licensing and third-party notices
* Commands and tests run
* Known limitations and deferred findings
* Review approval

**Verify:** Review approval grants permission to begin Phase 7.

## Phase 6 completion report

Report:

* What changed
* Files changed
* Validation and results
* Known limitations
* Deferred findings
* Suggested commit message

Proceed through Phase 6 as one bounded implementation pass unless a material
design decision or explicit review gate is encountered. Stop before Phase 7.
