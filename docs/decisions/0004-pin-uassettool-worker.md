# 0004: Pin the UAssetToolRivals worker release

- Status: Accepted
- Date: 2026-08-25
- Updated: 2026-11-09

## Context

Task 6.2 needs a specific UAssetToolRivals worker release pinned by version, source revision, and checksum, per `docs/decisions/0003-uassettoolrivals-boundary.md`'s decision to use a supervised helper process driving a prebuilt self-contained CLI.

`ROADMAP.md` originally said the worker would come "from the maintained UAssetToolRivals fork," meaning `mewclouds/UAssetToolRivals`. In August 2026 that fork had zero published releases and its tracked commit had diverged from upstream, so Cratebug pinned upstream `XzantGaming/UassetToolRivals` `v1.5.6` (`952bd331`) instead. Owning a fork was left open if a concrete need to diverge or control the release pipeline arose.

That need arrived with upstream `main` after `v1.5.6`: extract/pack of plugin and engine mounts, `chunknames` omitted from companion PAKs unless opted in, and faster IoStore listing. Upstream did not tag those commits, and its unpublished `release.yml` switched to a framework-dependent publish that needs a .NET 8 runtime. Cratebug's installer still expects a self-contained `UAssetTool.exe` with no runtime. The managed fork now tracks upstream `main` and publishes that self-contained CLI.

## Decision

Pin UAssetToolRivals from the managed fork:

| Field | Value |
|---|---|
| Repository | `mewclouds/UAssetToolRivals` |
| Release tag | `v1.6.0` |
| Target branch | `main` |
| Source revision (commit) | `e266253f0040dcb0f9160dc74e6789c7998f1b3f` |
| Release asset | `UAssetTool-win-x64.zip` |
| Asset size | 35,975,724 bytes (~34.3 MB) |
| SHA-256 | `82096dedebe8b5c1760b7da401341c2a7d141ed46328600f8b87180c8c546f8b` |
| Linux release asset | `UAssetTool-linux-x64.tar.gz` |
| Linux asset size | 34,646,111 bytes (~33.0 MB) |
| Linux SHA-256 | `85dd64ddeaa9e551d7fd61778aff96465cfe83137d4127f30171939ddc9b92c0` |
| Published | 2026-11-10T01:51:30Z |
| Worker's own reported version | `UAssetTool v1.6.0+e266253f0040dcb0f9160dc74e6789c7998f1b3f` |

`v1.5.9` changes only how the worker is published: self-contained still, but no longer a single file. `v1.5.8` was a 69.6 MB single-file bundle, and Windows Defender scanned it in full on its first execution — measured at 8.1 s for a freshly written copy, against 52 ms once the verdict was cached. Because the verdict is keyed to the file, that cost returned with every worker update and dominated Cratebug's first classification after one. The same publish as a 148 KB apphost beside its runtime DLLs measured 1.2 s under the same conditions, and stops the worker unpacking native libraries into `%TEMP%` on first run, which left an orphaned directory behind on every version bump. The archive now carries the whole publish directory, so `build/uassettool/` and the installer's `uassettool/` hold ~199 files rather than one executable.

`v1.5.10` adds direct IoStore decryption through the CLI and JSON interface. It decrypts existing blocks without asset conversion or recompression. Cratebug calls this operation for staged bundles.

`v1.6.0` adds the batch game patch check. Cratebug sends the selected IoStore bundles and mapping paths in one request. The worker scans package headers in parallel and shares the game index and parsed mappings across the check.

Both release asset hashes were verified locally against fresh downloads and matched GitHub's recorded digests. The extracted Windows binary also reported the pinned commit in its `--version` output.

`fetch-uassettool.ps1` at the repository root downloads this exact asset, verifies its SHA-256 before trusting it, extracts it to `build/uassettool/` (already covered by `.gitignore`'s `*.exe` and `/build/` rules, so the ~73 MB extracted binary is never committed), and then runs `--version` to confirm the extracted binary's reported source revision still matches the value pinned above. A checksum mismatch deletes the downloaded file and throws rather than silently continuing with an unverified binary; this was tested directly by temporarily pointing the script at a deliberately wrong checksum and confirming it rejects the download and cleans up.

The worker itself is licensed under GPL-3.0, the same license as Cratebug, so bundling it as a separate supervised process introduces no license conflict. Its own bundled third-party components (UAssetAPI, repak, retoc, Json.NET, all MIT) are reproduced in `THIRD_PARTY_NOTICES.md`.

## Reproducing this pin

From a clean checkout, run `.\fetch-uassettool.ps1` from the repository root. It requires no .NET SDK or other tooling beyond PowerShell itself, since the release is a self-contained publish. It extracts as a directory of files rather than one executable, and leaves the verified archive in place for its own fast path, which is why the installer excludes `*.zip` when packaging that directory.

The installer also excludes `oo2core*.dll`. The worker downloads that Oodle library itself the first time it needs to decompress an Oodle-compressed container, writing it next to its own executable, so it appears in `build/uassettool/` after any extract or rebuild runs. It is proprietary RAD/Epic code that Cratebug has no license to redistribute, and `THIRD_PARTY_NOTICES.md` covers only the worker's MIT components. Shipping whatever happened to be in a local build directory would also make a developer's installer differ from CI's. Re-running it is a no-op once the pinned worker is already present and verified; it only re-downloads when the cached copy fails the checksum check.

## Re-pinning

Re-pin this table, both fetch scripts, and `PinnedSourceRevision` in `internal/uassettool/worker.go` together when:

- A newer fork release fixes a bug or adds an operation a later Cratebug task needs.
- The pinned release is ever pulled or altered (the local checksum check would start failing every fetch, which is itself the signal to re-pin or investigate).
- Upstream publishes a self-contained tag Cratebug would rather pin than the fork.

Never adjust the checksum to match an unexpected download without first confirming why it changed; a checksum that suddenly does not match the recorded value for the same tag is a signal to investigate, not to update quietly.

## Sources

- [UAssetToolRivals v1.6.0](https://github.com/mewclouds/UAssetToolRivals/releases/tag/v1.6.0) (release metadata and asset digests)
- Local SHA-256 against fresh Windows and Linux release downloads
- The extracted Windows binary's own `UAssetTool.exe --version` output
- `UAssetToolRivals/NOTICE.md` and `UAssetToolRivals/LICENSE` in the local clone (third-party notices and the worker's own GPL-3.0 license)
- `.gitignore` (confirms `build/` and `*.exe` are already excluded, so no new ignore rule was needed)
