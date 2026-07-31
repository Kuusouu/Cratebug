# Discovery fixtures

These files are tiny text placeholders. They are not valid Unreal Engine
containers and contain no game or mod data. Tests that need to modify a fixture
must first copy it into a temporary directory.

## Evidence

The following behavior is confirmed by BentoMod's scanner:

- `.pak`, `.bak_bento`, and `.pak_disabled` are recognized primary forms.
- Discovery is recursive and sidecars are associated by directory and stem.
- A leading `!` and trailing runs of at least seven nines are priority forms.
- Seven, eight, and nine trailing nines are retained as separate evidence cases.
- A leading `!` and an unrecognized or absent priority suffix can all produce
  numeric priority zero. Task 1.5 must preserve enough parse information to
  distinguish those cases.

The following are Cratebug specification decisions rather than confirmed
BentoMod behavior:

- `.pak_crateoff` is a recognized disabled primary. BentoMod does not recognize
  this extension.
- Incomplete bundles and orphan sidecars remain visible for diagnostics.
- Equal stems in different directories are distinct candidates.
- Enabled and disabled primaries with the same directory and stem are retained
  as an ambiguous candidate set.
- Folder and stem matching is case-insensitive for Windows-first discovery, but
  reported paths retain their original casing.

## Inventory

- `enabled`: an enabled `.pak`.
- `disabled`: each supported disabled primary form.
- `classic`: a primary without IoStore sidecars.
- `iostore`: a complete `.pak`, `.utoc`, and `.ucas` bundle.
- `nested`: recursive directory discovery.
- `priority`: leading `!`, seven/eight/nine nines, an absent suffix, and an
  unrecognized priority name.
- `partial`: primary plus only `.utoc`, and primary plus only `.ucas`.
- `orphan`: standalone `.utoc` and `.ucas` files with different stems.
- `duplicates`: the same primary stem in different directories.
- `ambiguous`: enabled and disabled primaries sharing a directory and stem.
