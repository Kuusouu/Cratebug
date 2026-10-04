# 0009: Direct IoStore decryption

- Status: Accepted
- Date: 2026-10-04

## Decision

Decrypt existing IoStore compression blocks with AES-256 in ECB mode.
Keep the compression bytes, chunk IDs, container ID, package metadata, and file paths.
Clear the encrypted flag only after all staged blocks decrypt successfully.

The pinned worker encrypts existing chunks through `pak_fixer`.
Its JSON interface does not expose a complete direct decrypt operation.
Its recompress operation skips compressed inputs.
Its JSON clone operation does not pass the arguments its CLI requires.
Use the Go standard library for block decryption. Keep the worker pin unchanged.

The format reader validates the TOC and all block ranges before it writes data.
It supports unsigned containers with one UCAS partition and TOC versions 1 through 5.
Reject newer versions, signed containers, and multiple partitions.
Keep the live bundle intact when validation fails.

The pinned writer leaves directory indexes clear in obfuscated containers.
Preserve those bytes. Decrypt properly encrypted indexes and validate their structure.
Preserve block padding and physical offsets so no archive reconstruction is necessary.

Both actions use private staged copies and the existing replacement rollback.
Clean companion metadata in staging before the live files change.
No legacy asset conversion runs during either action.

## Reason

The legacy rebuild used the fixed output name `rebuilt`.
The worker derives a new container ID from that output name.
Different mods therefore received the same container ID and container header chunk ID.
The legacy conversion can also change cooked asset references.

The library scan found 57 colliding IDs among 59 bundles.
Its 1,756 decoded chunks still read successfully.
Reading one archive does not establish that the game can use colliding archives together.
