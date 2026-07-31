# Phase 1 Review

**Date:** 2026-07-30
**Status:** Ready for review

## Outcome

Phase 1 adds a deterministic, read-only Go scanner for describing a mod
library. The scanner is independent of Wails and React and does not perform
filesystem writes.

## Fixture coverage

The synthetic fixture library in
[`internal/discovery/testdata`](../../internal/discovery/testdata/README.md)
covers:

- Enabled `.pak`, `.pak_crateoff`, `.bak_bento`, and `.pak_disabled` primaries.
- Classic and complete IoStore bundles.
- Nested folders and identical stems in separate folders.
- Leading `!`, seven/eight/nine trailing nines, absent priority, and malformed
  priority names.
- Partial sidecars, orphan `.utoc`/`.ucas` files, and ambiguous primaries.

All fixtures are tiny ASCII placeholders and contain no game or mod data.

## Classification rules

- `.pak` is enabled; the three disabled extensions retain their source format.
- A primary with both `.utoc` and `.ucas` is complete IoStore; a primary with
  neither is classic; one sidecar is IoStore with a missing-sidecar issue.
- Same-stem sidecars are associated only within the same physical folder, using
  Windows case-insensitive matching while retaining original path casing.
- Entry kind, bundle format, and issues are separate fields, so incomplete and
  ambiguous conditions can be reported together.
- Same-stem primaries are returned as separate entries with an ambiguous-primary
  issue, not silently merged.
- Leading `!` and trailing runs of at least seven nines are recognized priority
  forms. The parser preserves leading-`!`, unrecognized, and absent-suffix
  distinctions even when their numeric value is zero.
- Orphan sidecars remain visible as orphaned entries with diagnostics.

## Validation

The following checks passed:

```powershell
mise exec -c "go test ./internal/discovery"
.\check.ps1
git diff --check
```

The full repository check passed Go formatting, frontend checks and build,
Go vet, and all Go tests. Discovery tests also verify missing/non-directory and
empty roots, mixed-content and nested folders, case-insensitive sidecar
grouping, every supported primary form, both incomplete sidecar combinations,
deterministic ordering, repeat scans, filesystem changes between scans, and
before/after file snapshots proving scans do not mutate fixtures.

## Manual checks

No real mod directory was scanned. That would require separate explicit
permission and is unnecessary for this phase. No UI changed, so no application
screenshot was required.

## Limitations and deferred findings

- Exact incomplete IoStore validity rules remain deferred to the planned
  UAssetToolRivals investigation.
- Inaccessible nested directories were not forced in tests because reliable
  permission denial is environment-dependent on Windows.
- The scanner currently exposes an internal Go result model only; Wails
  bindings and UI models are intentionally deferred to Phase 2.
- The Phase 1 implementation did not modify the pre-existing user change in
  `TASKS.md`.

## Review approval

**Decision:** Pending
**Reviewer:**
**Approval date:**
**Notes:** Review of terminology, fixtures, and read-only behavior is
required before Phase 2 begins.
