# Cratebug Active Tasks

**Phase:** 17 - Filesystem watching and live library reconciliation
**Status:** Complete. Review approved 2026-09-20.

This file contains only the active work. Do not start the next phase.

## Objective

Watch the active Marvel Rivals mod directory recursively. Reconcile catalog state automatically when external additions, edits, or removals occur. Retain current folder filter, search term, and valid checked selections across live reloads. Check newly discovered or modified mods for unsupported companion PAK entries (`chunknames` / `patched_files`) and required IoStore encryption, offering their respective warning popups without requiring an application restart.

## Design decisions

* **Full directory scan over incremental patching.** `discovery.Scan` performs metadata-only traversal via `filepath.WalkDir` taking under 10 ms. Classification results and asset path listings are cached in memory by `(entryID, mtime)`. Companion PAK inspections are cached by `(pakPath, mtime, size)`. Incremental file patching adds high complexity and failure modes with no meaningful performance benefit.
* **Go owns the watcher.** Following repository rules, filesystem operations belong in Go, not React. `internal/watcher` wraps `fsnotify` and manages recursive directories and debouncing.
* **Trailing quiet-window debounce.** Rapid bursts of filesystem events (such as extracting archives or copying multi-gigabyte IoStore containers) are coalesced into a single notification after a 300 ms quiet period.
* **Noise filtering.** Non-mod file extensions and hidden files (such as `.tmp`, `.crdownload`, `desktop.ini`, and metadata stores) do not trigger catalog rescans.
* **Internal mutation suppression.** Cratebug operations (Enable, Disable, Rename, Move, Delete, Encrypt, Strip Companion, and Install) pause or suppress watcher notifications so Cratebug's own changes do not trigger duplicate scans or feedback loops.
* **Session dismissal tracking per mod ID.** Popups track dismissed entry IDs rather than only library root paths. A user who declines fixing Mod A will not be re-prompted for Mod A, but dropping dirty Mod B will prompt for Mod B.
* **Ordered popup queue.** Newly detected required encryption dialogs queue behind companion cleanup dialogs, preserving Phase 15 priority order.

## Out of scope

* File watching outside the active mod root
* Watching when Cratebug is closed
* Incremental diff patching of the `discovery.Library` struct

## 17.1 Docs and design decisions

Update ROADMAP and TASKS to define Phase 17. Document design decisions for watcher debouncing, scan strategy, and popup session tracking.

**Verify:** `ROADMAP.md` reflects Phase 17, and `TASKS.md` contains Phase 17 active tasks.

## 17.2 Filesystem watcher package

Implement `internal/watcher`:
- `Watcher` wrapping `github.com/fsnotify/fsnotify`
- Recursive subfolder discovery and dynamic addition/removal on directory create/delete events
- Trailing debounce (300 ms)
- Extension and file noise filtering
- Pause, Resume, and SetRoot methods

**Verify:** `go test ./internal/watcher/ -count=1`. Unit tests under `t.TempDir()` covering directory creation, file creation, debounced burst events, noise filtering, and pause/resume suppression.

## 17.3 Companion inspection cache

Implement in-memory inspection caching in `internal/mutation` for companion PAK scans:
- Keyed by `(pakAbs, mtimeNs, size)`
- Avoid redundant worker calls during rescans when `.pak` files have not changed

**Verify:** `go test ./internal/mutation/ -run TestFindUnsupportedCompanionPaks -count=1`. Verify cache hit avoids caller invocations on unchanged files.

## 17.4 App layer wiring and suppression

Wire `internal/watcher` into `App` (`app.go`):
- Start watcher when mod root is loaded or updated
- Emit `library:fs-changed` Wails event when changes settle
- Suppress watcher triggers during internal mutations in `ExecuteMutation`, `StripCompanionPaks`, `EncryptMods`, and mod installs

**Verify:** `go test ./... -count=1`. Unit tests confirming watcher suppression during mutations and event emission.

## 17.5 Frontend live refresh and popup triggering

Update `LibraryScreen.tsx`:
- Listen to `library:fs-changed` event
- Call `reloadLibrary()` while preserving active folder and search query
- Track dismissed companion cleanup and encryption entry IDs
- Trigger `CompanionPakDialog` and `RequiredEncryptionDialog` when new dirty mods are detected

**Verify:** `bun run check` and `bun test`.

## 17.6 Verification

Run canonical checks: `go test ./... -count=1`, `bun run check`, `bun test`.
