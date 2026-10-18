# Cratebug Roadmap

What has shipped, roughly in order, and ideas that have not been built yet.

This is a record, not a plan to follow. New work does not need an entry here before it starts. Add a line when something significant lands. User-facing details are in [CHANGELOG.md](CHANGELOG.md), and the reasoning behind specific choices is in [docs/decisions/](docs/decisions/).

## Shipped

The early work was organized as numbered phases. The numbers are kept so that older commits, decision records, and code comments still make sense.

### July 2026

- **Phase 0 - Repository and toolchain.** Pinned Go, Wails v2, React, TypeScript, Vite, and Bun toolchain, CI, a production Windows build, and an NSIS installer and uninstaller. See [0001](docs/decisions/0001-toolchain-baseline.md).
- **Phase 1 - Read-only mod discovery.** Recursive scanning; recognizing `.pak`, `.pak_crateoff`, `.bak_bento`, and `.pak_disabled`; pairing `.utoc` and `.ucas` sidecars; classic and IoStore classification; priority parsing; and reporting incomplete and orphaned files.

### August 2026

- **Phase 2 - Read-only library UI.** Selective BentoMod frontend migration: header, search, folder navigation, list and grid views, loading, empty, and error states, and light, dark, and system themes.
- **Phase 3 - Safe enable and disable.** Toggling with `.pak_crateoff`, collision checks, a game-running check enforced in Go, rollback, and post-operation reconciliation.
- **Phase 4 - Organization and recoverable deletion.** Rename, filename-based priority, moving between folders, nested folder management, and Recycle Bin deletion behind a confirmation delay. See [0002](docs/decisions/0002-organize-action-pattern.md).
- **Phase 5 - Metadata and persistence.** Versioned settings and metadata with safe writes, last-known-good recovery, persistent mod identity, and tags.
- **Phase 6 - UAssetToolRivals boundary.** A pinned UAssetToolRivals worker driven as a supervised helper process, a Cratebug-owned mod-type layer, and a measured concurrency policy. See [0003](docs/decisions/0003-uassettoolrivals-boundary.md) and [0004](docs/decisions/0004-pin-uassettool-worker.md).
- **Phase 7 - UAssetToolRivals UI integration.** Mod category, hero and skin names, and hero portraits in the library, classified progressively and cached between scans.
- **Phase 8 - Installation and archive safety.** Staged extraction, traversal and unsafe-link protection, an install preview, collision decisions, and cancellation.
- **Phase 9 - Asset conflict inspection.** Detection of same-priority and cross-priority asset overlap, with a conflict details dialog.
- **Phase 10 - Updates and release hardening.** A CalVer GitHub release pipeline, in-app update check and silent apply with relaunch, a "what's new" dialog, and first public release hardening. Phase 11 was folded into this one.
- **Phase 12 - Library auto-detection.** A per-store provider seam with Steam detection, and confirmed creation of a missing `~mods` folder.

### September 2026

- **Phase 13 - Epic Games detection.** An Epic provider that reads launcher manifests.
- **Phase 14 - Batch actions and in-place encryption.** A checked selection with Ctrl and Shift ranges, an Actions menu, and in-place IoStore encrypt and decrypt. See [0005](docs/decisions/0005-batch-actions-and-encryption.md).
- **Phase 15 - Companion PAK cleanup and required encryption.** Rewriting companion `.pak` files that contain `chunknames` or `patched_files`, and offering encryption for IoStore mods that reach outside `/Game/Marvel/Characters`. See [0006](docs/decisions/0006-companion-pak-cleanup.md).
- **Phase 16 - Nexus Mods integration.** Bring-your-own API key, premium API downloads, free-account `nxm://` handling, and single-instance link handoff. Replaced the generic install-from-URL path. See [0007](docs/decisions/0007-nexus-mods-integration.md).
- **Phase 17 - Filesystem watching.** A recursive watcher on the mod root reconciles external changes live and triggers cleanup and encryption prompts for newly added mods.
- **Phase 18 - Linux distribution.** A portable Go core, XDG trash, Secret Service key storage, the Linux worker, `nxm://` through `.desktop` files, and an AppImage build, checked on Pop!_OS, Fedora, and CachyOS.

### October 2026

- **Phase 20 - Per-mod install encryption.** An encryption toggle for each complete IoStore mod in the install preview. Decryption later moved to a direct worker action that preserves cooked chunks and container IDs. See [0008](docs/decisions/0008-install-encryption.md), [0009](docs/decisions/0009-direct-iostore-decryption.md), and [0010](docs/decisions/0010-worker-owned-iostore-decryption.md).
- **Phase 21 - Backup and restore.** Whole-library zip backup and staged restore from the Tools dialog, with counts on both ends and rollback on failure. Native file-dialog verification was still pending at approval.
- **Phase 22 - Signature bypass tool.** Installing and removing the pinned UTOC signature bypass from the Tools dialog, with checksum verification and refusal to touch files Cratebug does not own. Not run against a real game install. See [0011](docs/decisions/0011-signature-bypass-tool.md).
- **Midnight Iris palette.** New light and dark themes and six accent presets.
- **Toolchain and UI foundation.** Moved from Biome to oxlint and oxfmt, adopted Tailwind CSS and shadcn/ui on Base UI, and enforced the coding guidelines through lint and CI checks.

## Ideas for later

Nothing here is committed or ordered.

- Linux AppImage self-updating (planned as Phase 19): find the AppImage asset, replace the running file, and relaunch, with a clear message when running outside an AppImage.
- Importing BentoMod or Repak-X state.
- Game launching and crash monitoring.
- Character data updates.
- Recompression and VFX updating.
- Virtual collections.
- Permanent deletion.
- Better reconciliation of external renames.
