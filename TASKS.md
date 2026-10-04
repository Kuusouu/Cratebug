# Cratebug Active Tasks

**Phase:** 20 - Per-mod install encryption
**Status:** Implementation complete. Review pending.

Phase 18 is complete. Phase 19 stays deferred.
The user approved Phase 20 on 2026-10-03.

## 20.1 Per-mod install encryption [IMPLEMENTED]

Add an encryption toggle to each mod in the shared install preview.
Start clear mods with the toggle off. Preserve encrypted mods.
Disable the toggle for classic and incomplete bundles.
Encrypt selected staged bundles before the install copies files into the library.
Stop the install if a rebuild fails. Keep the source files intact.
Show the current encryption target while the worker runs.

**Verify:**

1. Test independent choices, mixed source states, invalid bundles, rebuild failure, and cancellation with temporary fixtures.
2. Run `check.ps1` and the frontend tests.
3. Drive the app with `PinkVFX.zip` and several mods in a disposable fixture library.
4. Capture the preview, progress, success, and failure states.
5. Verify the installed encryption state with the backend. Stop for review.

**Results (2026-10-03):**

- `check.ps1` passed. Go format, vet, tests, frontend format, lint, typecheck, and production frontend build passed.
- All 82 frontend tests passed. New Go tests cover independent choices, mixed states, source preservation, unsupported bundles, failure, and cancellation.
- The app installed `PinkVFX.zip` with encryption. A two-mod install kept one copy clear and encrypted the other copy.
- A second-target failure left all 12 destination file hashes intact. An encrypted-source install preserved all three bundle files byte for byte.
- The source ZIP hash stayed intact. The preview and action buttons fit at 1400 x 950 and 1350 x 650 CSS pixels.

GPT 6 Astra reviewed the new tests at high effort.
The review removed one duplicate rollback assertion block.
All four test functions keep distinct install checks.
The Go package tests, `check.ps1`, and all 82 frontend tests passed after the removal.

Windows denied access to `C:\ModsFixtures`.
The app checks used `.playwright-cli/fixtures/install-encryption/library` instead.
The browser check replaced the native file picker with fixture paths.
The real Go backend handled all file operations.
The browser snapshot tool failed. The screenshots are frames from browser recordings.

**Screenshots:** `.playwright-cli/screenshots/phase-20/`

- `task-20.1-multiple-preview.png`
- `task-20.1-progress.png`
- `task-20.1-multiple-success.png`
- `task-20.1-already-encrypted.png`
- `task-20.1-failure.png`

These checks used the browser.
They did not check the native WebView, Linux, a live Nexus download, or the game.
The Nexus path shares the preview and apply flow.
Wails logged a browser IPC error during setup, before the install checks.
The real install calls and backend checks passed. Leave that runtime issue outside this task.
Stop for review. Do not start another phase.

## 20.2 Direct encryption fix (2026-10-04)

Encrypt (install toggle and Actions) no longer rebuilds through
`extract_iostore` + `create_mod_iostore`: that conversion drops cooked class
references without the game's class database (2 corrupted textures on Shark,
384 lost references on PinkVFX). Encrypt now copies the bundle to a temp
directory, runs the pinned worker's `pak_fixer` with obfuscation on the copy,
verifies the encrypted flag, and swaps files through park+replace+rollback.
Decrypt still rebuilds; the pinned worker exposes no direct decrypt rewrite.

**Verify:**

1. `check.ps1` passed. All 82 frontend tests passed (`bun test`).
2. Real worker + real Shark fixture: library encrypt kept all 6 internal
   paths identical, set the encrypted flag, and left no `chunknames` names.
   Staged install with the toggle installed 1 encrypted mod, kept the
   listing, and left the source ZIP byte-identical.
3. No browser check: this machine has no Node, so `playwright-cli` cannot
   run. The toggle UI is unchanged; only the backend changed.

Catalog encryption filter (same fix, per user approval): the header has a
filter-icon button next to Tags whose popover holds titled sections, starting
with Encryption (All / Encrypted / Unencrypted). Later filters append a
section entry (`FilterMenu.tsx`, `LibraryScreen.tsx`) without touching the
menu. Narrowed views show only classified complete IoStore bundles;
unclassified entries and ineligible bundles stay hidden until they qualify.
5 new frontend tests (87 total pass). The confirm dialog and user guide now
say encrypt instead of rebuild.

## 20.3 Direct decryption and existing mod repair (2026-10-04)

The user authorized the decrypt fix and repair of the supplied mod library.
This follow-up belongs to the Phase 20 encryption correction.

The legacy decrypt path used the fixed output name `rebuilt`.
The worker derived the same container ID for every rebuilt mod.
The library scan found that ID in 57 of 59 bundles.
Their different container header payloads also shared one chunk ID.

Go now decrypts the existing AES blocks in private staged copies.
It preserves compressed bytes, physical offsets, cooked assets, chunk IDs, and the container ID.
It handles clear obfuscated indexes and properly encrypted directory indexes.
It rejects unsupported versions, signed containers, and multiple partitions before any live replacement.
Both actions clean companion metadata before replacement.
The worker pin stays unchanged. See decision 0009.

**Validation:**

1. The focused Go package tests passed. New checks cover compressed bytes, both index forms, unsafe layouts, and the pinned worker.
2. The updated decrypt path processed copies of all 59 supplied bundles. All 1,756 decoded chunks matched the pre-change hashes.
3. Shark passed Encrypt then Decrypt. All seven decoded chunks, its original container ID, and its internal paths stayed intact.
4. The canonical `check.ps1` passed. All 87 frontend tests passed.
5. Wails launched with an isolated config and a disposable Shark copy. Its bound dev URL returned HTTP 200.

The T3 browser status and open calls both returned `Authentication required`.
No browser interaction or screenshot check completed.
No native layout check completed.
The app text now describes direct decryption. The layout did not change.
The dev process and its child processes closed after validation.

**Library repair:**

Back up all 177 original files outside the mod library before replacement.
The backup file hashes matched the originals.
Repair the 57 affected copies with IDs derived from their actual bundle filenames.
Update each TOC ID, container header chunk ID, and embedded container header ID.
Preserve all other decoded data and retain each encryption state.
Verify the copies before replacement. Verify the library after replacement.

The final library has 59 unique IDs and 1,756 verified decoded chunks.
All 59 companion PAK hashes stayed intact.
The two unaffected bundles stayed intact.
The library still has 56 encrypted bundles, three clear bundles, and 177 files.
The repair changed 114 sidecar files. It did not rename the mods.
The backup and detailed manifests remain outside version control.

The collision repair does not establish parity with unavailable original downloads.
Ten overlapping package paths remain. Some can be intentional overrides.
Do not remove those mods without further diagnosis.
The user tested the repaired library. The game still crashed on its async asset load thread.
All 177 game files matched the verified repaired copies before the isolation test.
The crash code used a null object reference as a package import index.
A targeted scan found null import dependencies in two Project Galacta assets.
These assets were `WBP_Galacta` and `GAL_ModLoader`.
The scan does not establish whether the original download contained those dependencies.
No Project Galacta-only game test completed, so the crash cause remains unconfirmed.

The user approved temporary mod isolation tests.
The game process stayed open with all mods disabled. No menu check completed.
The user then stopped the tests to reinstall fresh mods.
All 177 game files returned to their original locations and enabled states.
All 177 file hashes matched the isolation manifest after restoration.
No mod remained in the temporary park directory.
The final `go vet ./...`, `go test ./...`, and `git diff --check` passed.
Stop for review. Do not start another phase.

## 20.4 Move direct decryption into UAssetToolRivals (2026-10-04)

The user approved the worker change and use of the fixture library.
UAssetToolRivals v1.5.10 now exposes `decrypt_iostore` through its CLI and JSON interface.
Cratebug pins commit `c137d9abc4a7509d25f89ca9c6c0be3b7a87da23`.
See decisions 0004 and 0010.

The worker decrypts existing blocks without asset conversion or recompression.
It preserves compressed bytes, container IDs, chunk IDs, metadata, and paths.
It supports unsigned, single-partition TOC versions 1 through 5.
It validates block ranges before replacement and restores the UCAS backup after replacement failure.
Cratebug calls the worker on private staged copies.
Cratebug keeps bundle staging, companion cleanup, state checks, and live replacement rollback.
The Go IoStore format reader is removed.

**Validation:**

1. `check.ps1` passed. Go format, vet, all Go tests, frontend format, lint, and production build passed.
2. `go test ./internal/uassettool -run TestDecryptIoStoreDirectWithWorker -count=1 -v` passed with the fetched v1.5.10 worker.
3. UAT decrypted copies of all 58 supplied bundles and passed Encrypt then Decrypt. All 1,716 decoded chunk hashes matched.
4. Cratebug sent 55 worker decrypt requests. All 174 output file hashes matched direct UAT output. The source files stayed intact.
5. Shark, its manual reference, and the original PinkVFX download passed. All 399 decoded chunk hashes matched their inputs.

Playwright drove the Wails app in Edge with a writable workspace copy of PAJAMAPARTYHUD.
The UI decrypt action succeeded. The `.pak`, `.utoc`, and `.ucas` hashes matched direct UAT output.
The worker reported an unencrypted state. The source fixture in `C:\ModsFixtures` stayed unchanged.
The screenshot is `.playwright-cli/screenshots/phase20/task-20.4-decrypted.png` at 1280 by 800 pixels.
Wails logged a browser IPC startup error and the page returned a favicon 404.
The bound decrypt call still succeeded. No native DPI or game test completed.

The automatic encryption prompt still describes a rebuild. Defer that text issue.
Stop for review. Do not start another phase.
