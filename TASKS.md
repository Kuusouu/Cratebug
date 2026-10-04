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
