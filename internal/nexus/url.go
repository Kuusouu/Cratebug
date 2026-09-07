package nexus

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	// Protocol-handler arguments and pasted site URLs should be short;
	// anything huge is a paste error or an injection payload.
	maxRawURLBytes = 2048

	redactedQuery     = "?<redacted>"
	unparseableURL    = "<unparseable URL>"
	nexusModsHost     = "nexusmods.com"
	nexusModsWWWHost  = "www.nexusmods.com"
	nxmScheme         = "nxm"
	httpsScheme       = "https"
	oauthHost         = "oauth"
	premiumHost       = "premium"
	collectionsMarker = "/collections"
)

// Matches /mods/<id>/files/<id> with optional trailing slash. Case-insensitive
// because some handlers rewrite path case.
var nxmModFilePath = regexp.MustCompile(`(?i)^/mods/(\d+)/files/(\d+)/?$`)

// Parses an nxm:// download link. Premium is false when key or expires is
// present and true when both are absent. Signing parameters are returned
// separately so they never sit on DownloadRequest.
func ParseDownloadURL(raw string) (DownloadRequest, DownloadSecrets, error) {
	if err := checkRawURL(raw); err != nil {
		return DownloadRequest{}, DownloadSecrets{}, err
	}

	u, err := url.Parse(raw)
	if err != nil {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: invalid download URL")
	}
	if u.Scheme != nxmScheme {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: unsupported URL scheme %q", u.Scheme)
	}
	if u.User != nil {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: download URL must not include userinfo")
	}

	if err := unsupportedNXM(u); err != nil {
		return DownloadRequest{}, DownloadSecrets{}, err
	}

	host := strings.ToLower(u.Hostname())
	if host != GameDomain {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("%w (%s)", ErrWrongGame, host)
	}

	parts := nxmModFilePath.FindStringSubmatch(u.Path)
	if parts == nil {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: download URL is missing mod or file id")
	}

	modID, err := strconv.Atoi(parts[1])
	if err != nil || modID <= 0 {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: invalid mod id")
	}
	fileID, err := strconv.Atoi(parts[2])
	if err != nil || fileID <= 0 {
		return DownloadRequest{}, DownloadSecrets{}, fmt.Errorf("nexus: invalid file id")
	}

	query := u.Query()
	secrets := DownloadSecrets{
		Key:     query.Get("key"),
		Expires: query.Get("expires"),
	}
	return DownloadRequest{
		Game:    GameDomain,
		ModID:   modID,
		FileID:  fileID,
		Premium: secrets.Key == "" && secrets.Expires == "",
	}, secrets, nil
}

// Parses an HTTPS Nexus Mods page URL for Marvel Rivals. Accepts both
// /{game}/mods/{id} and /games/{game}/mods/{id}, with an optional file id
// in the path or the file_id query parameter.
func ParseModPageURL(raw string) (ModPage, error) {
	if err := checkRawURL(raw); err != nil {
		return ModPage{}, err
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ModPage{}, fmt.Errorf("nexus: invalid mod page URL")
	}
	if u.Scheme != httpsScheme {
		return ModPage{}, fmt.Errorf("nexus: mod page URL must be HTTPS")
	}
	if u.User != nil {
		return ModPage{}, fmt.Errorf("nexus: mod page URL must not include userinfo")
	}

	host := strings.ToLower(u.Hostname())
	if host != nexusModsHost && host != nexusModsWWWHost {
		return ModPage{}, fmt.Errorf("nexus: not a Nexus Mods URL")
	}

	game, modID, fileID, err := parseModPagePath(u.Path)
	if err != nil {
		return ModPage{}, err
	}
	if game != GameDomain {
		return ModPage{}, fmt.Errorf("%w (%s)", ErrWrongGame, game)
	}

	if rawFileID := u.Query().Get("file_id"); rawFileID != "" && fileID == 0 {
		id, convErr := strconv.Atoi(rawFileID)
		if convErr != nil || id <= 0 {
			return ModPage{}, fmt.Errorf("nexus: invalid file_id")
		}
		fileID = id
	}

	return ModPage{Game: GameDomain, ModID: modID, FileID: fileID}, nil
}

// Returns the first os.Args-style element that parses as an nxm:// download
// link. Junk, Windows paths, and flags are skipped rather than reported.
func FirstDownloadURL(args []string) string {
	for _, raw := range args {
		if _, _, err := ParseDownloadURL(raw); err == nil {
			return raw
		}
	}
	return ""
}

// Rewrites a URL for error text. On parse failure the literal placeholder
// is returned so raw input — which may carry signing secrets — is never
// echoed. A query string is replaced by structure, not by parameter name.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return unparseableURL
	}
	out := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		out += redactedQuery
	}
	return out
}

func checkRawURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("nexus: empty URL")
	}
	if len(raw) > maxRawURLBytes {
		return fmt.Errorf("nexus: URL exceeds %d characters", maxRawURLBytes)
	}
	// A quote would break out of the protocol handler's "%1" argument.
	if strings.Contains(raw, `"`) {
		return fmt.Errorf("nexus: URL contains an unsafe character")
	}
	return nil
}

func unsupportedNXM(u *url.URL) error {
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.EscapedPath())
	if host == oauthHost {
		return fmt.Errorf("nexus: OAuth callback links are not supported")
	}
	if host == premiumHost {
		return fmt.Errorf("nexus: premium links are not supported")
	}
	if strings.Contains(path, collectionsMarker) {
		return fmt.Errorf("nexus: collection links are not supported")
	}
	return nil
}

func parseModPagePath(path string) (game string, modID, fileID int, err error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 1 && strings.EqualFold(parts[0], "games") {
		parts = parts[1:]
	}
	if len(parts) < 3 || !strings.EqualFold(parts[1], "mods") {
		return "", 0, 0, fmt.Errorf("nexus: not a Nexus Mods mod page URL")
	}

	game = strings.ToLower(parts[0])
	modID, err = strconv.Atoi(parts[2])
	if err != nil || modID <= 0 {
		return "", 0, 0, fmt.Errorf("nexus: invalid mod id")
	}

	switch {
	case len(parts) == 3:
		return game, modID, 0, nil
	case len(parts) == 5 && strings.EqualFold(parts[3], "files"):
		fileID, err = strconv.Atoi(parts[4])
		if err != nil || fileID <= 0 {
			return "", 0, 0, fmt.Errorf("nexus: invalid file id")
		}
		return game, modID, fileID, nil
	default:
		return "", 0, 0, fmt.Errorf("nexus: not a Nexus Mods mod page URL")
	}
}
