# Cratebug Product Specification

**Status:** Draft v0.1  
**Project:** Cratebug

## 1. Product overview

Cratebug is an open-source desktop mod manager for Marvel Rivals.

It is a fresh implementation built with Go, Wails, React, and TypeScript. It replaces BentoMod while preserving useful workflows, compatibility with existing mod libraries, and the recognizable structure of its interface.

Cratebug is a behavioral rewrite, not a line-by-line port. BentoMod may be inspected as a read-only reference for observable behavior, compatibility formats, fixtures, and visual direction. Its architecture and known bugs must not be reproduced merely for compatibility.

## 2. Goals

Cratebug should:

- Make installing, organizing, enabling, disabling, and inspecting Marvel Rivals mods straightforward.
- Treat related mod files as one logical bundle.
- Make filesystem operations safe, predictable, and recoverable.
- Preserve compatibility with existing BentoMod libraries where practical.
- Report failures clearly instead of silently leaving partially modified mods.
- Remain understandable and maintainable for a small open-source project using AI-assisted development.
- Reuse and improve useful parts of BentoMod's frontend without inheriting its structure wholesale.
- Ship as a conventional Windows application with an installer and uninstaller.

## 3. Product principles

### Behavior before implementation

Requirements describe what users should observe. They do not prescribe Go interfaces, package layouts, React state libraries, persistence schemas, or process boundaries unless a separate decision establishes them.

### Filesystem safety before convenience

Cratebug manages user files. Preventing data loss and unclear partial changes is more important than completing an operation quickly.

### Compatibility without permanent inheritance

Cratebug should read relevant BentoMod formats, but it does not need to continue writing every BentoMod-specific format.

### Small and inspectable

The project should begin with simple boundaries and few abstractions. New layers must solve demonstrated problems.

### Honest recovery

Cratebug must not claim success unless the resulting filesystem state has been checked. When complete rollback is impossible, it must report the final state clearly.

## 4. Platform and technology

Cratebug is Windows-first and initially targets Windows 10 64-bit version 1909 or newer.

The initial stack is:

- Go
- Wails v2
- React
- TypeScript
- Vite 8
- Bun
- Biome

The repository must pin stable, non-preview tool and dependency versions. Upgrades are deliberate maintenance work, not incidental feature changes.

Cratebug begins in a fresh repository and initially uses GPL-3.0.

## 5. Core terminology

### Mod

A logical piece of installed game content represented by one primary file and, when applicable, related sidecar files.

### Primary file

The file used to determine whether a mod is enabled or disabled.

Recognized primary forms:

- `.pak` - enabled
- `.pak_crateoff` - disabled by Cratebug
- `.bak_bento` - disabled by BentoMod
- `.pak_disabled` - legacy disabled form

### Sidecar

A file associated with the same mod filename stem.

Known sidecars:

- `.utoc`
- `.ucas`

`.sig` is excluded unless later evidence shows that Cratebug must manage it.

### Classic mod

A mod represented by a primary PAK without an IoStore bundle.

### IoStore mod

A mod represented by a primary file with associated `.utoc` and `.ucas` files.

### Incomplete bundle

A related file group that appears to be missing a required member. Exact IoStore validity rules must be established through fixtures and UAssetToolRivals investigation before Cratebug mutates incomplete bundles.

### Asset conflict

Two or more enabled mods contain overlapping internal Unreal asset paths.

This is distinct from a destination filename collision, invalid bundle, or duplicate priority.

## 6. Core workflows

Cratebug should eventually allow users to:

- Locate or configure the Marvel Rivals mod directory.
- Scan and display installed mods and physical folders.
- Search, filter, and inspect the library.
- Enable and disable mods.
- Rename mods.
- Change filename-based priority.
- Move mods between folders.
- Create, rename, and organize folders.
- Delete mods through a recoverable Windows deletion mechanism.
- Install supported local archives.
- Review installation collisions before replacement.
- Inspect asset conflicts through UAssetToolRivals.
- Attach and preserve metadata such as tags.
- Refresh after external filesystem changes.
- View clear progress and failures for long-running operations.

The roadmap determines implementation order.

## 7. Bundle behavior

Cratebug must treat a primary file and its recognized sidecars as one logical bundle during controlled operations.

Rename, move, priority, deletion, and installation operations must be planned against the complete recognized bundle before mutation begins.

Cratebug must not silently modify only part of a bundle and then report success.

After an operation, Cratebug must inspect affected paths and reconcile its displayed state with the filesystem.

Controlled moves and renames must preserve Cratebug metadata. The internal identity mechanism remains an implementation decision.

External renames may initially appear as one removed mod and one newly discovered mod.

## 8. Enabled and disabled formats

Cratebug writes its native disabled format:

```text
Example_9999999_P.pak
Example_9999999_P.pak_crateoff
```

Disabling changes the primary filename only. Associated `.utoc` and `.ucas` files retain their ordinary names.

Cratebug must read and enable:

- `.pak_crateoff`
- `.bak_bento`
- `.pak_disabled`

Enabling restores the primary filename to `.pak`, subject to collision and safety checks.

Cratebug does not write `.bak_bento` for newly disabled mods.

## 9. Priority and folders

Marvel Rivals mod priority is represented through filenames. Cratebug must recognize established BentoMod patterns, including a leading `!` and trailing runs of nines.

Changing priority may rename the primary and recognized sidecars. Multiple unrelated mods may share a priority; only actual path collisions or safety failures block the operation.

Folders are physical directories beneath the configured mod root. Nested folders are supported. The root mod directory cannot be renamed or deleted through Cratebug.

Virtual collections are outside the initial scope.

## 10. Filesystem safety

Before changing files, Cratebug must:

1. Determine exact source and destination paths.
2. Confirm affected paths remain inside the configured mod root or approved staging directory.
3. Identify the complete recognized bundle.
4. Detect missing sources and destination collisions.
5. Check whether Marvel Rivals is running when the operation is unsafe during gameplay.
6. Produce an operation plan before applying mutations.

Installation must use staging and reject path traversal, unsafe links, and destination escape.

Failed or cancelled installation must not leave a partial bundle presented as installed.

Multi-step operations must attempt rollback when a later step fails, then rescan affected paths and report the exact final state.

Safety restrictions must be enforced by the Go application layer. Disabled React controls are not sufficient enforcement.

## 11. Game-running protection

Read-only scanning may occur while Marvel Rivals is running.

Mutating operations are blocked by default while the game process is running, including:

- Enable or disable
- Rename
- Priority changes
- Move
- Delete
- Installation or replacement

An advanced override is deferred and must never be enabled by default.

## 12. Deletion

Normal deletion must use the Windows Recycle Bin or another recoverable Windows shell operation.

The interface should retain a short deliberate delay before destructive confirmation. This is an extra guard and does not replace backend validation.

Permanent deletion is not required initially. Cratebug must never silently fall back from recoverable deletion to permanent deletion.

## 13. Persistence and metadata

Cratebug-owned settings and metadata must be stored separately from BentoMod data.

Persisted data must:

- Include an explicit schema version.
- Be validated when loaded.
- Be written without destroying the last known-good version first.
- Preserve a recoverable previous version where practical.
- Avoid machine-specific paths in portable mod metadata.
- Tolerate malformed or outdated data without corrupting the library.

The choice between JSON, SQLite, or another format is deferred until real access patterns are known.

Tags and metadata must survive controlled renames and moves.

## 14. BentoMod compatibility

BentoMod is a read-only behavioral and visual reference.

Cratebug must not modify BentoMod's repository, installation, or persisted state during normal operation or migration.

A future optional importer may support selected data such as:

- Game path
- Custom tag catalog
- Mod tag assignments
- Approved appearance preferences

Import must preview changes and require confirmation. Unsafe bypass settings, launcher state, updater state, and crash-monitor state should not migrate automatically.

Direct reuse of BentoMod code or project-authored assets must respect GPL-3.0. Separately licensed assets may follow their own licenses.

## 15. UAssetToolRivals

UAssetToolRivals is a core dependency for Unreal archive inspection and transformation.

Cratebug must access it through a narrow boundary so that:

- Tool errors become useful application errors.
- Version compatibility can be checked.
- The integration can be tested independently.
- Tool failure does not leave filesystem operations in an unknown state.
- The integration mechanism can change without rewriting the frontend or core behavior.

The choice between NativeAOT FFI and a helper process is deferred pending focused review and a small prototype.

Only required operations should be integrated.

## 16. User interface direction

Cratebug should preserve the useful information architecture of BentoMod rather than redesigning the application from scratch.

The frontend begins from a clean React, TypeScript, and Vite project. BentoMod components, CSS, and assets may be migrated selectively and refactored as they are introduced.

Migrated code should:

- Use understandable component boundaries.
- Separate rendering from application and filesystem operations.
- Avoid oversized components and tangled shared state.
- Use typed application bindings.
- Support keyboard navigation and common Windows scaling.
- Support light, dark, and system appearance.
- Preserve the playful identity without reducing usability.

Reference screenshots and a standard viewport will be documented during frontend migration.

## 17. Responsiveness, security, and privacy

Filesystem, archive, and UAssetToolRivals work must not block frontend rendering.

Startup and explicit refresh may perform full scans. Successful Cratebug-owned operations should normally reconcile only affected paths. Cratebug may perform a recovery scan when incremental state cannot be trusted.

Cratebug must:

- Treat archives and mod contents as untrusted input.
- Reject path traversal and mutations outside approved directories.
- Avoid executing mod-provided programs or scripts.
- Avoid requiring administrator privileges for routine use.
- Avoid collecting or transmitting user data unless a future feature explicitly requires consent.
- Log diagnostic information without unnecessarily exposing private data.

## 18. Initial non-goals

The initial release does not require:

- Cross-platform support
- Game launching
- Automatic application updates
- Full mod-directory backup and restore
- Crash monitoring
- Browser-extension or deep-link intake
- Signature bypass tooling
- VFX updating
- Character database updating
- Automatic recompression tools
- Virtual collections
- Permanent deletion
- Perfect detection of external renames

## 19. Deferred decisions

| Decision | Trigger |
|---|---|
| Exact patch versions | Repository initialization |
| Persistent mod identity | Metadata and controlled rename work |
| JSON versus SQLite | Persistence prototype |
| Exact incomplete IoStore rules | Fixtures and UAssetToolRivals investigation |
| FFI versus helper process | UAssetToolRivals prototype and external review |
| Canonical UI screenshots and viewport | Frontend migration |
| Installer branding and upgrades | First distributable build |
| Advanced game-running override | Post-core safety review |

## 20. Initial release standard

Cratebug is ready for an initial stable release when:

- It installs and uninstalls conventionally on supported Windows systems.
- It discovers and displays a real Marvel Rivals mod library.
- It recognizes native and supported legacy disabled formats.
- Core mutations operate on complete recognized bundles.
- Unsafe paths, collisions, and malformed archives are rejected before mutation.
- Failed operations report their final filesystem state.
- Normal deletion is recoverable.
- Game-running safety is enforced by the backend.
- Metadata survives controlled renames and moves.
- Long-running operations do not freeze the interface.
- Core workflows have automated tests using disposable fixtures.
- Principal interface states have been reviewed in the running application.
