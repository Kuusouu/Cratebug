# Cratebug Active Tasks

**Phase:** 16 - Nexus Mods integration (BYOK)
**Status:** Active

This file contains only the active work. Do not start the next phase.

## Objective

Make Nexus Mods a first-class install source. Each user pastes their own personal API key (BYOK). Premium accounts download through the Nexus API. Free accounts must click Mod Manager Download on the Nexus website; Cratebug catches the `nxm://` link and uses its signed parameters. The generic "Install from URL" path is removed. Downloaded archives enter the existing Phase 8 staged preview and apply pipeline unchanged.

## Design decisions

* **BYOK, local only.** The user pastes a personal Nexus API key. No SSO, no shared application credential. The key never leaves the machine except as the `apikey` header on user-initiated Nexus API calls.
* **Key stays out of `metadata.json`.** `LoadMetadata` returns the whole document to the WebView. Store the key at `%AppData%\Cratebug\nexus.key` via DPAPI (`internal/secret`). There is no `GetNexusAPIKey`.
* **`nxm://` prompt only if taken.** The setting defaults to on. Register silently when nothing owns the scheme. If another app owns it, show a one-time dialog naming that owner and ask before taking over. Record the displaced command so unregister restores it. Never a silent grab.
* **Runtime owns registration, not the installer.** The Wails `CUSTOM_PROTOCOL_ASSOCIATE` macro starts with an unconditional `DeleteRegKey`, which would hijack without consent and destroy the previous owner. Leave `wails.json` alone. The uninstaller runs `--uninstall-cleanup` and only deletes `HKCU\Software\Classes\nxm` if it still points inside `$INSTDIR`.
* **Install from URL is gone.** `App.InstallFromURL`, `InstallFromUrlDialog`, the toolbar Link button, and `internal/install/download.go` are removed. Streaming, stall, and progress mechanics move into `internal/nexus`. No code path accepts a user-typed download URL.
* **Install only.** No in-app browsing, search, endorsements, tracked mods, or update checking. Persist `NexusModID` / `NexusFileID` / `NexusVersion` on the installed mod so a later phase can check for updates.
* **Free-user click is a product constraint.** Do not scrape Nexus, drive a browser, or synthesise a download key. A free account that pastes a mod URL opens the page and parks until an `nxm://` link arrives.
* **File picker when the URL has no `file_id`.** Show MAIN and OPTIONAL files, pre-selecting the primary. Do not auto-install.
* **Secrets never cross the Wails boundary.** Signed `key` / `expires` stay in Go (`linkSecrets`). The frontend sees identifiers and display metadata only.
* **Header-driven rate limits.** Read `X-RL-*` headers. Never hardcode published numbers. Pre-flight refuse when remaining is zero and reset is still in the future.
* **Dev builds do not register.** Skip `nxm://` registration when `AppVersion` is `dev` or the executable is not under the install directory.

## Out of scope

* In-app Nexus browsing, search, endorsements, tracked mods, or update checking
* Browser-extension intake
* Automating the free-user website click
* Nexus application registration (the maintainer handles that separately)
* Silent takeover of another app's `nxm://` handler
* Arbitrary-URL remote install
* Windows Credential Manager (DPAPI is the chosen store)

## 16.1 Docs

Write the ROADMAP Phase 16 entry (with the Supersedes line), this TASKS file, SPEC additions (§6 workflows, §17 privacy/Nexus, §18 non-goal narrowing), the ROADMAP deferred-list edit, and `docs/decisions/0007-nexus-mods-integration.md`.

**Verify:** ROADMAP names Phase 16 and carries Supersedes. SPEC no longer lists deep-link intake as a non-goal. 0007 states the two download paths.

## 16.2 Secret storage

`internal/secret` — neutral `secret.go`, `protect_windows.go`, `protect_other.go`, injectable seam.

**Verify:** `go test ./internal/secret/ -count=1`. Neutral tests with a fake protector under `t.TempDir()`: round-trip; missing file → `ErrNotConfigured`; `Configured()` false→true→false; `Clear` on absent → nil; corrupted ciphertext errors rather than panics; empty rejected; atomic write leaves no temp file; mode `0600`. Windows tests exercise real DPAPI in memory only (protect→unprotect equal; wrong entropy fails; truncated ciphertext fails) — no filesystem, no residue.

## 16.3 Nexus API client

`internal/nexus`: `Client` with `Validate`, `Mod`, `Files`, `File`, `DownloadLinks`; rate-limit capture and pre-flight refusal; the TTL cache; typed sentinels; header-injection rejection on the key.

**Verify:** `go test ./internal/nexus/ -count=1` against `httptest`. Cover each status → sentinel mapping, header parsing, cache hit/miss, pre-flight rate-limit refusal, and that no error string contains an API key or a `key=` parameter.

## 16.4 Link parsing and redaction

`ParseDownloadURL`, `ParseModPageURL`, `FirstDownloadURL`, `redactURL` in `internal/nexus/url.go`. All pure.

**Verify:** Table tests — premium link, free link with `key`/`expires`, wrong scheme, wrong game, collections/oauth/premium hosts, missing segments, non-numeric IDs, absurd length, an embedded `"`; both site URL shapes with and without `file_id`; `FirstDownloadURL` over empty / one / not-at-index-0 / two / a Windows path / `--uninstall-cleanup`. `redactURL` on unparseable input returns the literal placeholder.

## 16.5 Nexus download; retire the generic URL download

Move `internal/install/download.go` → `internal/nexus/download.go` with Nexus-scoped URL provenance, `FileInfo.file_name`, HTTPS-only redirects, size check, throttled progress, and `redactURL` on every error path. Delete `internal/install/download.go` and `download_test.go`. Export `install.IsSupportedInstallFile`. Port the eight existing download tests and add redirect-downgrade, size-mismatch, caller-cancellation, and `TestDownloadErrorsRedactQueryParameters`.

**Verify:** `go test ./internal/nexus/ ./internal/install/ -count=1`. Confirm no remaining reference to `DownloadRemoteFile` anywhere in the tree.

## 16.6 Metadata additions

Add the `NexusProtocol` snapshot struct to `metadata.Settings` with a `Set*` validator, and `NexusModID` / `NexusFileID` / `NexusVersion` (all `omitempty`) to `metadata.ModRecord`. Confirm the install path actually reaches `EnsureMod` for a freshly installed mod before wiring the write; if it does not, add it here.

**Verify:** `go test ./internal/metadata/ -count=1` — reject / accept / round-trip / loads-as-empty, matching `settings_test.go`. `CurrentSchemaVersion` stays at 1; assert a schema-1 document with no `nexusProtocol` field loads clean.

## 16.7 App layer

`app_nexus.go` with the bound methods, the `pendingURLMu`-guarded buffer, `linkSecrets`, `ctxReady`, the download cancel func, and the `nexus:link` event. Remove `App.InstallFromURL`. Regenerate and commit `frontend/wailsjs/**`.

A working premium download is reachable here from DevTools before any UI exists. Ask before running live-API checks.

**Verify:** `go test ./... -count=1`, including `-race`. New `app_nexus_test.go`: key set/reject (empty, whitespace-only, control characters), `NexusKeyState` false→true→false against a `t.TempDir()` store, buffer take-once semantics, concurrent Set/Take under `-race`, and the SENTINEL leak test (`nexus:link` JSON must not contain `SENTINEL`). Bindings regenerated and committed.

## 16.8 Protocol registration, single instance, uninstall cleanup

`internal/urlscheme`. `main.go`: `singleInstanceID`, `SingleInstanceLock`, `parseLaunchArgs`, the `--uninstall-cleanup` branch. `project.nsi` uninstall additions.

**Verify:** Go unit tests against a fake registry, runnable anywhere — `Status` for none/self/selfStale/other/other-with-UserChoice/machine-wide-only; `commandExecutablePath` for quoted-with-spaces, bare, quoted-with-flags, empty, unbalanced quote; `Register` records the exact `Snapshot` including a previous owner with extra subkeys; `Unregister` with a foreign owner in place is a no-op; `Unregister` restores value-for-value; the produced command is exactly `"<exe>" "%1"` for a path with a space; `deleteKeyTree` visits deepest-first. Plus an integration test against a test-only scheme (`cratebug-test-<random>`) with `t.Cleanup` — never `nxm`. Plus `main_test.go` for `parseLaunchArgs`, and `onSecondInstanceLaunch` with `ctxReady == false`.

**Manual:** cold `nxm://` click with Cratebug closed; warm click with it open; with it minimised; two clicks within ~200 ms (a second window is possible — the `FindWindowW` race); plain second double-click (foregrounds, no new window); take-over of a foreign handler and unregister restoring it; confirm a `wails dev` session does not register; confirm `os.Args[1]` carries the URL.

## 16.9 Settings UI

The Settings "Nexus Mods" section: paste-only masked key, connected state (name, Premium/Free), link to the Nexus API-key page, Disconnect, the `nxm://` handler switch (`role="switch"`), remaining-requests line, take-over confirmation, and the UserChoice warning.

**Verify:** `bun run check`, `bun test`. Screenshot each account state.

## 16.10 Install UI

Delete `InstallFromUrlDialog.tsx` and every Install-from-URL reference. Add `InstallFromNexusDialog.tsx` and `NexusLinkDialog.tsx`. Change `InstallSource`'s remote arm to `{ kind: "nexus"; modId: number; fileId: number }`. Swap the toolbar `Link` icon for `Download`. Add `nexusPresentation.ts` + tests. Add the `install:progress` and `nexus:link` listeners and the mount-effect `TakePendingNexusLink()` call.

**Verify:** `bun run check`, `bun test`. Screenshots: not-connected, connected-free, connected-premium, paste dialog, file picker, nxm confirm, download progress, free-user-needs-website-click, and each error state.

## 16.11 Verify

`.\check.ps1`, `go test ./... -count=1 -race`, `bun test`. Update `docs/USER_GUIDE.md` (getting an API key, what the handler toggle does, the free-user click) and `docs/TROUBLESHOOTING.md` (key rejected, rate limited, link expired, another app owns `nxm://`, a Windows default-apps override). Screenshots to `docs/screenshots/phase-16/task-<n>-<state>.png`. Run a real uninstall and confirm the handler is restored and `nexus.key` is gone. Stop at the review gate.
