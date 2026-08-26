# Install fixtures

## rar test archive

`test.part01.rar` and `test.part02.rar` are copied unmodified from
`github.com/mholt/archives`' own test suite (`testdata/test.part*.rar` in that
module, MIT licensed). Together they form a real, valid multi-volume RAR
archive containing a single generated text file (`test.txt`), not any mod
data.

No open-source Go tooling, including the `nwaples/rardecode` library that
`mholt/archives` uses for RAR support, can write RAR archives - the format is
proprietary and only readable, not writable, without a licensed encoder. These
files are reused rather than authored here so the rar extraction and staging
path has real archive coverage instead of none.

Only `test.part01.rar` is used directly. Cratebug's `ExtractArchive` opens a
selected archive through a single `os.File` reader, never `mholt/archives`'
multi-volume-aware `Name`+`FS` sibling lookup (see `Rar.Extract` in that
module), so selecting just the first volume of this two-volume set is exactly
what happens if a user selects only the first part of a real multi-volume rar.
It proves that case fails cleanly (a clear "multi-volume archive continues in
next file" error, no partial staging state left unaccounted for) rather than
silently succeeding truncated or corrupting anything.

## empty.rar

A real, valid, single-volume rar archive with zero entries, created by the
project maintainer (no Go tooling here can author a rar itself - see above)
and renamed from `Empty RAR Archive.rar`. It proves an archive that decodes
without error but contains no `.pak`/`.utoc`/`.ucas` files is rejected the
same way any other empty selection is, distinct from the multi-volume failure
case above.

A single-volume rar containing real mod content was verified manually against
a fixture the project maintainer supplied, not committed here - see the
Phase 8 review.
