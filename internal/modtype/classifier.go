// Package modtype determines a coarse, UI-facing category for a mod without
// routing through UAssetToolRivals's heavy archive-extraction actions. See
// docs/decisions/0003-uassettoolrivals-boundary.md for why that heavy path
// is avoided, and TASKS.md's task 6.7 for this package's scope: category
// classification only. Hero and skin-name resolution is a separate,
// external-data-dependent concern this package does not cover.
package modtype

import "strings"

// A coarse classification of what a mod primarily changes.
type Category string

const (
	CategoryAudio      Category = "Audio"
	CategoryMovies     Category = "Movies"
	CategoryUI         Category = "UI"
	CategoryMesh       Category = "Mesh"
	CategoryStaticMesh Category = "Static Mesh"
	CategoryVFX        Category = "VFX"
	CategoryTexture    Category = "Texture"
	CategoryBlueprint  Category = "Blueprint"
	CategoryText       Category = "Text"
	CategoryUnknown    Category = "Unknown"
)

// Known content-root prefixes internal asset paths are stored under.
// Stripping them lets the same heuristics match both classic PAK listings
// (uassettool.ListPak) and IoStore listings (uassettool.ListIoStoreFiles).
var contentRootPrefixes = []string{"Marvel/Content/Marvel/", "/Game/Marvel/"}

// Aggregates, across every internal path in a mod, which kinds of content it
// contains. A mod can trip more than one flag; category() below picks a
// single primary category from the combination.
type characteristics struct {
	hasSkeletalMesh bool
	hasStaticMesh   bool
	hasTexture      bool
	hasMaterial     bool // VFX
	hasAudio        bool
	hasUI           bool
	hasMovies       bool
	hasText         bool
	hasBlueprint    bool
}

// Derives a coarse category from a mod's internal asset paths, as returned
// by uassettool.ListPak or uassettool.ListIoStoreFiles. Ports the filename
// and path heuristics BentoMod uses for its own instant type display
// (bentomod/src/utils.rs:112-296).
func Classify(paths []string) Category {
	var chars characteristics
	for _, path := range paths {
		chars.observe(path)
	}
	return chars.category()
}

func (c *characteristics) observe(path string) {
	rawLower := strings.ToLower(path)

	stripped := path
	for _, prefix := range contentRootPrefixes {
		if trimmed, ok := strings.CutPrefix(stripped, prefix); ok {
			stripped = trimmed
			break
		}
	}

	pathLower := strings.ToLower(stripped)
	filenameLower := pathLower
	if slash := strings.LastIndexByte(pathLower, '/'); slash >= 0 {
		filenameLower = pathLower[slash+1:]
	}

	// Extensionless paths are common in IoStore listings (package names
	// without a file extension); treat them as uasset-equivalent so the
	// mesh/texture/blueprint prefix checks below still apply to them.
	isUasset := strings.HasSuffix(filenameLower, ".uasset") || !strings.Contains(filenameLower, ".")

	if strings.HasPrefix(filenameLower, "sk_") && isUasset {
		c.hasSkeletalMesh = true
	}
	if strings.HasPrefix(filenameLower, "sm_") && isUasset {
		c.hasStaticMesh = true
	}
	if strings.HasPrefix(filenameLower, "t_") && isUasset {
		c.hasTexture = true
	}
	if strings.HasPrefix(filenameLower, "mi_") && (strings.Contains(pathLower, "/vfx/") || strings.HasPrefix(pathLower, "vfx/") || strings.Contains(rawLower, "/vfx/")) {
		c.hasMaterial = true
	}
	if strings.Contains(pathLower, "wwiseaudio") || strings.Contains(rawLower, "wwiseaudio") {
		c.hasAudio = true
	}
	if strings.Contains(pathLower, "/ui/") || strings.HasPrefix(pathLower, "ui/") || strings.Contains(rawLower, "/ui/") {
		c.hasUI = true
	}
	if strings.Contains(pathLower, "/movies/") || strings.HasPrefix(pathLower, "movies/") || strings.Contains(rawLower, "/movies/") ||
		strings.HasSuffix(pathLower, ".bik") || strings.HasSuffix(pathLower, ".mp4") {
		c.hasMovies = true
	}
	if strings.Contains(pathLower, "/stringtable/") || strings.HasPrefix(pathLower, "stringtable/") || strings.Contains(rawLower, "/stringtable/") ||
		strings.Contains(pathLower, "/data/stringtable/") {
		c.hasText = true
	}

	stem := filenameLower
	if dot := strings.LastIndexByte(stem, '.'); dot >= 0 {
		stem = stem[:dot]
	}
	isBlueprintName := strings.HasPrefix(filenameLower, "bp_") ||
		strings.HasSuffix(stem, "_c") ||
		strings.HasSuffix(stem, "bp")
	if (isBlueprintName || strings.Contains(pathLower, "/blueprints/")) && isUasset {
		c.hasBlueprint = true
	}
}

// Resolves the aggregated flags to a single primary category, in BentoMod's
// exact priority order (bentomod/src/utils.rs:272-296): a mod that is purely
// one kind of content (for example only audio files) gets that category; a
// mod that mixes audio with mesh, texture, or material content is
// categorized by that content instead, with audio falling back to a lower
// priority match further down.
func (c characteristics) category() Category {
	pureContent := !c.hasSkeletalMesh && !c.hasStaticMesh && !c.hasTexture && !c.hasMaterial

	switch {
	case c.hasAudio && pureContent:
		return CategoryAudio
	case c.hasMovies && pureContent:
		return CategoryMovies
	case c.hasUI && pureContent:
		return CategoryUI
	case c.hasSkeletalMesh:
		return CategoryMesh
	case c.hasStaticMesh:
		return CategoryStaticMesh
	case c.hasMaterial:
		return CategoryVFX
	case c.hasAudio:
		return CategoryAudio
	case c.hasTexture:
		return CategoryTexture
	case c.hasBlueprint:
		return CategoryBlueprint
	case c.hasText:
		return CategoryText
	default:
		return CategoryUnknown
	}
}
