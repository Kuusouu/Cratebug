# 0010: Worker-owned IoStore decryption

- Status: Accepted; implemented with UAssetToolRivals v1.5.10
- Date: 2026-10-04
- Supersedes: The Go implementation choice in decision 0009, after the worker pin update

## Decision

Keep the IoStore format and AES block operation in UAssetToolRivals.
Expose `decrypt_iostore` through its CLI and JSON worker interface.
Keep the direct decryption behavior from decision 0009.
Preserve compressed bytes, cooked assets, container IDs, chunk IDs, metadata, and paths.
Do not convert assets to the legacy format.

Support unsigned TOC versions 1 through 5 with one UCAS partition.
Reject unsupported layouts before replacement.
Preserve a clear obfuscated directory index.
Decrypt and validate an encrypted directory index.
A clear index cannot validate a supplied AES key.
Cratebug supplies the established Marvel Rivals key.

UAT prepares decrypted sidecars beside the input files.
It parks the original UCAS before replacement.
It restores that backup when replacement fails.
It retains and reports the backup if rollback also fails.
It leaves the companion PAK intact.

Cratebug keeps its private bundle staging and live replacement rollback.
It cleans companion metadata before the worker call.
It requires a clear encryption state in the worker response.
It verifies the staged state before live replacement.
The Go format reader is removed. Cratebug keeps bundle staging, companion cleanup, state checks, and live replacement rollback.

## Release and migration

UAssetToolRivals v1.5.10 was published on 2026-10-04.
Cratebug pins commit `c137d9abc4a7509d25f89ca9c6c0be3b7a87da23`.
Both release asset hashes match the published GitHub digests.
The tested migration now calls the released worker and removes the Go format reader.

The new worker owns IoStore parsing and AES decryption.
Cratebug owns private staging, companion policy, and live replacement.

## Reason

The Go repair corrected the unsafe legacy rebuild without a worker release.
The worker should own the archive format and cryptographic operation.
This change removes the duplicate format reader from Cratebug.
It keeps the manager responsible for bundle staging, companion policy, and live replacement.

The fixture checks prove asset and identity preservation for the tested inputs.
They do not prove that every mod works in the game.
