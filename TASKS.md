# Cratebug Active Tasks

**Phase:** 15 - Unsupported companion PAK cleanup and required encryption
**Status:** Complete. Review approved 2026-09-06; see `docs/reviews/phase-15-review.md`.

This file contains only the active work. Do not start the next phase.

## Objective

Warn when mods still contain `chunknames` / `patched_files` companion PAK entries, then rewrite only those `.pak` files one at a time. Offer the rewrite on first library load and strip during install. After classify, offer encryption for complete unencrypted IoStore mods whose listings leave `/Game/Marvel/Characters`, and run those rebuilds through a write-worker pool.

## Design decisions

* **Detect by listing.** `list_pak` on each primary. A path containing `chunknames` or `patched_files` (any mount prefix) is unsupported. Filesystem scan does not see inside PAKs.
* **Pak-only rewrite.** Extract remaining files, `create_pak` without those names, replace the live `.pak` (including `.pak_crateoff`). Leave `.utoc` / `.ucas` untouched. Metadata-only companions become a valid stub PAK.
* **First load, not every refresh.** After the first populated scan of a library root this session, offer Yes/No. Dismiss is session-only. `reloadLibrary` after a mutation does not re-prompt.
* **Install strips.** Preview warns. The issue does not block install. Apply rewrites the staged `.pak` before copy so the library never receives the crashy names.
* **One new mutation.** `FindUnsupportedCompanionPaks` then `StripCompanionPaks` loops one ID at a time with progress and cancel, same shape as encrypt. AES key is used only to list an encrypted index. The rewritten PAK is not obfuscated.
* **Game-running lock.** Same as other writes.
* **Required encryption after classify.** A complete unencrypted IoStore needs the game key when any retained listing path sits outside `/Game/Marvel/Characters` (same content-root strip classify uses). Empty listings and companion metadata names do not force it. Classic PAK stays ineligible.
* **Companion dialog first.** The companion offer starts at scan. The encryption offer waits until that offer has settled and its dialog is closed. Encrypt strips `chunknames` / `patched_files` from the rebuilt `.pak` before replace.
* **Write-worker pool, not the classify processes.** `SetModEncryption` from the app launches `NewWriteWorker` processes sized with `DefaultWorkerPoolSizeForLibrary`. The classify pool stays on `NewPinnedWorker` (short timeout, may be busy). Sequential `SetModEncryption` with one caller stays for tests.

## Out of scope

* Rebuilding `.utoc` / `.ucas` for companion cleanup
* Persisted "never ask again"
* Install-time obfuscation
* BentoMod changes
* `ROADMAP.md` later phases

## 15.1 Docs

Write the ROADMAP Phase 15 entry, this TASKS file, SPEC additions, and `docs/decisions/0006-companion-pak-cleanup.md`.

**Verify:** ROADMAP names Phase 15. SPEC says install strips and first load warns. 0006 says pak-only rewrite.

## 15.2 Detect + CreatePak

Add `IsCompanionMetadataPath` / `CompanionPakUnsupportedPaths` in `internal/uassettool`. Reuse them from `CompanionPakHasRawFiles`. Add typed `CreatePak`. The worker rejects an empty file list, so a metadata-only rewrite packs `.cratebug-companion-stub`. Prove a disposable dirty PAK lists as dirty, then a stub or hybrid rewrite lists clean.

**Verify:** Unit tests plus supervised-worker rewrite. `go test ./internal/uassettool/ -count=1` passes.

## 15.3 Library mutation

`FindUnsupportedCompanionPaks` and `StripCompanionPaks` in `internal/mutation`. Sequential. Mixed listing errors skip that member for find, fail that member for strip. Preserve disabled primary names. Rollback that `.pak` on replace failure.

**Verify:** Go tests cover metadata-only rewrite, skip-when-clean, disabled primary, rollback. `go test ./internal/mutation/ -count=1` passes.

## 15.4 App + first-load UI

`FindUnsupportedCompanionPaks`, `StripCompanionPaks`, `CancelCompanionCleanup` on `App`. First populated `scan()` of a root this session offers the dialog. Confirm copy names the anti-cheat crash and that utoc/ucas stay put. Progress + cancel.

**Verify:** `bun run check` from `frontend/`. Bindings regenerated.

## 15.5 Install warning + strip

`unsupportedCompanionPak` on staged/preview items. `hasBlockingIssues` stays issue-only. Preview banner. `ApplyInstall` rewrites flagged staged primaries before `install.Apply`.

**Verify:** Preview helper tests. Install apply still transactional. `bun test` and `go test ./internal/install/ ./internal/mutation/ -count=1` pass.

## 15.6 Verify companion cleanup

Run `.\check.ps1`, Go tests, `bun test`. Update USER_GUIDE and TROUBLESHOOTING. Screenshot the first-load dialog and the install banner when a disposable fixture can be driven.

## 15.7 Required encryption + write pool

`modtype.RequiresIoStoreEncryption` from retained classify paths. `FindModsNeedingEncryption` after classify. First populated load of a root this session offers Encrypt / Not now. Confirm calls `SetModEncryption(..., encrypt=true)`. App encrypt launches a `NewWriteWorker` pool sized by `DefaultWorkerPoolSizeForLibrary`. Sequential single-caller encrypt stays for unit tests. Do not re-prompt on Refresh or `reloadLibrary`. Do not encrypt the user's real library.

**Verify:** Path-rule tests (Characters vs UI/audio, prefixes, empty listing). Pooled rewrite test counts launched workers. `bun run check` from `frontend/`. Bindings include `FindModsNeedingEncryption`.

## 15.8 Verify

Run `.\check.ps1`, Go tests, `bun test`. Update USER_GUIDE and TROUBLESHOOTING for the required-encryption dialog. Screenshot that dialog when a disposable fixture can be driven. Stop at the review gate.
