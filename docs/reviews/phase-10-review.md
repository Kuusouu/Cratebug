# Phase 10 Review

**Date:** 2026-08-27
**Status:** Approved

## Outcome

Phase 10 gives Cratebug a real release pipeline and the ability to update itself. A CalVer tag push (`YYYY.MM.DD` with optional `-rcN`) triggers a GitHub Actions workflow that builds the Windows installer with the tagged version embedded in the Go binary and publishes a GitHub release whose notes are the tagged `CHANGELOG.md` section. An installed build can check for a newer release on demand, download the installer with progress reporting, and apply it silently in place through a detached helper that waits for the app to exit, runs the per-user NSIS installer with `/S`, and relaunches. A one-time "what's new" modal shows the running release's changelog after an update. A mod can also be installed from a direct HTTPS URL, flowing into Phase 8's staged-install pipeline unchanged. Phase 11 (release hardening) was folded into this phase per the roadmap: installer scope, upgrade-in-place behavior, user documentation, and the release itself were built and exercised together.

Two independent code-review passes were run over the branch before approval. Their findings — an update-download path inheriting the release-metadata request timeout, unsanitized release-asset names reaching the filesystem, a missing installer-existence check before the app quits, missing liveness bounds on remote mod downloads, batch-script line endings and expansion hazards, and workflow script interpolation — were fixed and unit-tested, or recorded as deferred below.

---

## CalVer release workflow (10.1)

`.github/workflows/release.yml` triggers on a tag matching `[0-9][0-9][0-9][0-9].[0-9][0-9].[0-9][0-9]*` plus a `workflow_dispatch` tag input. It validates the tag shape, strips zero-padding for NSIS's `VIProductVersion` while keeping the literal tag as the display version, injects it into the binary via `-ldflags "-X main.AppVersion=..."`, builds with the pinned `wails@v2.13.0` and `-nsis -installscope user`, extracts the tagged `CHANGELOG.md` section (failing loudly when absent), and publishes with `gh release create`, `--prerelease` on a hyphen. All GitHub expression interpolation flows through `env:` variables per GitHub's hardening guidance. Three runs of the workflow against `2026.08.27` all completed successfully, including the final re-tagged run that produced the published release.

## Update check, download, and apply (10.2)

`internal/update` is a pure-Go package independent of Wails and React. `ParseVersion`/`IsNewer` implement CalVer comparison with the documented simplification that same-date prerelease suffixes compare as strings (see limitations). `Client` fetches the latest or a specific release, validates that the asset URL is HTTPS, hosted on `github.com`, and inside this repo's `releases/download/` path, and downloads with retry on 429/5xx/network failures. Downloads write to a `.download` temp name renamed only on success, are bounded by response-header and read-idle timeouts rather than an overall cap, and reduce the asset name to a bare filename so a hostile value can never write outside the download directory. `ApplyUpdate` renders a cmd.exe helper (full `%SystemRoot%\System32` command paths, PID-based exit wait, `/S` silent install, relaunch, self-delete) written with CRLF endings, no delayed expansion, and `%`-escaped embedded paths. The Wails-bound `App` methods treat their arguments as untrusted: `ApplyUpdate` confines the installer path to the download directory, requires the `.exe` extension, and confirms the file still exists before the app quits; `DownloadUpdate` sanitizes the asset name before it reaches the filesystem. Unit tests cover version comparison, release parsing, retry classification, stall handling, name sanitization, path rejections (outside dir, non-.exe, traversal, missing file), and rendered script content.

## What's-new UI (10.3)

The Settings dialog gained an Updates section showing the running version and a check button. On an available update, a modal titled "What new crates are there?" renders the release body through a small changelog parser (headings, list items, paragraphs, inline `**bold**`), with download progress, an "Install & restart" action, and a "View release" link out to GitHub. After an applied update relaunches Cratebug, `CheckWhatsNew` shows the same modal once per version, tracked by the persisted `Settings.LastSeenVersion` field, which loads as empty for documents written before the field existed.

## Remote mod downloads (10.4)

`InstallFromURL` downloads the archive from a user-provided HTTPS URL via `install.DownloadRemoteFile` (HTTPS-only, filename resolved from Content-Disposition or the URL path and reduced to a bare name, response-header and read-idle liveness bounds, self-cleaning temp directory) and then runs the identical `stageAndPreview` flow as a local file selection, so collision checks, validation, preview, and apply are shared code rather than a parallel path. The URL dialog's client-side HTTPS check is documented as a courtesy; the backend re-validates. Failure reporting matches the Phase 8 install-failure states.

## Release hardening (10.5)

The installer stays per-user with no administrator requirement. Clean installation and upgrade-in-place were both exercised live by the maintainer (see 10.6): the update flow's silent `/S` install runs the NSIS installer over the existing installation. `docs/USER_GUIDE.md` documents installation, the update flow, and URL installs, including the unsigned-binary SmartScreen warning. `CHANGELOG.md` is established as the release-notes source of truth. License and third-party notices were reviewed; the Phase 7 worker notices remain the only ones required.

---

## Commands and tests run

```powershell
.\check.ps1
```

```powershell
go test ./... -count=1 -v
```

```powershell
bun test
```

Validation output on the merged master state:

- **Go formatting (`gofmt`):** Passed with 0 changes.
- **Frontend checks:** Biome format check, Biome lint (0 errors, 0 warnings), TypeScript typecheck, and the Vite production build all passed clean.
- **Go vet (`go vet ./...`):** Passed with 0 warnings.
- **Go test suite (`go test ./... -count=1 -v`):** 372 tests/subtests across all 9 packages passed, uncached. Phase 10's new coverage includes `internal/update` (version parsing/comparison, release client against `httptest` servers, asset-URL validation, download retry/stall/atomicity, asset-name sanitization, apply-script content and staging-clear behavior), remote-download tests in `internal/install` (naming, HTTPS rejection, stall, progress, cleanup), root-package `ApplyUpdate` rejection tests, and `LastSeenVersion` persistence tests in `internal/metadata`.
- **Frontend test suite (`bun test`):** 48 tests passed, 0 failed, 95 expect calls.

## End-to-end release evidence (10.6)

- **Release workflow runs (GitHub Actions):** three successful runs against the `2026.08.27` tag, including run 33124101958 (the first published release) and run 33132613646 (the re-tagged final release, built from the post-hardening commit `4ba2608`, 2m23s).
- **Published release:** "Cratebug 2026.08.27", not marked prerelease, exactly one asset (`Cratebug-amd64-installer.exe`), body matching the `## [2026.08.27]` CHANGELOG section verbatim.
- **Live install and update test (maintainer-performed):** the maintainer performed a clean install of the published installer, then drove that installed build through check for updates, download, silent apply (upgrade-in-place over the existing install), and automatic relaunch, and confirmed the running version afterwards. Reported as working; see the timing note in limitations.
- **CI:** the master CI workflow passed after the Phase 10 merge.
- **Screenshots:**
  - `docs/screenshots/phase-10/installed-settings-updates-section.png` — the installed build's Settings > Updates section showing "Version 2026.08.27" against the live library.
  - `docs/screenshots/phase-10/settings-updates-section.png`, `check-for-updates-result.png`, `check-for-updates-toast-fixed.png`, `install-from-url-preview.png`, `hscroll-investigation.png` — development-phase verification of the update check, result/toast states, URL-install preview, and a scaling-fix investigation.

## Known limitations and deferred findings

1. **Live update test timing.** The maintainer's live end-to-end update ran against the first published build, which predates the final review-hardening commits. The republished release contains that hardening, which is covered by the unit suites, but the first live exercise of the hardened download/apply path will be the next real update from an installed 2026.08.27 to a future release.
2. **No user-facing cancel for the update download.** Escape is suppressed mid-download and no cancel action exists, matching the in-code comment ("not cancellable yet"). A cancel button wired to a per-download cancellable context is deferred.
3. **URL-download progress is emitted but not displayed.** `install:progress` events from a remote mod download have no frontend listener yet, so the preparing phase shows only a spinner. This carries over the existing staging-progress pattern and is deferred as its own task.
4. **Same-date prerelease ordering beyond rc9.** Prerelease suffixes compare as strings, so `rc10` sorts below `rc9` of the same date. Documented in `internal/update/version.go`; acceptable at the current release cadence, with a numeric-rc comparison as the ready fix if cadence changes.
5. **Unsigned installer.** SmartScreen's "unrecognized app" warning remains, as documented in `docs/USER_GUIDE.md` and excluded from scope in `TASKS.md`.

## Review approval

**Decision:** Approved. All Phase 10 tasks (10.1 through 10.6) and exit criteria are met per the canonical checks, the release-workflow runs and published release, and the maintainer's live update verification. Cratebug is ready for its first public release.
