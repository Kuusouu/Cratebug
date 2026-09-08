# 0004: Pin the UAssetToolRivals worker release

- Status: Accepted
- Date: 2026-08-25
- Updated: 2026-09-08

## Context

Task 6.2 needs a specific UAssetToolRivals worker release pinned by version, source revision, and checksum, per `docs/decisions/0003-uassettoolrivals-boundary.md`'s decision to use a supervised helper process driving a prebuilt self-contained CLI.

`ROADMAP.md` originally said the worker would come "from the maintained UAssetToolRivals fork," meaning `mewclouds/UAssetToolRivals`. In August 2026 that fork had zero published releases and its tracked commit had diverged from upstream, so Cratebug pinned upstream `XzantGaming/UassetToolRivals` `v1.5.6` (`952bd331`) instead. Owning a fork was left open if a concrete need to diverge or control the release pipeline arose.

That need arrived with upstream `main` after `v1.5.6`: extract/pack of plugin and engine mounts, `chunknames` omitted from companion PAKs unless opted in, and faster IoStore listing. Upstream did not tag those commits, and its unpublished `release.yml` switched to a framework-dependent publish that needs a .NET 8 runtime. Cratebug's installer still expects a self-contained `UAssetTool.exe` with no runtime. The managed fork now tracks upstream `main` and publishes that self-contained CLI.

## Decision

Pin UAssetToolRivals from the managed fork:

| Field | Value |
|---|---|
| Repository | `mewclouds/UAssetToolRivals` |
| Release tag | `v1.5.9` |
| Target branch | `main` |
| Source revision (commit) | `7c185ae5da2ac446cf58db75ebd34f9402b8b5dc` |
| Release asset | `UAssetTool-win-x64.zip` |
| Asset size | 35,859,430 bytes (~34.2 MB) |
| SHA-256 | `51427b436046a69872fe375df425b8e17d81c460999874d6c1346449f141f6d3` |
| Published | 2026-09-08T08:46:27Z |
| Worker's own reported version | `UAssetTool v1.5.9+7c185ae5da2ac446cf58db75ebd34f9402b8b5dc` |

`v1.5.9` changes only how the worker is published: self-contained still, but no longer a single file. `v1.5.8` was a 69.6 MB single-file bundle, and Windows Defender scanned it in full on its first execution — measured at 8.1 s for a freshly written copy, against 52 ms once the verdict was cached. Because the verdict is keyed to the file, that cost returned with every worker update and dominated Cratebug's first classification after one. The same publish as a 148 KB apphost beside its runtime DLLs measured 1.2 s under the same conditions, and stops the worker unpacking native libraries into `%TEMP%` on first run, which left an orphaned directory behind on every version bump. The archive now carries the whole publish directory, so `build/uassettool/` and the installer's `uassettool/` hold ~199 files rather than one executable.

The source revision was cross-checked two ways, not just taken from the release page: the SHA-256 above was verified locally against a fresh download and matched GitHub's own recorded asset digest exactly, and the commit hash was independently confirmed by running the extracted binary's own `--version` output, which bakes in its build commit as an informational version suffix. Both checks agree.

`fetch-uassettool.ps1` at the repository root downloads this exact asset, verifies its SHA-256 before trusting it, extracts it to `build/uassettool/` (already covered by `.gitignore`'s `*.exe` and `/build/` rules, so the ~73 MB extracted binary is never committed), and then runs `--version` to confirm the extracted binary's reported source revision still matches the value pinned above. A checksum mismatch deletes the downloaded file and throws rather than silently continuing with an unverified binary; this was tested directly by temporarily pointing the script at a deliberately wrong checksum and confirming it rejects the download and cleans up.

The worker itself is licensed under GPL-3.0, the same license as Cratebug, so bundling it as a separate supervised process introduces no license conflict. Its own bundled third-party components (UAssetAPI, repak, retoc, Json.NET, all MIT) are reproduced in `THIRD_PARTY_NOTICES.md`.

## Reproducing this pin

From a clean checkout, run `.\fetch-uassettool.ps1` from the repository root. It requires no .NET SDK or other tooling beyond PowerShell itself, since the release is a self-contained publish. It extracts as a directory of files rather than one executable, and leaves the verified archive in place for its own fast path, which is why the installer excludes `*.zip` when packaging that directory.

The installer also excludes `oo2core*.dll`. The worker downloads that Oodle library itself the first time it needs to decompress an Oodle-compressed container, writing it next to its own executable, so it appears in `build/uassettool/` after any extract or rebuild runs. It is proprietary RAD/Epic code that Cratebug has no license to redistribute, and `THIRD_PARTY_NOTICES.md` covers only the worker's MIT components. Shipping whatever happened to be in a local build directory would also make a developer's installer differ from CI's. Re-running it is a no-op once the pinned worker is already present and verified; it only re-downloads when the cached copy fails the checksum check.

## Re-pinning

Re-pin (update this document's table and the constants at the top of `fetch-uassettool.ps1` together) when:

- A newer fork release fixes a bug or adds an operation a later Cratebug task needs.
- The pinned release is ever pulled or altered (the local checksum check would start failing every fetch, which is itself the signal to re-pin or investigate).
- Upstream publishes a self-contained tag Cratebug would rather pin than the fork.

Never adjust the checksum to match an unexpected download without first confirming why it changed; a checksum that suddenly does not match the recorded value for the same tag is a signal to investigate, not to update quietly.

## Sources

- `gh release view v1.5.9 --repo mewclouds/UAssetToolRivals --json tagName,publishedAt,targetCommitish,assets` (release metadata and asset digest, checked 2026-09-08)
- Local SHA-256 against a freshly downloaded copy of the asset (cross-check against GitHub's recorded digest)
- The extracted binary's own `UAssetTool.exe --version` output (source-revision cross-check)
- `UAssetToolRivals/NOTICE.md` and `UAssetToolRivals/LICENSE` in the local clone (third-party notices and the worker's own GPL-3.0 license)
- `.gitignore` (confirms `build/` and `*.exe` are already excluded, so no new ignore rule was needed)
