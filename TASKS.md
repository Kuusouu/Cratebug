# Cratebug Active Tasks

**Phase:** 14 - Batch actions and in-place encryption
**Status:** Complete
**Branch:** `feat/phase-14-batch-encryption`

Review approved 2026-09-05. See `docs/reviews/phase-14-review.md`. Do not start the next phase.

## Objective

Users can check many mods and run the same action on the set from one Actions menu, including encrypting or decrypting complete IoStore bundles in place.

## Design decisions

* **Two selections.** Viewing (`selectedEntryID`) is the details panel and the single-mod context menu. Checking (`checkedEntryIDs`) is the batch target. Click a card = view it and make it the only checked mod. Click it again = clear both. Ctrl+click toggles check without clearing the rest. Shift+click ranges over the current filtered list.
* **Actions icon dropdown** in the catalog header, beside Tags. Visible chrome: `N selected`, Select all (visible mods), Clear. Menu: Enable, Disable, Move to..., Tags..., Encrypt/Decrypt, Delete....
* **Context menu stays single-mod.** Rename / priority / move / tags / delete for the right-clicked row only.
* **Card Enable switch stays.** Dropdown Enable/Disable is for the checked set.
* **Encrypt is IoStore-only.** Complete `.pak` + `.utoc` + `.ucas`. Classic, incomplete, orphaned, or non-mod: ineligible.
* **Uniform encrypt state required.** All encrypted → Decrypt. All unencrypted → Encrypt. Mixed → disabled with a mixed-state reason. Ineligible format uses a different reason.
* **Lock on every card/row** when `Identity.encrypted` is true. Separate from the category pill.
* **Batch organize loops existing APIs.** `SetModEnabled`, `MoveMod`, `AssignModTag` / `UnassignModTag`, `DeleteMod`. Skip already-in-state. One summary toast.
* **Encrypt/decrypt is a new mutation.** Rebuild in temp (`extract_iostore` + `create_mod_iostore` with `obfuscate`), replace, rollback that bundle on failure, `BlockedWhileGameRunning`. Sequential. Install-style progress. Write-op timeout above 30s.
* **AES key stays in Go.** Marvel Rivals game key, hardcoded. Frontend receives only `encrypted: boolean`.
* **SelectedModPanel** is a viewed-mod readout only. No Enable, Delete, or Clear on that strip.

## Out of scope

* Install-time obfuscation
* Classic-PAK encryption
* Converting classic mods to IoStore
* A persistent Bento-style bulk button row
* New batch Wails methods for enable/move/tags/delete
* Exposing the AES key to the frontend
* VFX, recompress, BentoMod changes
* `ROADMAP.md` later phases

## 14.1 Docs

Write the ROADMAP Phase 14 entry, this TASKS file, SPEC additions (checked set, batch partial success, encryption as a library mutation, key stays in Go), `docs/decisions/0005-batch-actions-and-encryption.md`, and a short 0002 addendum that batch actions go in the Actions menu.

Do not update USER_GUIDE or TROUBLESHOOTING here. That is 14.7.

**Verify:** ROADMAP names Phase 14 and no longer lists batch operations under Deferred. SPEC and 0005 state the key stays in Go. 0002 says batch lives in the Actions menu.

## 14.2 Worker write surface

Confirm the pinned `v1.5.6` `create_mod_iostore` accepts `obfuscate`. If it does not, re-pin per `docs/decisions/0004-pin-uassettool-worker.md` before any encrypt code.

Add typed `ExtractIoStore` and `CreateModIoStore` in `internal/uassettool/operations.go` only. Do not mirror the full request struct. Add `MarvelRivalsAESKey`. Add `aes_key` to `ListPak` when hybrid detection needs it. Add `ExtractPakAll` if hybrid rebuild needs companion-PAK files. Add `CallWithTimeout` so write calls can exceed the 30s default.

Prove extract → create with `obfuscate` true/false against a disposable hybrid fixture the worker itself built. Record whether extract→create needs a `.usmap`. If the pinned worker requires one for real meshes, stop and decide. Do not silently ship a broken encrypt.

**Verify:** Unit tests cover the new typed ops. The supervised-worker test encrypts then decrypts a disposable fixture and `IsIoStoreEncrypted` matches. `go test ./internal/uassettool/ -count=1` passes.

## 14.3 Encrypted as a fact

Add `Encrypted bool` to `Identity`. In `ListInternalPaths`, call `IsIoStoreEncrypted`, then `ListIoStoreFiles` with the game key when encrypted. Stop returning `ErrCannotDetermineType` for encryption. Update `determine_test.go` and any conflict test that treated encrypted as unavailable.

Regenerate Wails bindings so the frontend can read `identity.encrypted`.

**Verify:** Encrypted IoStore fixtures classify instead of returning `ErrCannotDetermineType`. `Identity.encrypted` is true. `go test ./internal/modtype/ ./internal/conflict/ -count=1` passes.

## 14.4 Multi-select UI + lock

`checkedEntryIDs` in `LibraryScreen`. No always-visible checkbox. Click, Ctrl+click, and Shift+click on compact, large, and list. Shift/Ctrl helpers as a unit-tested function over the filtered list. Catalog header: N selected, Select all, Clear. Lock mark when `encrypted`. Remap checked IDs after rename/move/rescan the same way viewing is remapped. Right-click views that row, selects it if it was not already checked, and opens the single-mod menu. Right-clicking an already-checked row keeps the rest of the set.

**Verify:** `bun test` covers the range helper. `bun run check` from `frontend/` passes.

## 14.5 Actions dropdown + batch organize

New `BatchActionsMenu` (one exported component + module CSS). Wire Enable/Disable/Move/Tags/Delete to loop existing handlers. Reuse move, tag, and delete dialogs by passing the checked ID list (or looping from `LibraryScreen`). Skip already-enabled / already-disabled. Summary toast. Batch must own the mutation lock for the whole set. `setModEnabled` today bails if any mutation is in flight.

**Verify:** `bun run check` from `frontend/` passes. Dialogs accept a batch without changing single-mod context-menu behavior.

## 14.6 Encrypt/decrypt

New `SetModEncryption` in `app.go` + `internal/mutation`: rescan, reject ineligible, reject mixed state server-side, extract to temp, `create_mod_iostore` with `hybrid` if the companion PAK has raw files, write beside the live bundle, replace primary+sidecars, keep folder / priority filename / `.pak_crateoff`. Progress + cancel. Confirm dialog: rebuild warning. Menu item label and disabled reason from a pure helper over the checked identities. After each success, classification cache misses on mtime and the lock updates.

**Verify:** Go tests cover mixed-state rejection, ineligible format, disabled-primary preservation, and rollback. Frontend helper tests cover Encrypt / Decrypt / mixed / ineligible. `go test ./internal/mutation/ ./internal/uassettool/ -count=1` passes.

## 14.7 Verify and review

Run `.\check.ps1`, Go tests, `bun test`, and `wails generate module` if bindings changed. Update `docs/USER_GUIDE.md` and `docs/TROUBLESHOOTING.md`. Capture running-app screenshots under `docs/screenshots/phase-14/`. Stop at the review gate. Do not start the next phase.

**Verify:** Canonical checks pass. Screenshots exist for empty check set, N selected, lock mark, mixed-state disabled Encrypt, and a successful encrypt or decrypt on fixtures.
