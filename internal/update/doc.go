// Package update checks for, downloads, and applies newer Cratebug releases.
// It is pure Go, independent of Wails and React, following the same
// package-boundary pattern as internal/conflict and internal/install.
//
// Cratebug uses CalVer tags (YYYY.MM.DD, with an optional -rcN prerelease
// suffix) rather than semantic versioning, so version comparisons in this
// package work directly on that tag shape instead of a semver library.
package update
