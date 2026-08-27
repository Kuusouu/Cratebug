# Cratebug Active Tasks

**Phase:** 10 - Automatic updates, remote mod downloads, and release hardening
**Status:** In progress.

This file contains only the active phase. Replace it when Phase 10 is complete.

## Phase objective

Give Cratebug a real release pipeline and the ability to update itself. A CalVer-tagged GitHub Actions workflow builds the installer and publishes a GitHub release with changelog notes drawn from `CHANGELOG.md`. An installed Cratebug can check for a newer release, download it, apply it silently in place through the existing per-user NSIS installer, and relaunch showing what changed. A mod can also be installed from a URL, reusing Phase 8's staged-install pipeline unchanged. Phase 11 (release hardening) is folded in here rather than sequenced after, since the update/apply flow needs a real, installable release to test against, and testing that release is most of what release hardening means in practice.

BentoMod's `check_for_updates`/`download_update`/`apply_update` (`archive/BentoMod/bentomod/src/main_tauri.rs:5403-5820`) and its `UpdateAppModal.tsx` are behavioral references for this phase: GitHub Releases API, a downloaded-asset staging step, a short-lived helper that waits for the app to exit before replacing it, and a changelog rendered from the release body. Cratebug's version scheme, installer mechanism, and silent-apply flow differ, per the design decisions below.

## Design decisions (from review discussion)

* **Versioning:** CalVer, `YYYY.MM.DD` with an optional `-rcN` prerelease suffix, matching the scheme already used in the user's `drvctl` project (`.github/workflows/publish.yml` there is the direct reference for the release workflow's tag-resolution and prerelease-detection logic).
* **Changelog:** `CHANGELOG.md` in the repo is the source of truth; the release workflow extracts the tagged version's section into the GitHub release body. The app fetches and renders that release body at runtime, it does not re-fetch `CHANGELOG.md` separately.
* **Release trigger:** pushing a tag matching `[0-9][0-9][0-9][0-9].[0-9][0-9].[0-9][0-9]*` fires `.github/workflows/release.yml`, plus a `workflow_dispatch` input for rebuilding an existing tag. Mirrors `drvctl`'s pattern.
* **Apply mechanism:** silent NSIS install (`/S`) in the background, then automatic relaunch. No second installer UI appears mid-update.
* **Testing releases are real GitHub releases.** Every tag push, `gh release create`, and release deletion during this phase's end-to-end testing is a visible-to-others, only-partially-reversible action and needs your explicit go-ahead at the time it happens, not blanket approval from this file.

## Exit criteria

* An older installed build detects and offers a newer published release, applies it without losing user settings or metadata, and relaunches showing the changelog.
* A URL-sourced download goes through the same staging, validation, and preview as a local archive install (Phase 8), with no new trust boundary.
* Update and download failures report clearly and never leave a partially-applied install or a corrupted running installation.
* Clean systems can install, run, upgrade, and uninstall.
* No known critical data-loss issue remains anywhere in the app, not just this phase's new surface.
* Release artifacts are reproducible from the tag-triggered workflow, not a local build.
* Review approves the update, remote-download, and release flows, including at least one real end-to-end test against a published (then deleted) prerelease.

## Out of scope

Phase 10 does not include:

* Silent or forced background updates without the user seeing and confirming the update first
* Browser-extension or deep-link mod intake
* Cross-platform packaging
* Code-signing the installer (no certificate exists yet; unsigned-app warnings remain, as already noted in README.md)

## 10.1 CalVer release workflow

* Add `CHANGELOG.md` at the repo root, Keep a Changelog style, one section per version.
* Add `.github/workflows/release.yml`: triggers on a tag push matching `[0-9][0-9][0-9][0-9].[0-9][0-9].[0-9][0-9]*` and on `workflow_dispatch` with a tag input, following `drvctl`'s tag-resolution and hyphen-based prerelease-detection pattern.
* Normalize the zero-padded date into integer components for `wails.json`'s `info.productVersion` before the build, since NSIS's `VIProductVersion` rejects the same leading-zero form MSBuild does in the `drvctl` workflow; keep the literal tag as the display/informational version.
* Inject the resolved version into the Go binary at build time (`-ldflags -X`), since nothing today exposes an app version to the Go layer. This is required both for the update-check comparison in 10.2 and for showing the running version somewhere in the UI.
* Build with the same `wails build -nsis -installscope user` invocation CI already runs; extract the tagged version's `CHANGELOG.md` section as release notes; `gh release create` uploading the installer, with `--prerelease` when the tag has a hyphen.

## 10.2 Backend: update check, download, silent apply

* New `internal/update` package, pure Go, independent of Wails and React, following the existing package-boundary pattern.
* Fetch the latest (or, for testing, a specific) GitHub release for this repo; compare its tag against the build's embedded version.
* Version comparison: CalVer's zero-padded `YYYY.MM.DD` sorts correctly as a plain string compare; only the `-rcN` suffix needs explicit handling (a non-prerelease outranks any `-rcN` of the same date). Don't pull in a semver library, since a hyphenated CalVer tag isn't valid strict semver anyway.
* Download the installer asset to a temp directory with retry on transient failures (BentoMod's `download_update` is a reference for the retry shape, not a template).
* Treat the downloaded installer as untrusted input matching SPEC.md's archive-trust rules: HTTPS only, verify the asset URL actually belongs to this repo's release, never execute anything before the full file is on disk.
* Apply: since a running exe can't replace itself on Windows, spawn a short-lived helper that waits for Cratebug to exit, runs the installer with `/S` against the existing per-user NSIS template, then relaunches `Cratebug.exe`. Verify `/S` actually works silently against the current Wails-generated NSIS template before relying on it; this hasn't been exercised before.
* Wails-bound methods on `App` for check/download/apply. Download needs real progress reporting (`runtime.EventsEmit`) rather than the single-synchronous-call pattern classification and conflict detection used, since a multi-second, cancellable installer transfer is genuinely different from those two.

## 10.3 Frontend: "what's new" update UI

* An update check surfaces from Settings (or another header entry), matching existing busy-indicator conventions.
* On an available update: a changelog modal titled "What new crates are there?" rendering the fetched release-body markdown, with a footer reading "Brought to you with <3 by the maintainer(s) of Cratebug." Reimplement changelog block-parsing as a small Cratebug component; BentoMod's `parseChangelog`/`renderChangelogBlocks` (`UpdateAppModal.tsx:24-143`) is a reference for the heading/list/bold shape, not something to port verbatim.
* Download progress and an "Install & Restart" action once ready; Cratebug exits itself once the apply helper has been launched.
* Show the same modal once after an applied update relaunches Cratebug, scoped to "what changed since the version you last saw" (tracked via existing metadata persistence), so the update is visible without the user hunting for it.

## 10.4 Remote mod downloads

* A URL-entry surface (dialog or similar) that downloads a mod archive into the existing Phase 8 staged-install pipeline unchanged.
* Downloaded bytes go straight into Phase 8's staging area; nothing is presented as installed before Phase 8's existing preview/collision/apply flow completes.
* Failure and cancellation reporting matches Phase 8's existing archive-install failure states; no new trust or validation path.

## 10.5 Release hardening

* Installer branding, upgrade-in-place behavior (re-running the NSIS installer over an existing per-user install without creating a duplicate entry), and uninstall correctness.
* License and third-party notices reviewed for completeness (the UAssetToolRivals worker notices from Phase 7 are already handled; confirm nothing new needs adding).
* Clean-machine Windows 10 and 11 install, upgrade, and uninstall testing.
* Accessibility, scaling, performance, security, and recovery review across the app as a whole, matching SPEC.md section 20's initial-release standard.
* User documentation for install, update, and remote mod download.

## 10.6 End-to-end release testing, then validate and review

* Push one or more real `-rcN` prerelease tags to exercise the actual workflow end to end: build, publish, then drive an intentionally-older local build through check, download, silent apply, relaunch, and the "what's new" modal against a real published release. Each tag push and release creation needs your explicit approval at the time.
* Delete the `-rcN` test releases and tags once the flow is verified, then cut the first real tagged release.
* Run `check.ps1`.
* Build synthetic/disposable fixtures for `internal/update`'s version-comparison and download-retry logic.
* Launch the app and verify the changelog modal, download progress, and post-update relaunch against a real or staged release.
* Create `docs/reviews/phase-10-review.md` covering all new workflows.

**Verify:** Review approval grants permission to consider Cratebug ready for its first public release.
