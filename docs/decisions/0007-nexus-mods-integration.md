# 0007: Nexus Mods integration

- Status: Accepted
- Date: 2026-09-06

## Context

Marvel Rivals mods are overwhelmingly distributed through Nexus Mods.
Cratebug can install a local archive (Phase 8) or a user-typed HTTPS URL
(Phase 10). Nexus download links are signed, short-lived, and issued
through its API, so the URL flow cannot reach them. Users download by
hand and feed the file to the picker.

Nexus has two download paths. Premium accounts may request a link from
the API with mod and file IDs. Free accounts may not; they must click
Mod Manager Download on the website, which fires an `nxm://` link
carrying signed `key` and `expires` parameters. That click is a Nexus
product constraint.

`App.LoadMetadata` returns the entire `metadata.Document` to the
frontend, so a field added to `metadata.Settings` is automatically
visible in the WebView. There is no credential store in the repo today.
Wails binds every exported method on `App`, including calls from
devtools.

The Wails NSIS macro `CUSTOM_PROTOCOL_ASSOCIATE` begins with an
unconditional `DeleteRegKey` of the scheme key. Adding `info.protocols`
to `wails.json` would hijack `nxm://` at install time, destroy the
previous owner before anything recorded it, and delete whoever owns the
scheme at uninstall even if that is now another app.

## Decision

**Bring-your-own-key, not SSO.** The user pastes a personal Nexus API
key. There is no shared application credential and no browser login in
this phase. The key is stored only on this machine and sent only as the
`apikey` header on user-initiated Nexus API calls.

**Two download paths, both first-class.** Premium accounts call
`download_link.json` with the API key. Free accounts start from the
Nexus website; Cratebug parses the `nxm://` link and passes `key` /
`expires` to the same endpoint. The website click is not automated,
scraped, or synthesised.

**Prompt only if taken.** `nxm://` registration defaults to on. If
nothing owns the scheme, register silently. If another application owns
it, name that owner and ask before taking over. Record the displaced
command so unregister can restore it. Never a silent grab. `selfStale`
(our basename, missing file) may be reclaimed without prompting.

**Runtime owns registration; the installer only cleans up.** Leave
`wails.json` alone because the Wails associate macro deletes the scheme
key unconditionally. Register and unregister from Go. Uninstall runs
`--uninstall-cleanup` and deletes `HKCU\Software\Classes\nxm` only if
the command still points inside `$INSTDIR`.

**Remove generic URL install.** `App.InstallFromURL`,
`InstallFromUrlDialog`, and `internal/install/download.go` go away.
Streaming, stall, and progress mechanics move into `internal/nexus`.
No code path accepts a user-typed download URL. A Nexus-scoped
downloader takes its URL from an API response and its filename from
`FileInfo.file_name`.

**DPAPI for the API key, not `metadata.json` and not Credential
Manager.** Store a DPAPI blob at `%AppData%\Cratebug\nexus.key`. There
is no `GetNexusAPIKey`. DPAPI and Credential Manager both protect
against other users and offline disk access. Neither protects against
malware running as the same user. `golang.org/x/sys` already ships the
DPAPI surface and does not ship `CredWriteW` / `CredReadW` /
`CredDeleteW`.

**Secrets never cross the Wails boundary.** The API key and signed
`nxm://` `key` / `expires` stay in Go. The frontend receives
identifiers and display metadata only.

**Read rate-limit headers; do not hardcode the numbers.** Parse
`X-RL-*` on every response. Refuse a request when remaining is zero
and reset is still in the future. Published limits have changed.

**Install only.** No in-app browsing, search, endorsements, tracked
mods, or update checking. Persist Nexus mod, file, and version IDs on
the installed mod so a later phase can check for updates.

## Alternatives considered

SSO or a Cratebug-owned Nexus application credential. Rejected. It
would put a shared secret in the client and contradict the
Acceptable Use Policy rule against storing user API keys on a server.

Keeping a generic "Install from URL" beside Nexus. Rejected. The
weakest link in that path was trusting a user-supplied URL and
guessing a filename from `Content-Disposition`.

Windows Credential Manager. Rejected for this phase. It would need
hand-rolled `CREDENTIALW` `unsafe` code or a new dependency, and its
test seam is a process-global namespace. The credential would appear
in the Windows UI, which is the real advantage; security is otherwise
equivalent. Only `internal/secret`'s implementation file changes if
this is revisited.

Installer-owned `nxm://` via `wails.json` `info.protocols`. Rejected
because the Wails macro deletes the previous owner unconditionally.

Automating the free-user website click. Rejected. It is a Nexus
product constraint.

## Consequences

- New packages: `internal/nexus`, `internal/urlscheme`, `internal/secret`.
- `metadata.Settings` may hold the protocol `Snapshot` (paths, not
  secrets). Schema version stays at 1.
- Dev builds (`AppVersion == "dev"` or an executable outside the
  install directory) must not register `nxm://`.
- A later phase can add update checks from persisted Nexus IDs
  without a re-install. Nexus application registration remains a
  gate on public distribution and is handled outside this phase.
