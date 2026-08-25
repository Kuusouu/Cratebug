# 0004: Pin the UAssetToolRivals worker release

- Status: Accepted
- Date: 2026-08-25

## Context

Task 6.2 needs a specific UAssetToolRivals worker release pinned by version, source revision, and checksum, per `docs/decisions/0003-uassettoolrivals-boundary.md`'s decision to use a supervised helper process driving the officially released self-contained CLI.

`ROADMAP.md` originally said the worker would come "from the maintained UAssetToolRivals fork," meaning `mewclouds/UAssetToolRivals` (the fork the `UAssetToolRivals` git submodule under the BentoMod archive actually tracks, per its `.gitmodules`). That fork has zero published releases, and its tracked commit (`cff29ba`, "Hybrid Iostore implementation") has diverged from upstream's own history rather than sitting on top of it — they are related but no longer identical codebases. Cutting the fork's first release would mean pushing a tag to a public repository to trigger its first-ever CI-built release, and re-validating this phase's prototype evidence against that specific build rather than the one already tested. The user decided against that for now: pin upstream's existing, already-published, actively maintained release directly, and update `ROADMAP.md`'s wording to match. Owning a fork remains available later if a concrete need to diverge or control the pipeline arises; it was not adopted speculatively.

## Decision

Pin UAssetToolRivals from upstream directly:

| Field | Value |
|---|---|
| Repository | `XzantGaming/UassetToolRivals` |
| Release tag | `v1.5.6` |
| Target branch | `main` |
| Source revision (commit) | `952bd331976c6f28efb36ca320c82c27e2456023` |
| Release asset | `UAssetTool-win-x64.zip` |
| Asset size | 31,781,186 bytes (~30.3 MB) |
| SHA-256 | `16c051cbc68bef0b9050ca83a8fd3d8d997156ed1e91f4112042f41443bdabaf` |
| Published | 2026-08-24T20:21:06Z |
| Worker's own reported version | `UAssetTool v1.0.0+952bd331976c6f28efb36ca320c82c27e2456023` |

The source revision was cross-checked two ways, not just taken from the release page: the SHA-256 above was verified locally against a fresh download and matched GitHub's own recorded asset digest exactly, and the commit hash was independently confirmed by running the extracted binary's own `--version` output, which bakes in its build commit as an informational version suffix. Both checks agree.

`fetch-uassettool.ps1` at the repository root downloads this exact asset, verifies its SHA-256 before trusting it, extracts it to `build/uassettool/` (already covered by `.gitignore`'s `*.exe` and `/build/` rules, so the ~73 MB extracted binary is never committed), and then runs `--version` to confirm the extracted binary's reported source revision still matches the value pinned above. A checksum mismatch deletes the downloaded file and throws rather than silently continuing with an unverified binary; this was tested directly by temporarily pointing the script at a deliberately wrong checksum and confirming it rejects the download and cleans up.

The worker itself is licensed under GPL-3.0, the same license as Cratebug, so bundling it as a separate supervised process introduces no license conflict. Its own bundled third-party components (UAssetAPI, repak, retoc, Json.NET, all MIT) are reproduced in `THIRD_PARTY_NOTICES.md`.

## Reproducing this pin

From a clean checkout, run `.\fetch-uassettool.ps1` from the repository root. It requires no .NET SDK or other tooling beyond PowerShell itself, since the release is a self-contained single-file publish. Re-running it is a no-op once the pinned worker is already present and verified; it only re-downloads when the cached copy fails the checksum check.

## Re-pinning

Re-pin (update this document's table and the constants at the top of `fetch-uassettool.ps1` together) when:

- A newer upstream release fixes a bug or adds an operation a later Cratebug task needs.
- The pinned release is ever pulled or altered upstream (the local checksum check would start failing every fetch, which is itself the signal to re-pin or investigate).
- Cratebug moves to owning its own fork, per the option left open in Context above.

Never adjust the checksum to match an unexpected download without first confirming why it changed; a checksum that suddenly does not match the recorded value for the same tag is a signal to investigate, not to update quietly.

## Sources

- `gh release view v1.5.6 --repo XzantGaming/UassetToolRivals --json tagName,publishedAt,targetCommitish,assets` (release metadata and asset digest, checked 2026-08-25)
- Local `sha256sum` against a freshly downloaded copy of the asset (cross-check against GitHub's recorded digest)
- The extracted binary's own `UAssetTool.exe --version` output (source-revision cross-check)
- `UAssetToolRivals/NOTICE.md` and `UAssetToolRivals/LICENSE` in the local submodule checkout (third-party notices and the worker's own GPL-3.0 license)
- `.gitignore` (confirms `build/` and `*.exe` are already excluded, so no new ignore rule was needed)
