// Package nexus talks to the Nexus Mods API and parses nxm:// and site
// download URLs. It does not stream file downloads.
package nexus

import (
	"errors"
	"time"
)

// Marvel Rivals' Nexus Mods game domain.
const GameDomain = "marvelrivals"

const (
	defaultAPIBaseURL = "https://api.nexusmods.com"
	applicationName   = "Cratebug"

	// Timeout for a single Nexus API request. File downloads use their own
	// client and are out of scope for this package.
	requestTimeout = 15 * time.Second

	// Maximum bytes read from a non-OK response body when building an error
	// message, so a misbehaving server can't make an error message unbounded.
	maxErrorBodyBytes = 4096

	// Upper bound on a pasted API key. Real Nexus keys are much shorter;
	// anything larger is either a paste error or a header-injection attempt.
	maxAPIKeyBytes = 512

	// Vortex caches Nexus mod/file metadata for five minutes; match that
	// so a file-picker session does not re-hit the API on every click.
	metadataCacheTTL = 5 * time.Minute
)

var (
	// No key is configured, or the key is empty at call time.
	ErrNoAPIKey = errors.New("nexus: no API key configured")

	// The API rejected the key (HTTP 401).
	ErrInvalidKey = errors.New("nexus: API key is invalid")

	// A keyless download_link request was refused (HTTP 403). Free accounts
	// must start the download from the Nexus website.
	ErrPremiumRequired = errors.New("nexus: premium is required for this download")

	// A keyed download_link request was refused (HTTP 403 or 410). The
	// signed nxm:// parameters are no longer valid.
	ErrLinkExpired = errors.New("nexus: download link has expired")

	// The requested mod or file does not exist (HTTP 404).
	ErrNotFound = errors.New("nexus: not found")

	// The API returned 429, or remaining requests are exhausted and the
	// matching reset is still in the future.
	ErrRateLimited = errors.New("nexus: rate limited")

	// An nxm:// host or a site-URL path named a game other than Marvel Rivals.
	ErrWrongGame = errors.New("nexus: URL is not for Marvel Rivals")
)

// Account profile returned by the validate endpoint. The API also sends a
// key field; it is intentionally omitted so it cannot be marshalled to the
// frontend.
type User struct {
	UserID      int    `json:"user_id"`
	Name        string `json:"name"`
	IsPremium   bool   `json:"is_premium"`
	IsSupporter bool   `json:"is_supporter"`
	Email       string `json:"email"`
	ProfileURL  string `json:"profile_url"`
}

// Public metadata for one Nexus mod.
type ModInfo struct {
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Version    string `json:"version"`
	PictureURL string `json:"picture_url"`
	Author     string `json:"author"`
	ModID      int    `json:"mod_id"`
}

// One downloadable file attached to a Nexus mod.
type FileInfo struct {
	FileID     int    `json:"file_id"`
	Name       string `json:"name"`
	FileName   string `json:"file_name"`
	Version    string `json:"version"`
	ModVersion string `json:"mod_version"`
	// Size and SizeKB are both kilobytes on the REST file endpoints.
	// SizeInBytes is the only byte-accurate field when Nexus sends it.
	Size              int64  `json:"size"`
	SizeKB            int    `json:"size_kb"`
	SizeInBytes       int64  `json:"size_in_bytes"`
	CategoryName      string `json:"category_name"`
	IsPrimary         bool   `json:"is_primary"`
	UploadedTimestamp int64  `json:"uploaded_timestamp"`
}

// One CDN location from download_link.json.
type DownloadLink struct {
	URI       string `json:"URI"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

// Snapshot of the X-RL-* headers from the most recent API response.
type RateLimit struct {
	HourlyLimit     int
	HourlyRemaining int
	HourlyReset     time.Time
	DailyLimit      int
	DailyRemaining  int
	DailyReset      time.Time
}

// Parsed nxm:// link handed to the frontend later. It carries no key or expires.
type DownloadRequest struct {
	Game    string `json:"game"`
	ModID   int    `json:"modId"`
	FileID  int    `json:"fileId"`
	Premium bool   `json:"premium"` // false when the link carried key/expires
}

// Signing parameters from a free-user nxm:// link. Kept off DownloadRequest
// so they cannot be JSON-marshalled to the frontend.
type DownloadSecrets struct {
	Key     string
	Expires string
}

// A Nexus site URL for a Marvel Rivals mod, with an optional file id.
type ModPage struct {
	Game   string
	ModID  int
	FileID int // 0 if absent
}
