package update

import (
	"fmt"
	"regexp"
	"strconv"
)

var tagPattern = regexp.MustCompile(`^(\d{4})\.(\d{2})\.(\d{2})(-.+)?$`)

// Parsed Cratebug CalVer release tag, e.g. "2026.08.27" or
// "2026.08.27-rc1".
type Version struct {
	Tag        string `json:"tag"`
	Year       int    `json:"year"`
	Month      int    `json:"month"`
	Day        int    `json:"day"`
	Prerelease string `json:"prerelease,omitempty"` // suffix after the hyphen, without the hyphen; empty for a stable release
}

// Parses a CalVer release tag. It returns an error for anything
// that doesn't match the release workflow's YYYY.MM.DD[-suffix] form,
// including the placeholder "dev" version that local and CI builds carry.
func ParseVersion(tag string) (Version, error) {
	matches := tagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return Version{}, fmt.Errorf("update: %q is not a valid CalVer release tag (expected YYYY.MM.DD or YYYY.MM.DD-suffix)", tag)
	}

	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return Version{}, fmt.Errorf("update: %q has an unparseable year: %w", tag, err)
	}
	month, err := strconv.Atoi(matches[2])
	if err != nil {
		return Version{}, fmt.Errorf("update: %q has an unparseable month: %w", tag, err)
	}
	day, err := strconv.Atoi(matches[3])
	if err != nil {
		return Version{}, fmt.Errorf("update: %q has an unparseable day: %w", tag, err)
	}

	prerelease := ""
	if matches[4] != "" {
		prerelease = matches[4][1:] // drop the leading "-"
	}

	return Version{
		Tag:        tag,
		Year:       year,
		Month:      month,
		Day:        day,
		Prerelease: prerelease,
	}, nil
}

// Reports whether v is a newer release than other.
//
// CalVer's zero-padded YYYY.MM.DD sorts correctly as a plain date compare,
// so this deliberately avoids a semver library: a hyphenated CalVer tag like
// "2026.08.27-rc1" isn't valid strict semver anyway (semver's grammar
// forbids leading zeros in numeric identifiers). The only wrinkle CalVer
// itself introduces is the prerelease suffix: a stable release outranks any
// -rcN of the same date, and two prereleases of the same date fall back to
// a plain string compare of the suffix (covers "rc2" > "rc1"; anything else
// is at least deterministic rather than wrong).
func (v Version) IsNewer(other Version) bool {
	if v.Year != other.Year {
		return v.Year > other.Year
	}
	if v.Month != other.Month {
		return v.Month > other.Month
	}
	if v.Day != other.Day {
		return v.Day > other.Day
	}
	if v.Prerelease == other.Prerelease {
		return false
	}
	if v.Prerelease == "" {
		return true
	}
	if other.Prerelease == "" {
		return false
	}
	return v.Prerelease > other.Prerelease
}
