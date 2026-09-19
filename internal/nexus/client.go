package nexus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cacheKeyValidate    = "validate"
	cacheKeyPreferences = "preferences"
	preferencesQuery    = "query { preferences { adult isBlockingContent } }"
	adultContentBlocked = "ADULT_CONTENT_BLOCKED"
	// Looks up one mod by domain and id. Do not pass viewAdultContent: that
	// overrides the signed-in account's Content Blocking setting.
	modAdultQuery = `query AdultCheck($ids: [CompositeDomainWithIdInput!]!) { legacyModsByDomain(ids: $ids) { nodes { modId adult adultContent } } }`
)

type downloadMode int

const (
	downloadNone downloadMode = iota
	downloadKeyless
	downloadKeyed
)

type cacheEntry struct {
	expires   time.Time // zero means session-scoped (no time expiry)
	value     any
	cachedKey string // Validate only: the API key used when the entry was stored
}

type filesResponse struct {
	Files []FileInfo `json:"files"`
}

type graphqlRequest struct {
	Query     string `json:"query"`
	Variables any    `json:"variables,omitempty"`
}

type graphqlPreferencesResponse struct {
	Data struct {
		Preferences *Preferences `json:"preferences"`
	} `json:"data"`
	Errors []graphqlError `json:"errors"`
}

type graphqlError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code string `json:"code"`
	} `json:"extensions"`
}

type graphqlModAdultNode struct {
	ModID        int  `json:"modId"`
	Adult        bool `json:"adult"`
	AdultContent bool `json:"adultContent"`
}

type graphqlModAdultResponse struct {
	Data struct {
		LegacyModsByDomain *struct {
			Nodes []graphqlModAdultNode `json:"nodes"`
		} `json:"legacyModsByDomain"`
	} `json:"data"`
	Errors []graphqlError `json:"errors"`
}

// Talks to api.nexusmods.com. APIKey is read at call time and must never
// be copied onto a type that is JSON-marshalled to the frontend.
type Client struct {
	APIKey     string
	HTTPClient *http.Client

	// Overrides the Nexus API root; empty means the real API. Tests point
	// this at an httptest.Server instead of the network.
	BaseURL string

	// Sent as Application-Version and inside the User-Agent. Passed in so
	// this package does not import main.
	AppVersion string

	mu        sync.Mutex
	rateLimit RateLimit
	cache     map[string]cacheEntry
	now       func() time.Time
}

// Builds a client that sends apiKey and appVersion on each request.
func NewClient(apiKey, appVersion string) *Client {
	return &Client{APIKey: apiKey, AppVersion: appVersion}
}

// Checks the API key and returns the account it belongs to. The result is
// cached for the life of the Client until the key changes or the API
// returns 401.
func (c *Client) Validate(ctx context.Context) (User, error) {
	if v, ok := c.cacheGet(cacheKeyValidate); ok {
		return v.(User), nil
	}

	var user User
	if err := c.get(ctx, "/v1/users/validate.json", nil, downloadNone, &user); err != nil {
		return User{}, err
	}
	c.cachePut(cacheKeyValidate, user, 0)
	return user, nil
}

// Fetches public metadata for one Marvel Rivals mod. Cached for five minutes.
func (c *Client) Mod(ctx context.Context, modID int) (ModInfo, error) {
	if err := requirePositiveID("mod id", modID); err != nil {
		return ModInfo{}, err
	}
	key := fmt.Sprintf("mod:%d", modID)
	if v, ok := c.cacheGet(key); ok {
		return v.(ModInfo), nil
	}

	var info ModInfo
	if err := c.get(ctx, fmt.Sprintf("/v1/games/%s/mods/%d.json", GameDomain, modID), nil, downloadNone, &info); err != nil {
		return ModInfo{}, err
	}
	c.cachePut(key, info, metadataCacheTTL)
	return info, nil
}

// Lists the files attached to a mod. Cached for five minutes.
func (c *Client) Files(ctx context.Context, modID int) ([]FileInfo, error) {
	if err := requirePositiveID("mod id", modID); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("files:%d", modID)
	if v, ok := c.cacheGet(key); ok {
		return cloneFileInfos(v.([]FileInfo)), nil
	}

	var payload filesResponse
	path := fmt.Sprintf("/v1/games/%s/mods/%d/files.json", GameDomain, modID)
	if err := c.get(ctx, path, nil, downloadNone, &payload); err != nil {
		return nil, err
	}
	c.cachePut(key, cloneFileInfos(payload.Files), metadataCacheTTL)
	return cloneFileInfos(payload.Files), nil
}

// Fetches one file's metadata. Cached for five minutes.
func (c *Client) File(ctx context.Context, modID, fileID int) (FileInfo, error) {
	if err := requirePositiveID("mod id", modID); err != nil {
		return FileInfo{}, err
	}
	if err := requirePositiveID("file id", fileID); err != nil {
		return FileInfo{}, err
	}
	key := fmt.Sprintf("file:%d:%d", modID, fileID)
	if v, ok := c.cacheGet(key); ok {
		return v.(FileInfo), nil
	}

	var info FileInfo
	path := fmt.Sprintf("/v1/games/%s/mods/%d/files/%d.json", GameDomain, modID, fileID)
	if err := c.get(ctx, path, nil, downloadNone, &info); err != nil {
		return FileInfo{}, err
	}
	c.cachePut(key, info, metadataCacheTTL)
	return info, nil
}

// Resolves CDN locations for a file. Empty secrets use the premium/keyless
// path; Key and Expires are sent as query parameters. The result is never
// cached: download links are short-lived.
func (c *Client) DownloadLinks(ctx context.Context, modID, fileID int, secrets DownloadSecrets) ([]DownloadLink, error) {
	if err := requirePositiveID("mod id", modID); err != nil {
		return nil, err
	}
	if err := requirePositiveID("file id", fileID); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/games/%s/mods/%d/files/%d/download_link.json", GameDomain, modID, fileID)
	var query url.Values
	mode := downloadKeyless
	if secrets.Key != "" || secrets.Expires != "" {
		mode = downloadKeyed
		query = url.Values{}
		if secrets.Key != "" {
			query.Set("key", secrets.Key)
		}
		if secrets.Expires != "" {
			query.Set("expires", secrets.Expires)
		}
	}

	var links []DownloadLink
	if err := c.get(ctx, path, query, mode, &links); err != nil {
		return nil, err
	}
	return links, nil
}

// Loads the signed-in user's adult-content preference. Cached for five
// minutes. Does not send viewAdultContent; that would override the account.
func (c *Client) Preferences(ctx context.Context) (Preferences, error) {
	if v, ok := c.cacheGet(cacheKeyPreferences); ok {
		return v.(Preferences), nil
	}

	var payload graphqlPreferencesResponse
	if err := c.postJSON(ctx, "/v2/graphql", graphqlRequest{Query: preferencesQuery}, &payload); err != nil {
		return Preferences{}, err
	}
	if payload.Data.Preferences == nil {
		if len(payload.Errors) > 0 {
			return Preferences{}, fmt.Errorf("nexus: preferences: %s", redactErrorBody(c.APIKey, payload.Errors[0].Message))
		}
		return Preferences{}, fmt.Errorf("nexus: preferences: missing data")
	}
	prefs := *payload.Data.Preferences
	c.cachePut(cacheKeyPreferences, prefs, metadataCacheTTL)
	return prefs, nil
}

// Asks GraphQL whether this Marvel Rivals mod is visible and adult, without
// overriding Content Blocking. REST and signed nxm:// downloads still
// return adult files when the website hides the page.
func (c *Client) graphQLModAdult(ctx context.Context, modID int) (found, adult bool, err error) {
	if err := requirePositiveID("mod id", modID); err != nil {
		return false, false, err
	}
	key := fmt.Sprintf("modAdult:%d", modID)
	if v, ok := c.cacheGet(key); ok {
		node := v.(graphqlModAdultNode)
		return true, node.Adult || node.AdultContent, nil
	}

	var payload graphqlModAdultResponse
	req := graphqlRequest{
		Query: modAdultQuery,
		Variables: map[string]any{
			"ids": []map[string]any{
				{"gameDomain": GameDomain, "modId": modID},
			},
		},
	}
	if err := c.postJSON(ctx, "/v2/graphql", req, &payload); err != nil {
		return false, false, err
	}
	if graphqlAdultBlocked(payload.Errors) {
		return false, false, ErrAdultContentBlocked
	}
	if payload.Data.LegacyModsByDomain != nil {
		nodes := payload.Data.LegacyModsByDomain.Nodes
		if len(nodes) == 0 {
			return false, false, nil
		}
		node := nodes[0]
		c.cachePut(key, node, metadataCacheTTL)
		return true, node.Adult || node.AdultContent, nil
	}
	if len(payload.Errors) > 0 {
		return false, false, fmt.Errorf("nexus: mod adult: %s", redactErrorBody(c.APIKey, payload.Errors[0].Message))
	}
	return false, false, fmt.Errorf("nexus: mod adult: missing data")
}

func graphqlAdultBlocked(errs []graphqlError) bool {
	for _, err := range errs {
		if strings.EqualFold(err.Extensions.Code, adultContentBlocked) {
			return true
		}
		if strings.Contains(strings.ToLower(err.Message), "adult content blocked") {
			return true
		}
	}
	return false
}

// A copy of the most recently observed rate-limit headers.
func (c *Client) RateLimit() RateLimit {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rateLimit
}

func (c *Client) get(ctx context.Context, path string, query url.Values, mode downloadMode, dest any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, mode, dest)
}

func (c *Client) postJSON(ctx context.Context, path string, payload, dest any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("nexus: encoding %s: %w", path, err)
	}
	return c.do(ctx, http.MethodPost, path, nil, encoded, downloadNone, dest)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, mode downloadMode, dest any) error {
	key := c.APIKey
	if err := validateAPIKey(key); err != nil {
		return err
	}
	if c.blockedByRateLimit() {
		return ErrRateLimited
	}

	rawURL := c.baseURL() + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return fmt.Errorf("nexus: building request for %s: %w", redactURL(rawURL), err)
	}
	// Query is attached after NewRequest so a construction error cannot
	// embed signed key=/expires= parameters.
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}

	// Exactly one site sets apikey. Header.Set does not validate values,
	// so validateAPIKey must have already rejected CR/LF and controls.
	req.Header.Set("apikey", key)
	req.Header.Set("Application-Name", applicationName)
	req.Header.Set("Application-Version", c.AppVersion)
	req.Header.Set("User-Agent", fmt.Sprintf("%s/%s (Windows)", applicationName, c.AppVersion))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return wrapFetchError(c.APIKey, req.URL.String(), err)
	}
	defer resp.Body.Close()

	c.captureRateLimit(resp.Header)

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("nexus: decoding %s: %w", redactURL(req.URL.String()), err)
		}
		return nil
	case http.StatusUnauthorized:
		c.invalidateAuthCaches()
		return ErrInvalidKey
	case http.StatusForbidden:
		switch mode {
		case downloadKeyed:
			return ErrLinkExpired
		case downloadKeyless:
			return ErrPremiumRequired
		}
		return c.statusError(req, resp)
	case http.StatusGone:
		if mode == downloadKeyed {
			return ErrLinkExpired
		}
		return c.statusError(req, resp)
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		c.noteRateLimited()
		return ErrRateLimited
	default:
		return c.statusError(req, resp)
	}
}

func (c *Client) statusError(req *http.Request, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	return fmt.Errorf("nexus: %s returned %s: %s", redactURL(req.URL.String()), resp.Status, redactErrorBody(c.APIKey, strings.TrimSpace(string(body))))
}

func (c *Client) httpClient() *http.Client {
	base := c.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: requestTimeout}
	}
	// Copy so a caller-supplied client is not mutated. Go follows redirects
	// and re-sends custom headers (it only strips Authorization and Cookie),
	// so apikey would otherwise be forwarded to a different host.
	clone := *base
	clone.CheckRedirect = refuseRedirect
	return &clone
}

var errRedirect = errors.New("nexus: refusing HTTP redirect")

func refuseRedirect(*http.Request, []*http.Request) error {
	return errRedirect
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return defaultAPIBaseURL
}

func (c *Client) nowTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *Client) blockedByRateLimit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.nowTime()
	if c.rateLimit.HourlyRemaining <= 0 && c.rateLimit.HourlyReset.After(now) {
		return true
	}
	if c.rateLimit.DailyRemaining <= 0 && c.rateLimit.DailyReset.After(now) {
		return true
	}
	return false
}

func (c *Client) captureRateLimit(h http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := headerInt(h, "X-RL-Hourly-Limit"); ok {
		c.rateLimit.HourlyLimit = v
	}
	if v, ok := headerInt(h, "X-RL-Hourly-Remaining"); ok {
		c.rateLimit.HourlyRemaining = v
	}
	if v, ok := headerTime(h, "X-RL-Hourly-Reset"); ok {
		c.rateLimit.HourlyReset = v
	}
	if v, ok := headerInt(h, "X-RL-Daily-Limit"); ok {
		c.rateLimit.DailyLimit = v
	}
	if v, ok := headerInt(h, "X-RL-Daily-Remaining"); ok {
		c.rateLimit.DailyRemaining = v
	}
	if v, ok := headerTime(h, "X-RL-Daily-Reset"); ok {
		c.rateLimit.DailyReset = v
	}
}

func (c *Client) noteRateLimited() {
	c.mu.Lock()
	defer c.mu.Unlock()
	hourlyZero := c.rateLimit.HourlyRemaining <= 0
	dailyZero := c.rateLimit.DailyRemaining <= 0
	if hourlyZero == dailyZero {
		// Both exhausted, or neither (cannot tell which bucket 429 named).
		c.rateLimit.HourlyRemaining = 0
		c.rateLimit.DailyRemaining = 0
		return
	}
	if hourlyZero {
		c.rateLimit.HourlyRemaining = 0
		return
	}
	c.rateLimit.DailyRemaining = 0
}

func (c *Client) cacheGet(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		return nil, false
	}
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if (key == cacheKeyValidate || key == cacheKeyPreferences) && entry.cachedKey != c.APIKey {
		delete(c.cache, key)
		return nil, false
	}
	if !entry.expires.IsZero() && !c.nowTime().Before(entry.expires) {
		delete(c.cache, key)
		return nil, false
	}
	return entry.value, true
}

func (c *Client) cachePut(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil {
		c.cache = make(map[string]cacheEntry)
	}
	entry := cacheEntry{value: value}
	if ttl > 0 {
		entry.expires = c.nowTime().Add(ttl)
	}
	if key == cacheKeyValidate || key == cacheKeyPreferences {
		entry.cachedKey = c.APIKey
	}
	c.cache[key] = entry
}

func (c *Client) invalidateAuthCaches() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache != nil {
		delete(c.cache, cacheKeyValidate)
		delete(c.cache, cacheKeyPreferences)
	}
}

// Reports whether key is acceptable as an HTTP header value. Empty keys,
// oversized keys, and anything outside printable ASCII are rejected so a
// crafted paste cannot inject headers.
func CheckAPIKey(key string) error {
	return validateAPIKey(key)
}

func validateAPIKey(key string) error {
	if key == "" {
		return ErrNoAPIKey
	}
	if len(key) > maxAPIKeyBytes {
		return fmt.Errorf("nexus: API key exceeds %d characters", maxAPIKeyBytes)
	}
	for i := 0; i < len(key); i++ {
		// Printable ASCII excluding space ('!'..'~'). Whitespace and
		// controls would let a crafted key inject headers because
		// Header.Set does not validate values.
		if key[i] < '!' || key[i] > '~' {
			return fmt.Errorf("nexus: API key contains invalid characters")
		}
	}
	return nil
}

func requirePositiveID(name string, id int) error {
	if id <= 0 {
		return fmt.Errorf("nexus: invalid %s", name)
	}
	return nil
}

func headerInt(h http.Header, name string) (int, bool) {
	s := strings.TrimSpace(h.Get(name))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func headerTime(h http.Header, name string) (time.Time, bool) {
	return parseResetTime(strings.TrimSpace(h.Get(name)))
}

func parseResetTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0), true
	}
	if t, err := time.Parse(time.RFC1123, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func wrapFetchError(apiKey, rawURL string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = redactURL(urlErr.URL)
	}
	msg := err.Error()
	if strings.Contains(msg, "key=") || (apiKey != "" && strings.Contains(msg, apiKey)) {
		return fmt.Errorf("nexus: fetching %s: %s", redactURL(rawURL), "<redacted>")
	}
	return fmt.Errorf("nexus: fetching %s: %w", redactURL(rawURL), err)
}

func redactErrorBody(apiKey, body string) string {
	if body == "" {
		return body
	}
	if strings.Contains(body, "key=") || (apiKey != "" && strings.Contains(body, apiKey)) {
		return "<redacted>"
	}
	return body
}

func cloneFileInfos(files []FileInfo) []FileInfo {
	if files == nil {
		return nil
	}
	out := make([]FileInfo, len(files))
	copy(out, files)
	return out
}
