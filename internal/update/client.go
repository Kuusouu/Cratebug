package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.github.com"

	// Timeout for a single GitHub API request. This covers release metadata
	// only; the much larger asset download uses its own client (see Download).
	requestTimeout = 15 * time.Second

	// Maximum bytes read from a non-OK response body when building an error
	// message, so a misbehaving server can't make an error message unbounded.
	maxErrorBodyBytes = 4096
)

// Returned when GitHub has no matching release (a 404 from the releases
// endpoint) rather than something being wrong with the request.
var ErrNoRelease = errors.New("update: no matching release found")

// A single GitHub release, resolved down to what Cratebug needs to decide
// whether to update and what to show the user about it.
type Release struct {
	Version Version
	HTMLURL string
	Notes   string // the release body: the changelog section the release workflow copied in
	Asset   ReleaseAsset
}

// The Windows installer attached to a release.
type ReleaseAsset struct {
	Name string
	URL  string
}

// Fetches Cratebug releases from GitHub.
type Client struct {
	Owner      string
	Repo       string
	HTTPClient *http.Client

	// Overrides the GitHub API root; empty means the real API. Tests point
	// this at an httptest.Server instead of the network.
	BaseURL string
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: requestTimeout}
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultAPIBaseURL
}

// Fetches the most recently published release.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.baseURL(), c.Owner, c.Repo)
	return c.fetch(ctx, url)
}

// Fetches a specific release by tag. Used to drive end-to-end update
// testing against a known prerelease rather than whatever is newest.
func (c *Client) Tag(ctx context.Context, tag string) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", c.baseURL(), c.Owner, c.Repo, tag)
	return c.fetch(ctx, url)
}

type releasePayload struct {
	TagName string                `json:"tag_name"`
	HTMLURL string                `json:"html_url"`
	Body    string                `json:"body"`
	Assets  []releaseAssetPayload `json:"assets"`
}

type releaseAssetPayload struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (c *Client) fetch(ctx context.Context, url string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("update: building request for %s: %w", url, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Cratebug")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("update: fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return Release{}, fmt.Errorf("update: %s returned %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}

	var payload releasePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("update: decoding release payload from %s: %w", url, err)
	}

	version, err := ParseVersion(payload.TagName)
	if err != nil {
		return Release{}, fmt.Errorf("update: release %q has an unparseable tag: %w", payload.TagName, err)
	}

	asset, err := findInstallerAsset(payload.Assets)
	if err != nil {
		return Release{}, fmt.Errorf("update: release %q: %w", payload.TagName, err)
	}

	return Release{
		Version: version,
		HTMLURL: payload.HTMLURL,
		Notes:   payload.Body,
		Asset:   asset,
	}, nil
}

// Picks the Windows installer out of a release's assets. Cratebug only ever
// publishes one Windows/amd64 installer per release (release.yml uploads
// exactly one), so unlike BentoMod's multi-platform asset matching, any
// .exe is unambiguous here.
func findInstallerAsset(assets []releaseAssetPayload) (ReleaseAsset, error) {
	for _, a := range assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			return ReleaseAsset{Name: a.Name, URL: a.BrowserDownloadURL}, nil
		}
	}
	return ReleaseAsset{}, fmt.Errorf("no .exe installer asset found among %d release assets", len(assets))
}
