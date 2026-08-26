// Package conflict compares internal Unreal asset paths across enabled mods
// to find overlapping content. UAssetToolRivals only supplies the internal
// paths (via internal/uassettool, already resolved and cached per mod by
// internal/modtype); this package owns every decision about what counts as
// a conflict, matching the boundary SPEC.md section 15 describes.
package conflict
