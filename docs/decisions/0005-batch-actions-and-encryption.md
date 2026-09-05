# 0005: Batch actions and in-place encryption

- Status: Accepted
- Date: 2026-09-05

## Context

Phase 14 needs three decisions that later tasks should not reopen: how
multi-select relates to the existing viewing selection, where batch actions
live in the UI, and how Cratebug encrypts or decrypts an already-installed
IoStore bundle.

Cratebug today has one selection (`selectedEntryID`) and one-mod mutations.
Encrypted IoStore mods return `ErrCannotDetermineType` because Phase 6
explicitly did not manage AES keys (`internal/modtype/determine.go`,
`internal/uassettool/operations.go`). BentoMod obfuscates only at
create/update time and has no decrypt-in-place path. Decision 0003 limited
the worker surface to the operations a phase actually needs. Decision 0002
put single-mod organize actions on the context menu.

## Decision

**Viewing and checking are different.** The viewed mod (`selectedEntryID`)
still drives the details panel and the single-mod context menu. The checked
set (`checkedEntryIDs`) is the batch target. Clicking a card views it and
makes it the only checked mod. Clicking that card again clears both. Ctrl+click
toggles check without clearing the rest. Shift+click ranges over the current filtered list. There is no
always-visible checkbox on the card.

**Batch actions live in one Actions menu** in the catalog header, not a
Bento-style always-visible button row and not the selected-mod panel. The
context menu stays single-mod (0002). Enable/Disable on each card stays,
because that is still the frequent one-mod control.

**Encrypt and decrypt use the Marvel Rivals game AES key**, hardcoded in
Go, the same key BentoMod already uses so the game can load the result. The
key is a well-known public value, not a secret. It never appears in Wails
models, logs, or the frontend. The frontend only sees `Identity.encrypted`.

**In-place encryption is a rebuild.** UAssetToolRivals has no decrypt
action. Cratebug extracts the bundle (`extract_iostore`, and the companion
PAK when hybrid) and recreates it with `create_mod_iostore` and
`obfuscate` true or false, then replaces the live files. That is why the
confirm dialog must say this rewrites the bundle. Classic PAK encryption
and install-time obfuscation stay out of this phase.

**Only required write actions are added** to `internal/uassettool`:
`ExtractIoStore`, `CreateModIoStore`, and `ExtractPakAll` if hybrid
rebuild needs it. The rest of the worker surface stays unmirrored.

## Alternatives considered

A Cratebug-generated AES key would encrypt files the game cannot read. Not
selected.

`clone_mod_iostore` copies a container. It does not flip obfuscation. Not
selected.

A persistent bulk toolbar matching BentoMod would clutter Cratebug's
catalog header. The Actions menu is the smaller control.

New batch Wails methods for enable/move/tags/delete would duplicate
existing one-mod safety. Looping those methods from the frontend is
enough. Encrypt/decrypt is the one new Go mutation, because it is a
multi-file rebuild with progress and cancellation.

## Consequences

- `ListInternalPaths` lists encrypted IoStore containers with the game key
  instead of returning `ErrCannotDetermineType`. Encrypted mods can
  classify and participate in conflict scans. Phase 7's "encrypted =
  Unknown" presentation is superseded by a lock mark plus a real category.
- Mixed encryption state and ineligible formats disable Encrypt/Decrypt in
  the UI and are rejected again in Go.
- Write calls need a timeout longer than the worker's 30s default.
- Decision 0002 still governs single-mod organize actions.
