# Phase 14 Review

**Date:** 2026-09-05
**Status:** Approved

## Outcome

Phase 14 adds a checked set and one catalog-header Actions menu. Users can Enable, Disable, Move, Tags, Encrypt/Decrypt, or Delete many mods from that menu. Viewing (`selectedEntryID`) still drives the details panel and the single-mod context menu. Checking (`checkedEntryIDs`) is the batch target. Click, Ctrl+click, and Shift+click follow Explorer rules. There is no always-visible checkbox on the card.

Encrypted IoStore mods classify instead of returning `ErrCannotDetermineType`. `Identity.encrypted` is a fact. Cards show a corner lock mark separate from the category pill. In-place encrypt and decrypt rebuild each complete IoStore triple through UAssetToolRivals (`extract_iostore` + `create_mod_iostore` with `obfuscate`). The Marvel Rivals AES key stays in Go. The frontend receives only the boolean.

Decision 0005 locks the selection split, the Actions menu, the key location, and the rebuild shape. Decision 0002 now says batch lives in the Actions menu and the selected-mod panel is a readout only.

---

## Docs and decisions (14.1)

`ROADMAP.md` names Phase 14 and no longer lists batch operations under Deferred. `SPEC.md` states the checked set, batch partial-success reporting, encryption as a library mutation, and that the AES key stays in Go. `docs/decisions/0005-batch-actions-and-encryption.md` records those locks. The 0002 addendum puts batch actions on the Actions menu and removes Enable/Delete/Clear from the selected-mod panel.

`docs/USER_GUIDE.md` and `docs/TROUBLESHOOTING.md` cover multi-select, the Actions menu, Encrypt/Decrypt eligibility, the game-running lock, and partial-batch results.

## Worker write surface (14.2)

`internal/uassettool` gained typed `ExtractIoStore`, `ExtractPakAll`, `CreateModIoStore`, and `ListPakWithKey`. `MarvelRivalsAESKey` is a package constant. `NewWriteWorker` uses `DefaultWriteCallTimeout` (10 minutes) so a real extract/create can finish. The rest of the worker request surface stays unmirrored.

Pinned `v1.5.6` accepts `obfuscate`. The supervised-worker subtest `iostore obfuscate round trip` builds a disposable hybrid fixture, encrypts it, asserts `IsIoStoreEncrypted` is true, decrypts it, and asserts false. That fixture is hybrid-only (`readme.txt`, no meshes). extract→create did not need a `.usmap`. Real mesh mods may still need one. That limit is recorded in the test.

## Encrypted as a fact (14.3)

`Identity` carries `Encrypted`. `ListInternal` calls `IsIoStoreEncrypted`, then `ListIoStoreFiles` with the game key when the container is encrypted. Classic PAKs stay `encrypted=false`. Encrypted IoStore no longer returns `ErrCannotDetermineType` for encryption alone. Wails models expose `identity.encrypted`. The key string does not appear in the frontend tree.

## Multi-select UI and lock (14.4)

`checkedEntryIDs` lives in `LibraryScreen`. `nextCheckedIDs` in `checkedSelection.ts` is the source of truth: a plain click views that mod and makes it the only checked member, a second click on that same only member clears both, Ctrl/Meta toggles, Shift ranges over the filtered list, Shift+Ctrl removes the range. IDs remap after rename/move and drop after a rescan. Right-click views the row, selects it if it was not already checked, and keeps the rest of an already-checked set.

The catalog header shows `N selected`, Select all (visible mods), and Clear. Clear empties the checked set and the viewed id together. The lock mark is card chrome (`clip-path` triangle, lock glyph), not a facts-row badge, in compact, large, and list.

## Actions dropdown and batch organize (14.5)

`BatchActionsMenu` is one exported component plus module CSS. An empty checked set disables the menu items. Enable/Disable/Move/Tags/Delete loop the existing one-mod Wails methods. Batch Enable/Disable call `SetModEnabled` directly so the single-mod handler's in-flight bail does not abort the rest of the set. `batchFeedback` reports partial success as a warning and never claims full success on a mixed result. Move, tag, and delete dialogs accept a batch count. The context menu stays single-mod. The card Enable switch stays.

## Encrypt and decrypt (14.6)

`SetModEncryption` in `app.go` plus `internal/mutation` is the one new Go mutation. It rescans, rejects classic/incomplete/ineligible, rejects mixed encryption state before any write, then rebuilds each bundle in a temp directory. Hybrid companions use `ListPakWithKey` and `ExtractPakAll`. A list failure fails that mod instead of dropping raw files. Replace parks live files, copies the rebuilt triple onto the original names (including `.pak_crateoff`), and rolls that bundle back on copy failure. Folder and priority filename stay. Game-running is blocked in the App method. Progress emits `encrypt:progress`. `CancelEncryption` stops before the next bundle. The frontend reloads the library after cancel or error so lock marks match disk.

The Actions item label and disable reason come from `encryptionMenuState`. The confirm dialog says the operation is a rebuild, not a bit-flip.

---

## Commands and tests run

```powershell
.\check.ps1
```

```powershell
mise exec -c "go test ./... -count=1"
```

```powershell
mise exec -c "bun test"
```

Validation on the working tree (2026-09-05):

- **`check.ps1`:** Passed. gofmt clean, Biome format and lint clean, TypeScript typecheck clean, Vite production build succeeded, `go vet` clean, all 10 Go packages pass.
- **Go suite (`go test ./... -count=1`):** all 10 packages passed uncached. New coverage includes `internal/uassettool` (typed extract/create/list-with-key, companion-PAK metadata filter, supervised obfuscate round trip), `internal/modtype` (encrypted IoStore listed with the game key, `Identity.encrypted`), and `internal/mutation` (classic rejection, mixed-state rejection, disabled-primary preservation, failed-rebuild rollback, companion-list fail-closed, already-at-target skip).
- **Frontend suite (`bun test`):** 65 tests pass, 0 fail. New coverage: `checkedSelection.test.ts` (replace, re-click clear, Ctrl toggle, Shift range, Shift+Ctrl remove, remap, retain) and `encryptionAction.test.ts` (complete IoStore, classic ineligible, encrypt, decrypt, mixed).
- **Wails bindings:** `SetModEncryption`, `CancelEncryption`, `mutation.EncryptionBatchResult`, and `Identity.encrypted` are in `frontend/wailsjs`.

## End-to-end evidence

Driven in the wails dev browser tab (Playwright / Edge, `http://localhost:34115`) against the scanned library at `C:\Users\mew\Downloads\~mods` (74 mods). That drive was UI-only. No encrypt, decrypt, or other mutation was applied to that library.

1. **Empty check set** — header shows `0 selected`. Select all / Clear / Actions chrome present. Screenshot: `docs/screenshots/phase-14/task-14.4-empty-check-set.png`.
2. **N selected** — two cards checked, header `2 selected`, checked tint visible. Screenshot: `docs/screenshots/phase-14/task-14.4-n-selected.png`.
3. **Clear** — checked set emptied. Screenshot: `docs/screenshots/phase-14/task-14.4-clear-selection.png`.
4. **Lock mark** — encrypted IoStore card shows the corner lock in grid and list, separate from the IoStore / category pills. Screenshots: `docs/screenshots/phase-14/task-14.4-lock-mark.png`, `task-14.4-lock-mark-list.png`.
5. **Mixed Encrypt disabled** — one encrypted and one unencrypted IoStore checked. Actions menu open, Encrypt greyed. Screenshot: `docs/screenshots/phase-14/task-14.5-mixed-encrypt-disabled.png`.

Encrypt and decrypt success on a disposable IoStore is proven by the supervised-worker round trip and the mutation replace/rollback tests, not by a running-app screenshot. `C:\ModsFixtures` was not present. The user's library was not used for a rewrite.

The Playwright tab is a separate process from the native WebView2 window. Shared Go backend means classification and `Identity.encrypted` are real. Size- and DPI-sensitive lock chrome was not checked in the native window.

---

## Known limitations and deferred findings

1. **No running-app encrypt/decrypt screenshot.** 14.7 asked for a successful rebuild on fixtures. That capture is missing. The worker fixture and mutation tests cover the rewrite. A later pass can screenshot a disposable library without touching a real `~mods`.
2. **Catalog screenshots vs final click rules.** Some early `task-14.4` / `task-14.5` shots were taken while card checkboxes still existed. The locked design is Explorer-style click with tint and border only. Current `ModCatalog.tsx` has no checkbox.
3. **Browser-session parity.** The drive ran in the wails dev browser tab, not the native window.
4. **Hybrid mesh `.usmap`.** The disposable worker fixture has no real meshes. extract→create did not need a `.usmap`. A real mesh rebuild that the pinned worker cannot finish without one is an open risk. The test records it.
5. **Companion PAK `chunknames` / `patched_files`.** After 3 September 2026 the game/anti-cheat can crash if those metadata names remain in an installed companion `.pak`. Cratebug already ignores them for hybrid detection. It does not strip them. Encrypt rebuilds the whole triple and may write them again. A pak-only cleanup is later work, not this phase.
6. **Install-time obfuscation and classic PAK encryption.** Out of scope by design.
7. **Console noise pre-existing.** A Wails `ipc.js` TypeError at page load and a favicon 404 appear in the dev session. Both predate this phase.

## Review decision

**Decision:** Approved. All Phase 14 tasks (14.1 through 14.7) and exit criteria are met per the canonical checks, the unit and supervised-worker suites, and the live UI drive (empty set, N selected, lock mark, mixed Encrypt disabled). Phase 14 is complete. Do not start a next phase until one is specified.
