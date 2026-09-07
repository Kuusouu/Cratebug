# Phase 15 Review

**Date:** 2026-09-06
**Status:** Approved

## Outcome

Phase 15 adds two first-load offers and one new library mutation.

Cratebug now detects installed and incoming mods whose companion `.pak` still contains the IoStore bookkeeping names `chunknames` or `patched_files`, which crash Marvel Rivals anti-cheat as of 3 September 2026. Detection is by `list_pak`, not filesystem scan, because those names live inside the archive. The fix rewrites only the `.pak`: extract the remaining files, `create_pak` without the metadata names, replace the live primary. `.utoc` and `.ucas` are never opened for write.

After classify, Cratebug also detects complete unencrypted IoStore mods whose retained listing includes any path outside `/Game/Marvel/Characters`. Those will not load unencrypted, so the first populated load of a root offers to encrypt them. Those rebuilds run through a `NewWriteWorker` pool sized by `DefaultWorkerPoolSizeForLibrary` rather than through the classification pool, which uses a short timeout and may be busy.

Both offers are once per library root per session. Refresh and post-mutation reload do not re-prompt, and dismissal is not persisted. The companion offer settles first when both apply.

Decision 0006 records the pak-only rewrite and the detect-by-listing rule. The 0005 addendum records the write-worker pool and the required-encryption offer.

---

## Docs and decisions (15.1)

`ROADMAP.md` names Phase 15 and states both halves in its Outcome. `SPEC.md` section 6 adds the two warnings to core workflows, section 11 adds companion PAK cleanup to the game-running block list, and section 15 states the detection rule, the pak-only rewrite, the install-time strip, the `/Game/Marvel/Characters` rule, and that library encrypt and decrypt run through a write-timeout worker pool rather than the classification pool.

`docs/decisions/0006-companion-pak-cleanup.md` records detect-by-`list_pak`, the pak-only rewrite with a `.cratebug-companion-stub` entry for the metadata-only case, the once-per-root prompt, the non-blocking install warning with a strip on apply, and the rejected alternatives: a full IoStore rebuild, a `discovery.Issue` from `Scan`, and a persisted "never ask again". The 0005 addendum covers the pool and the required-encryption offer.

`docs/USER_GUIDE.md` and `docs/TROUBLESHOOTING.md` cover both dialogs.

## Detect and CreatePak (15.2)

`internal/uassettool` gained `IsCompanionMetadataPath` and `CompanionPakUnsupportedPaths`, and `CompanionPakHasRawFiles` now reuses them so hybrid detection and cleanup detection cannot drift apart. Matching is on the path component, so any mount prefix, including `../../../`, is caught.

Typed `CreatePak` was added to the worker surface. The pinned `v1.5.6` worker rejects an empty file list, so a metadata-only companion is rewritten with one harmless `.cratebug-companion-stub` entry rather than an empty PAK. `CompanionPakHasRawFiles` ignores that stub, so a cleaned mod does not later classify as hybrid because of it.

## Library mutation (15.3)

`FindUnsupportedCompanionPaks` and `StripCompanionPaks` live in `internal/mutation`. Both are sequential. Find skips a member whose listing fails; strip fails that member and leaves its `.pak` as it was. Disabled primary names (`.pak_crateoff`, `.bak_bento`, `.pak_disabled`) are preserved through the replace. A failed replace rolls that `.pak` back.

## App and first-load UI (15.4)

`FindUnsupportedCompanionPaks`, `StripCompanionPaks`, and `CancelCompanionCleanup` are on `App` (`app.go:399`, `:409`, `:444`), with the same game-running block, progress event, and cancel shape as encrypt. Progress emits `companion:progress`.

`offeredCompanionCleanupRootsRef` and `offeredRequiredEncryptionRootsRef` (`LibraryScreen.tsx:413-414`) are `Set<string>` refs keyed by library root, so an offer is made once per root for the lifetime of the process and neither a Refresh nor a `reloadLibrary` after a mutation re-prompts. `companionOfferSettled` gates the encryption offer behind the companion offer.

## Install warning and strip (15.5)

`StagedMod.UnsupportedCompanionPak` and `PreviewItem.UnsupportedCompanionPak` carry the flag to the preview. It is deliberately not a `discovery.Issue`: `hasBlockingIssues` (`installPresentation.ts:149`) stays issue-only, so the warning informs without blocking install. `selectedUnsupportedCompanionCount` drives a `role="status"` footer note.

`ApplyInstall` calls `stripStagedCompanionPaks` (`app_companion.go:35`) before `install.Apply`, so a dirty archive never lands in the library. That function re-lists rather than trusting the preview flag, and a staged file that cannot be listed is copied as-is, since an unlistable PAK is a different problem from the metadata one.

## Required encryption and write pool (15.7)

`modtype.RequiresIoStoreEncryption` decides from the retained classify path listing, using the same content-root strip classify already applies. Companion metadata names do not force encryption, an empty listing does not force it, and classic PAK stays ineligible.

`mutation` sizes the encrypt worker pool with `uassettool.DefaultWorkerPoolSizeForLibrary` (`encryption.go:233`). The sequential single-caller path stays for unit tests.

---

## Commands and tests run

```powershell
.\check.ps1
```

```powershell
mise exec -c "go test ./internal/uassettool/ ./internal/mutation/ ./internal/modtype/ -count=1 -v"
```

```powershell
cd frontend; mise exec -c "bun test"
```

Validation on the working tree at `50ec6b4` (2026-09-06), run for this review:

- **`check.ps1`:** Passed. gofmt clean, Biome format and lint clean, TypeScript typecheck clean, Vite production build succeeded, `go vet` clean, all 10 Go packages pass.
- **Go suite:** all 10 packages pass uncached. New Phase 15 coverage passing by name:
  - `internal/uassettool` — `TestIsCompanionMetadataPathMatchesMountPrefixes`, `TestCompanionPakUnsupportedPathsReturnsMatches`, `TestCompanionPakHasRawFilesIgnoresCleanupStub`, `TestCreatePakRejectsEmptyOutput`, `TestCreatePakRejectsEmptyFileList`, `TestCreatePakSendsMappedPaths`.
  - `internal/mutation` — `TestFindUnsupportedCompanionPaksFindsDirtyAndSkipsListErrors`, `TestStripCompanionPaksRewritesMetadataOnlyPrimary`, `TestStripCompanionPaksPreservesDisabledPrimary`, `TestStripCompanionPaksSkipsCleanPak`, `TestStripCompanionPaksRollsBackFailedRebuild`, `TestStripCompanionPaksKeepsHybridRawFiles`, `TestStripCompanionPaksReportsCancel`, `TestSetModEncryptionStripsCompanionMetadataFromRebuiltPak`, `TestSetModEncryptionIgnoresCompanionCleanupStub`, `TestSetModEncryptionPooledRewritesEachTarget`, `TestFindModsNeedingEncryption`.
  - `internal/modtype` — `TestRequiresIoStoreEncryption` with 13 subtests covering Characters-only, both content-root prefixes, extensionless paths, UI, audio, mixed, empty listing, companion-metadata-only, and metadata-plus-Characters.
- **Supervised worker:** `TestOperationsAgainstSupervisedWorkerAndFixtureArchives` passes all five subtests against the pinned `v1.5.6` worker, including the new `create_pak_stub_drops_companion_metadata_names`. Not skipped.
- **Frontend suite (`bun test`):** 66 tests pass, 0 fail, 120 `expect()` calls across 5 files.
- **Wails bindings:** `FindUnsupportedCompanionPaks`, `StripCompanionPaks`, `CancelCompanionCleanup`, and `FindModsNeedingEncryption` are in `frontend/wailsjs/go/main/App.d.ts`; `mutation.CompanionCleanupResult`, `mutation.CompanionCleanupFailure`, and `install.PreviewItem.unsupportedCompanionPak` are in `models.ts`.

## End-to-end evidence

**There is none for this phase, and that is the material gap in it.** No running-app drive was performed and `docs/screenshots/phase-15/` does not exist. `C:\ModsFixtures`, the standing disposable fixture library named in `AGENTS.md`, is not present on this machine, and tasks 15.6 and 15.8 both conditioned their screenshots on a fixture being drivable.

What that leaves proven and unproven:

- **Proven by tests.** The rewrite itself: metadata-only becomes a valid stub PAK, hybrid companions keep their other files, disabled primaries keep their suffix, a failed replace rolls back, cancel stops cleanly, and the rebuilt PAK from an encrypt no longer carries the metadata names. Also the path rule for required encryption, the pool sizing, and the install strip running before `install.Apply`.
- **Read, not run.** The once-per-root-per-session prompting, the ordering that settles the companion offer before the encryption offer, the confirm copy, the install preview banner, and the progress and cancel chrome. These are covered by reading `LibraryScreen.tsx` and by the frontend unit tests, not by observing the application.

## Known limitations and deferred findings

1. **No running-app verification for this phase.** Three exit criteria — the companion warning appearing once on first load of a dirty library, the install preview naming affected mods, and the required-encryption offer appearing after classify — rest on code reading rather than on a drive. Building `C:\ModsFixtures` with a dirty companion PAK and a Characters-leaving IoStore bundle, then driving `wails dev`, would close all three in one pass. This is the first thing to do if any doubt arises about Phase 15's UI behaviour.
2. **No Phase 15 screenshots.** `docs/screenshots/phase-15/` does not exist. Carried forward with finding 1.
3. **Carried from Phase 14: no running-app encrypt or decrypt capture.** Still open, and now joined by the companion-cleanup equivalent.
4. **Hybrid mesh `.usmap`.** Unchanged from Phase 14. The disposable worker fixture has no real meshes, so extract-then-create never needed a `.usmap`. A real mesh rebuild that the pinned worker cannot finish without one remains an open risk, recorded in the test.
5. **No persisted "never ask again".** Rejected in 0006 for this phase, so a user who dismisses an offer is asked again next launch. Deliberate: a leftover dirty library would otherwise go silent.
6. **Companion cleanup does not rebuild `.utoc` or `.ucas`.** By design. If a container itself ever carries the crashing names, this phase does not address it.
7. **Install-time obfuscation and classic PAK encryption.** Out of scope by design.

## Review decision

**Decision:** Approved. Tasks 15.1 through 15.8 are implemented, and the canonical checks, the Go suites, the supervised-worker round trip, and the frontend suite all pass as recorded above. The exit criteria covering the rewrite, rollback, cancellation, path rule, pool sizing, and install strip are met by automated evidence.

The three exit criteria that describe first-load UI behaviour are accepted on code review rather than on a running-app drive, and that shortfall is recorded as finding 1 rather than presented as verified. Phase 15 is complete. Do not start a next phase until one is specified.
