# 0011: UTOC signature bypass tool

- Status: Accepted
- Date: 2026-10-10

## Context

Marvel Rivals refuses to load modded files: its signature checking requires a
matching `.sig` for each `pak`/`utoc`/`ucas`. DeathChaos25's UTOC Signature
Bypass Patch (Nexus Mods 2940) disables that check by loading a plugin through
ThirteenAG's Ultimate ASI Loader: `dsound.dll` and a `plugins\` folder go next
to the game executable, which is `MarvelGame\Marvel\Binaries\Win64` on both
Steam and Epic installs. Without these files, no mod loads at all; installing
them by hand means extracting an archive into the game directory, the one
place Cratebug otherwise avoids touching. Task 22.1 offers it from the Tools
dialog instead.

## Decision

Fetch the author's own GitHub release at build time, pin it by hash, bundle
it into the installer, and install it into detected game folders on request:

| Field | Value |
|---|---|
| Repository | `DeathChaos25/MarvelRivalsUTOCSignatureBypass` |
| Release tag | `1.0.0` |
| Source revision (commit) | `8796909e744b3e6a1785cbf910243ec5573c42e5` |
| Release asset | `Marvel.Rivals.UTOC.Signature.Bypass.Patch.zip` |
| Asset size | 199,690 bytes |
| Asset SHA-256 | `4d01514adc70629628a0b89d37ed24e4805a879eabef0f48267a263f37f3d70b` |
| Bypass plugin (`.asi`) size | 44,032 bytes |
| Bypass plugin SHA-256 | `2f8b183149f30c94319a5f6636491ff39aa22afbbf3f5cbc5fb8f7ecc287a62d` |
| ASI loader (`dsound.dll`) size | 415,232 bytes |
| ASI loader SHA-256 | `bb8767f918c52a2ad055d2de9baffd2478598643b9894f09abd20d1f1ffd170c` |
| Published | 2025-05-03 (zip asset added 2026-04-14) |

The local downloads matched GitHub's recorded asset digests, the release
binary matches the tag's `OutputDLL` source blob
(`4bb15be0ea753bc6258945884b0082e24c57f48d`), and the same payload sits in the
release's `.7z` asset with identical per-file hashes. `Ultimate ASI Loader
License.txt` rides in the zip; the plugin's own license does not, so
`fetch-sigbypass.ps1` also downloads `LICENSE.txt` from the release tag
(`20c17d8b8c48a600800dfd14f95d5cb9ff47066a9641ddeab48dc54aec96e331`)
and both license copies travel with the installer.

`fetch-sigbypass.ps1` at the repository root downloads and verifies the
archive and the license, then extracts into `build/sigbypass/` (ignored by
git, like the worker). The release workflow and CI run it after
`fetch-uassettool.ps1`; the NSIS installer ships the directory, minus the
archive, as `$INSTDIR\sigbypass`.

Distribution choice: the loader is MIT and the plugin is LGPL-2.1, both
distributable with their notices (reproduced in `THIRD_PARTY_NOTICES.md`),
but the author's Nexus page forbids uploading the file to other sites. This
repo keeps the binaries out of git and fetches them from the author's own
release instead of re-hosting them.

## Install safety rules

`internal/sigbypass` owns the game-directory rules, testable against
fixtures:

- The target directory is derived from provider detection
  (`PaksPath/../../Binaries/Win64`) and is only used when
  `Marvel-Win64-Shipping.exe` is present there.
- Installation is blocked while Marvel Rivals runs.
- The payload is hash-verified before any copy, and each file is written
  through a temporary file beside the target before being renamed in.
- A pre-existing file that is not the pinned payload is a conflict: the
  install refuses before writing anything and never overwrites it.
- Removal deletes only files that still hash to the pinned digests; it
  refuses when a path holds a different file, and removes the `plugins`
  directory only when it ends up empty, so other ASI mods keep working.
- Cratebug tracks nothing: status derives from the files on disk, so a
  bypass installed by hand reads as installed, and a game reinstall (or
  Steam file verification removing the files) reads as not installed.

Uninstalling Cratebug leaves the game-directory files in place; deleting
them would silently break mod loading after the manager is gone. Game files
are out of uninstall scope.

The payload ships only in Windows releases, so the card reports an
unavailable payload on other platforms rather than pretending to work.

## Re-pinning

Re-pin (update this document's table, the constants in
`fetch-sigbypass.ps1`, and the digests in `internal/sigbypass` together) when:

- The author publishes a newer release, typically because a game patch broke
  the bypass. The tool cannot update itself without a Cratebug release.
- The pinned tag or asset is ever pulled or altered (the checksum check
  failing on every fetch is itself the signal to re-pin or investigate).

Never adjust a checksum to match an unexpected download without first
confirming why it changed.

## Sources

- `gh release view 1.0.0 --repo DeathChaos25/MarvelRivalsUTOCSignatureBypass --json tagName,publishedAt,targetCommitish,assets` (release metadata and asset digests)
- Local SHA-256 against fresh `.zip` and upstream `.7z` downloads, plus per-file hashes inside both archives
- `git hash-object` on the extracted plugin matching the tag's `OutputDLL` blob
- The Nexus Mods 2940 page (install location, false-positive Defender note, Unmount Blocker follow-up)
- Ultimate ASI Loader (`ThirteenAG/Ultimate-ASI-Loader`, MIT) 7.7.0 version metadata in the loader binary
