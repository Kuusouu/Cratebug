# App service split (working plan)

Temporary planning file for breaking the single bound `App` struct into
several bound service structs. Delete this file when the split is done.

## Why

`App` binds 78 methods and holds about 30 fields across `app.go`,
`app_nexus.go`, `app_protocol.go`, `app_backup.go`, `app_companion.go`,
`app_sigbypass.go`, and `app_reveal.go`. Every feature can reach every other
feature's state. Splitting it gives each service its own state, gives the
frontend matching import boundaries (which also helps break up
`LibraryScreen.tsx`), and fixes the metadata write race below.

Wails v2 binds several structs through `Bind: []interface{}{...}` in
`main.go`. Each struct gets its own module, `frontend/wailsjs/go/main/<Struct>.js`.

## Shared core (unexported, not bound)

Everything that crosses domain lines today. It lands first, as one change,
with `App` still the only bound struct.

| Piece | Replaces | Used by |
| --- | --- | --- |
| `runtimeHost`: `ctx` behind a mutex, `emit(event, data)`, `context()` | `a.ctx` read without a lock in most places, but behind `pendingURLMu` in `app_nexus.go`; about 10 copies of `if a.ctx != nil { EventsEmit }`; `updateContext`, `requestContext` | everyone |
| watcher plus `suppress(fn)` | `suppressWatcherResult` | organize, PAK repair, install, restore |
| game check: `requireNotRunning()` | 4 copies of the game-running block; `requireGameNotRunning` in `app_sigbypass.go` | PAK repair, install, restore, signature bypass (organize goes through `mutation.Executor`) |
| `metadataGate`: store plus mutex, `read()`, `update(func(*metadata.Document) error)` | about 18 copies of load, modify, `Save` | settings, tags, organize, install, update, protocol, signature bypass, startup |
| `catalog`: classifier plus lazily loaded character table, `classify()`, `pathsFor()` | `classifier`, `tableMu`, `characterTable`, `tableLoaded`, `getCharacterTable` | library, PAK repair, install |
| `detector` | `detector` | library, signature bypass |

```go
type core struct {
    runtime  *runtimeHost
    watcher  *watcher.Watcher
    game     mutation.GameRunningChecker
    meta     *metadataGate
    catalog  *catalog
    detector *gamedetect.Registry
}
```

**Metadata race (a real bug).** Every metadata write is an unguarded load,
modify, save. If Wails runs binding calls concurrently, two overlapping writes
lose one of them: for example `AssignModTag` overlapping `SetTheme`, or
`ApplyInstall`'s `recordInstalledModMetadata` overlapping a tag edit. Confirm
how Wails v2 dispatches binding calls, then route every write through
`metadataGate.update`.

## Services

Listed from cleanest boundary to messiest.

### 1. UpdateService
- Bound: `GetAppVersion`, `CheckForUpdate`, `CheckWhatsNew`, `DownloadUpdate`, `ApplyUpdate`
- Own state: none (`updateClient`, `pathWithinDir`, update constants)
- Core: runtime (events, `Quit`), metadata (`LastSeenVersion`)
- Frontend: `LibraryScreen.tsx`

### 2. SignatureBypassService
- Bound: `SignatureBypassStatus`, `InstallSignatureBypass`, `RemoveSignatureBypass`
- Own state: `sigbypassTool`
- Core: game check, detector, metadata read (`LibraryProvider`)
- Frontend: `ToolsDialog.tsx`, `LibraryScreen.tsx`

### 3. BackupService
- Bound: `BackupLibrary`, `CancelBackup`, `RestorePreview`, `RestoreApply`, `DiscardRestorePreview`, `CanRetryRestore`, `CancelRestore`
- Own state: `restoreSessions`, plus the backup and restore cancel handles, which are package globals today (`backupMu`, `backupCancel`, `restoreMu`, `restoreCancel` in `app_backup.go`). Make them fields.
- Core: runtime, watcher, game check
- Restore writes the metadata file directly by path, not through the store. Keep that deliberate.
- Frontend: `ToolsDialog.tsx`

### 4. InstallService
- Bound: `SelectFilesForInstall`, `PrepareInstall`, `ApplyInstall`, `CancelInstall`
- Own state: `installSessionManager`; `markStagedCompanionPaks` and `stripStagedCompanionPaks` from `app_companion.go`
- Core: runtime, watcher, game check, catalog, metadata
- Exposes an unexported `stage(paths, defaultFolder, source *nexusSource)` for Nexus.
- **Untangle from Nexus:** today Nexus calls `stageAndPreview`, Install calls `clearPendingNexusSource` and `takePendingNexusSource`, and the pending source is one slot on `App`, not tied to an install session. Store the Nexus source on the staged session instead, so `ApplyInstall` reads it from there. That removes `pendingNexusSource` and its three helpers, and makes the dependency one way (Nexus depends on Install).
- Frontend: `InstallPreviewDialog.tsx`, `LibraryScreen.tsx`

### 5. NexusService
- Bound: `SetNexusAPIKey`, `ClearNexusAPIKey`, `NexusAccount`, `ResolveNexusModPage`, `TakePendingNexusLink`, `DiscardNexusLink`, `PrepareNexusInstall`, `CancelNexusDownload`, `NexusProtocolStatus`, `RegisterNexusProtocol`, `UnregisterNexusProtocol`
- Unexported entry points used by `main.go` and startup: `setLaunchURL`, `onSecondInstanceLaunch`, `ensureNexusProtocol`
- Own state: `secretStore`, `nexusMu`, `nexusClient`, `nexusDownloadCancel`, `nexusBaseURL`, `pendingURLMu`, `pendingLink`, `linkSecrets`, `ctxReady`, `protocol`, `allowProtocol`
- Core: runtime, metadata (protocol snapshot and opt-out); depends on InstallService
- Keep protocol registration in this service, in its own file (`app_protocol.go` today). `runUninstallCleanup` stays a plain function.
- Frontend: `SettingsDialog.tsx`, `InstallFromNexusDialog.tsx`, `InstallPreviewDialog.tsx`, `LibraryScreen.tsx`

### 6. PakRepairService (encryption and companion PAK cleanup)
- Bound: `SetModEncryption`, `CancelEncryption`, `FindModsNeedingEncryption`, `FindUnsupportedCompanionPaks`, `StripCompanionPaks`, `CancelCompanionCleanup`
- Own state: `encryptMu`, `encryptCancel`, `companionMu`, `companionCancel`, `companionCache`; `lazyWorkerCaller`
- Core: runtime, watcher, game check, catalog
- Frontend: `LibraryScreen.tsx`

### 7. OrganizeService
- Bound: `SetModEnabled`, `RenameMod`, `SetModPriority`, `MoveMod`, `CreateFolder`, `RenameFolder`, `MoveFolder`, `DeleteMod`, `DeleteFolder`, `IsFolderEmpty`
- Own state: `mutationExecutor`; `executeAndReconcile`
- Core: watcher, metadata (re-pointing tags after rename, priority, or move)
- Frontend: `LibraryScreen.tsx`, `FolderDeleteConfirmDialog.tsx`

### 8. LibraryService (read side and setup)
- Bound: `ScanLibrary`, `ClassifyLibrary`, `DetectConflicts`, `SetModRoot`, `DetectLibrary`, `CreateLibrary`, `SetLibraryWatcherSuspended`, `OpenFolderInExplorer`, `OpenModInExplorer`
- Own state: none
- Core: watcher (root), catalog, detector, metadata (`ModRoot`)
- Kept separate from Organize: this side only reads, Organize is the gated mutation path from SPEC section 10.
- Frontend: `LibraryScreen.tsx`

### 9. SettingsService (preferences and tags)
- Bound: `LoadMetadata`, `SetTheme`, `SetDefaultViewMode`, `SetAccentColor`, `SetSkipDestructiveDelay`, `SetLibraryProvider`, `CreateTag`, `RenameTag`, `DeleteTag`, `AssignModTag`, `UnassignModTag`
- Own state: none (`MetadataState`)
- Core: metadata
- Frontend: `LibraryScreen.tsx`

### What stays on App
Construction, `startup` (gives `ctx` to the runtime host, hands off the pending
nxm link, claims nxm, points the watcher at the saved root), and `shutdown`.
`App` becomes the composition root and binds nothing itself. Delete
`RuntimeStatus` and `NexusKeyState`; the frontend calls neither.

## Order

1. Core, alone. `App` stays the only bound struct. Includes the `metadataGate` race fix.
2. In parallel: UpdateService, SignatureBypassService, BackupService.
3. InstallService, then NexusService.
4. In parallel: PakRepairService, OrganizeService, LibraryService, SettingsService.
5. Cleanup: `*Type()` anchors, `App` reduced to the composition root, delete this file.

## Rules for each extraction

- One service per change. Move its methods, fields, and tests; update `main.go`'s `Bind` list.
- Update that service's frontend imports in the same change, so old and new bindings never overlap. Regenerate bindings (`wails generate module` or `wails dev`) and commit the regenerated `frontend/wailsjs` output.
- Tests that poke `App` fields directly (`app.watcher` 12 times, `app.protocol` 8, `app.allowProtocol` 6, `app.detector` 5, `app.nexusBaseURL` 4, `app.restoreSessions` 3, `app.sigbypassTool` 3, and others) move to injection through the service constructor, the way `newApp` does today.
- No behavior changes beyond the ones listed here (the metadata gate, the Nexus source on the staged session, backup globals becoming fields). Event names stay the same: `install:progress`, `encrypt:progress`, `companion:progress`, `backup:progress`, `restore:progress`, `update:downloadProgress`, `nexus:link`, `library:fs-changed`.
- Keep Wails out of `internal/`; services live in package `main`.
- Run `./check.ps1`, then launch with `wails dev` and exercise the moved feature against `C:\ModsFixtures`.

## `*Type()` anchors

`ClassificationType`, `ConflictType`, `UpdateType`, `EncryptionType`,
`CompanionCleanupType`, `NexusDownloadRequestType`, `BackupType`,
`RestorePreviewType`, `RestoreResultType`, `SignatureBypassStatusType`.
The frontend calls none of them; they exist to get types into `models.ts`.
Remove one only if a real method already returns its type, and confirm by
diffing `frontend/wailsjs/go/models.ts` after regenerating.
`NexusDownloadRequestType` stays: that type only travels through the
`nexus:link` event, and Wails does not generate types for event payloads.
