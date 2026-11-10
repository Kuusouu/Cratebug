# 0012: Game patch updater

- Status: Accepted
- Date: 2026-11-01

## Context

Cooked Marvel Rivals assets use unversioned properties: each serialized
property is identified only by its position in the engine's schema. The
2026-10-29 patch (build 3984769) inserted two properties into
`MaterialInstanceBasePropertyOverrides` at positions 40 and 41. Every later
property moved up by two, so material instances cooked before the patch
misread, and the game crashed on many skin and VFX mods. A mappings file
(`.usmap`) describes the schema for one build, and the community publishes
one per build in `SpaceDepot/rivals-depot`.

BentoMod's VFX updater rebuilt a mod by copying its colors onto the game's
current asset. That loses every value it does not copy. On one library mod it
dropped all eight material instances and kept only the textures.

## Decision

Check first, then renumber only what the user selects. The check is a light
plan: it reads each asset in the layouts that might apply and proves the
remap is portable, but it does not rebuild containers or read updated assets
back. The live library is untouched. The apply pass runs the full pipeline
for the selected mods, so a mod the plan says can be carried may still fail
verification; its live files stay untouched when it does.

Renumber, do not rebuild. For each unversioned asset in a complete IoStore
mod, Cratebug:

1. Finds the earlier layout it was cooked with: a layout that reads it cleanly
   and writes its export bytes back exactly. Only layouts that read the
   asset's classes differently from the current one are tried.
2. Rewrites every stored unversioned header from that layout to the current
   one, matching properties by name, array index, and type. Values are not
   touched. A removed, retyped, or reordered property stops the asset.
3. Lets the worker write the asset with the current mappings, then reads it
   back and requires the same values with no raw-export fallback.

The rebuilt container is extracted again before it replaces anything. Every
`.uexp`, `.ubulk`, and `.uptnl` must match byte for byte, and each updated
asset must still read cleanly with its original values. The live originals
are copied to `Cratebug/patch-update-originals/<build>/` under the user's
config directory first, and an earlier copy is never overwritten.

Mappings come from `SpaceDepot/rivals-depot` through the GitHub contents API.
A download is kept only when its Git blob SHA-1 matches the listing and it
parses as mappings. Files are cached under `Cratebug/usmap/`. When the
listing cannot be reached, the cache is used.

## Reasons and rejected alternatives

- **Writing with the new mappings alone does nothing.** UAssetAPI reuses each
  stored header as is. A write with the new mappings produced the old bytes.
- **Dropping the stored headers is unsafe.** UAssetAPI's header generator
  produced an asset that no longer read back, even with the original mappings.
  The apply pass remaps the existing headers.
- **Color transfer onto vanilla assets** loses mod values (see Context).

The header encoder follows the engine's builder: fragments break at gaps or
after 127 values, skips over 127 use value-less fragments, and the zero mask
holds only values of fragments that have zeroes. An identity remap of a real
material instance wrote its original bytes back exactly.

## Check cost and scope

A check names the mods to check; an empty list means the whole library. That is
the same target list apply already used, so a single-mod check costs only that
mod.

Cratebug resolves the scope and verified mapping paths, then sends all
eligible bundles to one UAssetToolRivals worker with `check_iostore_patch`.
The worker reads IoStore package headers in parallel and skips extraction
for bundles whose headers show they have no unversioned assets. It shares
one game index and the parsed mappings when it extracts and checks the
remaining bundles. Cratebug joins the returned verdicts to the library
entries. Cancelling the check stops the worker and cleans up its scratch
directory.

## Limits

- Native data after the properties is carried over unchanged. A patch that
  changes native serialization is not detected by mappings and is not fixed.
- Mods whose companion `.pak` holds loose files are reported, not rebuilt.
- Only the 12 most recent earlier layouts are tried.
- Passing checks shows the asset reads correctly. It does not prove it renders
  correctly in game.
