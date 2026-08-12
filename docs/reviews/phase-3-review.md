# Phase 3 Review

**Date:** 2026-08-12  
**Status:** Approved

## Outcome

Phase 3 adds the first filesystem mutation: a discovered mod can be enabled or
disabled through a Go-owned operation that validates the current scan, plans a
no-replace primary-file rename, and enforces game-running protection before it
changes the filesystem. React requests the operation and updates only the
affected catalog entry from the returned result.

## Supported behavior

- Disable an enabled `.pak` to Cratebug's `.pak_crateoff` format.
- Enable `.pak_crateoff`, BentoMod `.bak_bento`, and legacy `.pak_disabled`
  primary files back to `.pak`.
- Preserve `.utoc` and `.ucas` sidecars during a primary-file transition.
- Reject ambiguous primaries, destination collisions, missing scanner entries,
  invalid state transitions, and paths outside the selected mod root.
- Block every declared unsafe mutation while `Marvel-Win64-Shipping.exe` is
  running. The backend applies this restriction before filesystem mutation.
- Use scanner-issued stable entry IDs for React keys, operation requests, and
  targeted catalog reconciliation. An ID survives Cratebug's supported primary
  suffix transition.

## Manual interaction coverage

The running development application was checked against `C:\ModsTest`, a
user-designated disposable test library.

- Copies 10 through 15 were disabled as `.pak_crateoff`; their IoStore sidecars
  remained present on disk.
- The scanned catalog showed Copies 10 through 15 as disabled with Enable
  controls, and later `.pak` entries as enabled with Disable controls.
- A temporary `Marvel-Win64-Shipping.exe` process was used to exercise the
  game-running rejection. The UI retained the catalog and displayed the
  actionable error without changing the selected mod.
- Busy-state screenshot evidence was explicitly skipped by user decision. A
  single local rename completes too quickly for a reliable manual capture; no
  artificial production delay was added solely for documentation.

## Validation

The following command passed after the final Phase 3 changes:

```powershell
.\check.ps1
```

It ran Go formatting, frontend formatting and linting, TypeScript checking,
the Vite production build, Go vet, and all Go tests.

Focused mutation tests cover supported state transitions, sidecar preservation,
rejected plans, destination races, game-running safety, and stable entry IDs.
Automated tests use `t.TempDir()` fixtures; no real Marvel Rivals mod directory
was targeted by automated testing.

## Screenshots

- [Enabled and disabled catalog states](../screenshots/phase-3/task-3-6-disabled-mods.png)
- [Game-running error state](../screenshots/phase-3/task-3-6-game-running-error.png)

## Limitations and deferred findings

- Phase 3 intentionally supports only single-mod enable and disable. Batch
  actions, selection, and progress aggregation are deferred.
- A busy state exists for an in-flight operation, but visual evidence was not
  captured because the local rename completed too quickly and the user chose
  not to add an artificial test delay.
- Entry IDs are deterministic scanner identities, not persisted metadata.
  They survive supported primary suffix changes; an external rename is treated
  as a newly discovered entry on the next full scan.
- Priority changes, general rename or move operations, deletion, installation,
  metadata persistence, and archive inspection remain outside this phase.

## Review approval

**Decision:** Approved by user on 2026-08-12
**Notes:** Phase 3 is complete. Phase 4 may begin after its active task plan is
established.
