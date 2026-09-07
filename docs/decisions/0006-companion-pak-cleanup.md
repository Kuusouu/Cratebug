# 0006: Unsupported companion PAK cleanup

- Status: Accepted
- Date: 2026-09-05

## Context

As of 3 September 2026, Marvel Rivals anti-cheat crashes if an installed
mod `.pak` still contains the leftover IoStore bookkeeping names
`chunknames` and `patched_files` (often under a `../../../` mount).
repak-gui offers a pak-only rewrite and leaves `.utoc` / `.ucas` alone.

Cratebug already ignores those names when deciding hybrid rebuild
(`CompanionPakHasRawFiles`). It does not strip them. Encrypt rebuilds the
whole IoStore triple and may write the names again. Discovery scan cannot
see inside a PAK.

## Decision

**Detect by `list_pak`.** Any primary listing whose last path segment is
`chunknames` or `patched_files` is dirty. The AES key is used only when
the IoStore index needs it. The frontend receives entry IDs and a boolean
on install preview items, not listings or the key.

**Rewrite the `.pak` only.** Extract remaining files, `create_pak`
without the metadata names, replace the live primary (including a
disabled suffix). The pinned worker rejects an empty file list, so a
metadata-only companion is rewritten with one harmless
`.cratebug-companion-stub` entry. `.utoc` and `.ucas` are not opened
for write.

**Ask once per library root per session** after the first populated scan.
Refresh and post-mutation reload do not re-prompt. Dismiss is not
persisted.

**Install strips.** The preview warns and does not block. Apply rewrites
the staged `.pak` before copy so a dirty archive never lands in the
library.

**One sequential mutation**, same shape as encrypt: find, then strip each
ID with progress and cancel. Game-running blocks the write.

## Alternatives considered

Full IoStore rebuild via `create_mod_iostore`. Rejected. It is slower,
touches the real assets, and the worker may emit the same names.

A discovery.Issue from `Scan`. Rejected. Scan is filesystem-only, and
existing issues block install.

Persisted "never ask again". Rejected for this phase. A leftover dirty
library would stay silent.

## Consequences

- `create_pak` is added to the typed worker surface.
- Encrypt strips `chunknames` / `patched_files` from the rebuilt companion
  `.pak` before replace, using the same pak-only rewrite as cleanup.
  The companion dialog is still offered first. Required-encryption
  confirm waits until that offer has settled.
- Listing errors during find skip that member. Rewrite errors during
  strip fail that member and leave its `.pak` as it was. A staged file
  that cannot be listed is copied as-is (it is not the metadata issue).
