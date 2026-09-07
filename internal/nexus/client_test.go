package nexus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testAPIKey    = "test-api-key"
	sentinelKey   = "SENTINEL"
	sentinelQuery = "SENTINELQUERY"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewClient(testAPIKey, "test")
	client.BaseURL = server.URL
	return client
}

func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, body)
}

func writeRateLimit(w http.ResponseWriter, hourlyLimit, hourlyRemaining, dailyLimit, dailyRemaining int, hourlyReset, dailyReset time.Time) {
	w.Header().Set("X-RL-Hourly-Limit", strconv.Itoa(hourlyLimit))
	w.Header().Set("X-RL-Hourly-Remaining", strconv.Itoa(hourlyRemaining))
	w.Header().Set("X-RL-Hourly-Reset", strconv.FormatInt(hourlyReset.Unix(), 10))
	w.Header().Set("X-RL-Daily-Limit", strconv.Itoa(dailyLimit))
	w.Header().Set("X-RL-Daily-Remaining", strconv.Itoa(dailyRemaining))
	w.Header().Set("X-RL-Daily-Reset", strconv.FormatInt(dailyReset.Unix(), 10))
}

func TestClientValidateParsesUserAndIgnoresKeyField(t *testing.T) {
	// Arrange
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/validate.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != testAPIKey {
			t.Errorf("apikey header = %q, want the test key", r.Header.Get("apikey"))
		}
		if r.Header.Get("Application-Name") != "Cratebug" {
			t.Errorf("Application-Name = %q, want Cratebug", r.Header.Get("Application-Name"))
		}
		if r.Header.Get("Application-Version") != "test" {
			t.Errorf("Application-Version = %q, want test", r.Header.Get("Application-Version"))
		}
		if r.Header.Get("User-Agent") != "Cratebug/test (Windows)" {
			t.Errorf("User-Agent = %q, want Cratebug/test (Windows)", r.Header.Get("User-Agent"))
		}
		writeJSON(w, `{
			"user_id": 42,
			"key": "SENTINEL-MUST-NOT-APPEAR",
			"name": "Tester",
			"is_premium": true,
			"is_supporter": false,
			"email": "a@example.invalid",
			"profile_url": "https://nexusmods.com/users/42"
		}`)
	})

	// Act
	user, err := client.Validate(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Validate() = %v, want no error", err)
	}
	if user.UserID != 42 || user.Name != "Tester" || !user.IsPremium || user.Email != "a@example.invalid" {
		t.Errorf("User = %+v, want id 42 / Tester / premium", user)
	}

	rt := reflect.TypeOf(User{})
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if strings.EqualFold(field.Name, "Key") || field.Tag.Get("json") == "key" {
			t.Fatalf("User must not include the validate.json key field")
		}
	}
	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal(User) = %v", err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Unmarshal marshalled User: %v", err)
	}
	if _, ok := asMap["key"]; ok {
		t.Fatal("marshalled User includes a key field")
	}
	if strings.Contains(string(raw), "SENTINEL-MUST-NOT-APPEAR") {
		t.Fatal("marshalled User leaked the validate.json key value")
	}
}

func TestClientFilesUnwrapsArray(t *testing.T) {
	// Arrange
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/games/marvelrivals/mods/10/files.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(w, `{
			"files": [
				{
					"file_id": 7,
					"name": "Main",
					"file_name": "mod-1.0.zip",
					"version": "1.0",
					"mod_version": "1.0",
					"size": 2048,
					"size_kb": 2,
					"category_name": "MAIN",
					"is_primary": true,
					"uploaded_timestamp": 1700000000
				}
			]
		}`)
	})

	// Act
	files, err := client.Files(context.Background(), 10)

	// Assert
	if err != nil {
		t.Fatalf("Files() = %v, want no error", err)
	}
	if len(files) != 1 {
		t.Fatalf("len(Files) = %d, want 1", len(files))
	}
	got := files[0]
	if got.FileID != 7 || got.FileName != "mod-1.0.zip" || got.Size != 2048 || !got.IsPrimary {
		t.Errorf("FileInfo = %+v, want file 7 / mod-1.0.zip / size 2048 / primary", got)
	}
}

func TestClientMapsStatusToSentinel(t *testing.T) {
	tests := []struct {
		name   string
		status int
		call   func(*Client) error
		want   error
	}{
		{
			name:   "401 is invalid key",
			status: http.StatusUnauthorized,
			call:   func(c *Client) error { _, err := c.Validate(context.Background()); return err },
			want:   ErrInvalidKey,
		},
		{
			name:   "404 is not found",
			status: http.StatusNotFound,
			call:   func(c *Client) error { _, err := c.Mod(context.Background(), 1); return err },
			want:   ErrNotFound,
		},
		{
			name:   "429 is rate limited",
			status: http.StatusTooManyRequests,
			call:   func(c *Client) error { _, err := c.File(context.Background(), 1, 2); return err },
			want:   ErrRateLimited,
		},
		{
			name:   "403 keyless download requires premium",
			status: http.StatusForbidden,
			call: func(c *Client) error {
				_, err := c.DownloadLinks(context.Background(), 1, 2, DownloadSecrets{})
				return err
			},
			want: ErrPremiumRequired,
		},
		{
			name:   "403 keyed download is expired",
			status: http.StatusForbidden,
			call: func(c *Client) error {
				_, err := c.DownloadLinks(context.Background(), 1, 2, DownloadSecrets{Key: "k", Expires: "1"})
				return err
			},
			want: ErrLinkExpired,
		},
		{
			name:   "410 keyed download is expired",
			status: http.StatusGone,
			call: func(c *Client) error {
				_, err := c.DownloadLinks(context.Background(), 1, 2, DownloadSecrets{Key: "k", Expires: "1"})
				return err
			},
			want: ErrLinkExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})

			// Act
			err := tt.call(client)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestClientEmptyKeyIsErrNoAPIKey(t *testing.T) {
	// Arrange
	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	})
	client.APIKey = ""

	// Act
	_, err := client.Validate(context.Background())

	// Assert
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("Validate() = %v, want ErrNoAPIKey", err)
	}
	if hits != 0 {
		t.Fatalf("handler called %d times, want 0", hits)
	}
}

func TestClientParsesRateLimitHeaders(t *testing.T) {
	// Arrange
	hourlyReset := time.Unix(1_800_000_000, 0)
	dailyReset := time.Unix(1_800_086_400, 0)
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRateLimit(w, 100, 99, 1000, 998, hourlyReset, dailyReset)
		writeJSON(w, `{"name":"M","mod_id":1}`)
	})

	// Act
	_, err := client.Mod(context.Background(), 1)

	// Assert
	if err != nil {
		t.Fatalf("Mod() = %v, want no error", err)
	}
	got := client.RateLimit()
	if got.HourlyLimit != 100 || got.HourlyRemaining != 99 || got.DailyLimit != 1000 || got.DailyRemaining != 998 {
		t.Fatalf("RateLimit() = %+v, want hourly 100/99 daily 1000/998", got)
	}
	if !got.HourlyReset.Equal(hourlyReset) {
		t.Errorf("HourlyReset = %v, want %v", got.HourlyReset, hourlyReset)
	}
	if !got.DailyReset.Equal(dailyReset) {
		t.Errorf("DailyReset = %v, want %v", got.DailyReset, dailyReset)
	}
}

func TestParseResetTime(t *testing.T) {
	unix := time.Unix(1_700_000_000, 0)
	rfc1123 := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	rfc3339 := time.Date(2026, 9, 6, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   string
		want time.Time
		ok   bool
	}{
		{name: "unix seconds", in: "1700000000", want: unix, ok: true},
		{name: "RFC1123", in: rfc1123.Format(time.RFC1123), want: rfc1123, ok: true},
		{name: "RFC3339", in: rfc3339.Format(time.RFC3339), want: rfc3339, ok: true},
		{name: "empty", in: "", ok: false},
		{name: "garbage", in: "not-a-time", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, ok := parseResetTime(tt.in)

			// Assert
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if tt.ok && !got.Equal(tt.want) {
				t.Fatalf("time = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClientCachesValidateAndMod(t *testing.T) {
	// Arrange
	var now time.Time
	now = time.Unix(1_700_000_000, 0)
	var validateHits, modHits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/validate.json"):
			validateHits++
			writeJSON(w, `{"user_id":1,"name":"A"}`)
		case strings.HasSuffix(r.URL.Path, "/mods/5.json"):
			modHits++
			writeJSON(w, `{"name":"M","mod_id":5}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	client.now = func() time.Time { return now }

	// Act / Assert: second Validate and Mod do not hit the server
	if _, err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if _, err := client.Validate(context.Background()); err != nil {
		t.Fatalf("second Validate() = %v", err)
	}
	if validateHits != 1 {
		t.Fatalf("validate hits = %d, want 1", validateHits)
	}

	if _, err := client.Mod(context.Background(), 5); err != nil {
		t.Fatalf("Mod() = %v", err)
	}
	if _, err := client.Mod(context.Background(), 5); err != nil {
		t.Fatalf("second Mod() = %v", err)
	}
	if modHits != 1 {
		t.Fatalf("mod hits = %d, want 1", modHits)
	}

	// Act: key change invalidates Validate
	client.APIKey = "other-key"
	if _, err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate after key change = %v", err)
	}
	if validateHits != 2 {
		t.Fatalf("validate hits after key change = %d, want 2", validateHits)
	}

	// Act: TTL expiry invalidates Mod
	now = now.Add(metadataCacheTTL + time.Second)
	if _, err := client.Mod(context.Background(), 5); err != nil {
		t.Fatalf("Mod after TTL = %v", err)
	}
	if modHits != 2 {
		t.Fatalf("mod hits after TTL = %d, want 2", modHits)
	}
}

func TestClientInvalidatesValidateCacheOn401(t *testing.T) {
	// Arrange
	var validateHits int
	modStatus := http.StatusOK
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/validate.json") {
			validateHits++
			writeJSON(w, `{"user_id":1,"name":"A"}`)
			return
		}
		w.WriteHeader(modStatus)
		if modStatus == http.StatusOK {
			writeJSON(w, `{"name":"M","mod_id":1}`)
		}
	})
	if _, err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if validateHits != 1 {
		t.Fatalf("validate hits = %d, want 1", validateHits)
	}

	// Act
	modStatus = http.StatusUnauthorized
	_, err := client.Mod(context.Background(), 1)

	// Assert
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Mod() = %v, want ErrInvalidKey", err)
	}
	if _, err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate after 401 = %v", err)
	}
	if validateHits != 2 {
		t.Fatalf("validate hits after 401 = %d, want 2", validateHits)
	}
}

func TestClientRefusesWhenRateLimitExhausted(t *testing.T) {
	// Arrange
	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		writeJSON(w, `{"name":"M","mod_id":1}`)
	})
	client.rateLimit = RateLimit{
		HourlyRemaining: 0,
		HourlyReset:     time.Now().Add(time.Hour),
	}

	// Act
	_, err := client.Mod(context.Background(), 1)

	// Assert
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Mod() = %v, want ErrRateLimited", err)
	}
	if hits != 0 {
		t.Fatalf("handler called %d times, want 0", hits)
	}
}

func TestClientDoesNotPreflightWhenResetUnset(t *testing.T) {
	// Arrange
	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		writeJSON(w, `{"name":"M","mod_id":1}`)
	})
	client.rateLimit = RateLimit{HourlyRemaining: 0}

	// Act
	_, err := client.Mod(context.Background(), 1)

	// Assert
	if err != nil {
		t.Fatalf("Mod() = %v, want no error when reset is unset", err)
	}
	if hits != 1 {
		t.Fatalf("handler called %d times, want 1", hits)
	}
}

func TestClientMaps429AndZerosRemaining(t *testing.T) {
	// Arrange
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RL-Hourly-Remaining", "4")
		w.Header().Set("X-RL-Daily-Remaining", "9")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	// Act
	_, err := client.Mod(context.Background(), 1)

	// Assert
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Mod() = %v, want ErrRateLimited", err)
	}
	got := client.RateLimit()
	if got.HourlyRemaining != 0 || got.DailyRemaining != 0 {
		t.Fatalf("after 429 remaining = hourly %d daily %d, want both 0", got.HourlyRemaining, got.DailyRemaining)
	}
}

func TestClientRejectsHeaderInjectionKey(t *testing.T) {
	// Arrange
	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	})
	client.APIKey = "ok\r\nX-Injected: 1"

	// Act
	_, err := client.Validate(context.Background())

	// Assert
	if err == nil {
		t.Fatal("Validate() succeeded, want rejection of a header-injection key")
	}
	if errors.Is(err, ErrNoAPIKey) {
		t.Fatal("a non-empty invalid key must not be reported as ErrNoAPIKey")
	}
	if hits != 0 {
		t.Fatalf("handler called %d times, want 0", hits)
	}
	if strings.Contains(err.Error(), "ok\r\n") || strings.Contains(err.Error(), "X-Injected") {
		t.Fatalf("error leaked the rejected key: %v", err)
	}
}

func TestClientErrorsDoNotLeakSecrets(t *testing.T) {
	t.Run("500 on keyed download", func(t *testing.T) {
		// Arrange
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "nope")
		})
		client.APIKey = sentinelKey

		// Act
		_, err := client.DownloadLinks(context.Background(), 1, 2, DownloadSecrets{
			Key:     sentinelQuery,
			Expires: "1700000000",
		})

		// Assert
		if err == nil {
			t.Fatal("DownloadLinks() succeeded, want an error")
		}
		assertNoSecretLeak(t, err)
	})

	t.Run("transport error on keyed download", func(t *testing.T) {
		// Arrange
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, `[]`)
		}))
		client := NewClient(sentinelKey, "test")
		client.BaseURL = server.URL
		server.Close()

		// Act
		_, err := client.DownloadLinks(context.Background(), 1, 2, DownloadSecrets{
			Key:     sentinelQuery,
			Expires: "1700000000",
		})

		// Assert
		if err == nil {
			t.Fatal("DownloadLinks() succeeded against a closed server")
		}
		assertNoSecretLeak(t, err)
	})

	t.Run("500 includes API key only as header", func(t *testing.T) {
		// Arrange
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "upstream")
		})
		client.APIKey = sentinelKey

		// Act
		_, err := client.Validate(context.Background())

		// Assert
		if err == nil {
			t.Fatal("Validate() succeeded, want an error")
		}
		assertNoSecretLeak(t, err)
	})
}

func TestClientDownloadLinksAreNotCached(t *testing.T) {
	// Arrange
	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Query().Get("key") != "" {
			t.Errorf("keyless call carried a key query")
		}
		writeJSON(w, `[{"URI":"https://example.invalid/a","name":"CDN","short_name":"cdn"}]`)
	})

	// Act
	if _, err := client.DownloadLinks(context.Background(), 3, 4, DownloadSecrets{}); err != nil {
		t.Fatalf("first DownloadLinks() = %v", err)
	}
	if _, err := client.DownloadLinks(context.Background(), 3, 4, DownloadSecrets{}); err != nil {
		t.Fatalf("second DownloadLinks() = %v", err)
	}

	// Assert
	if hits != 2 {
		t.Fatalf("download_link hits = %d, want 2", hits)
	}
}

func TestClientRefusesRedirectsAndDoesNotForwardAPIKey(t *testing.T) {
	// Arrange
	var leakedKey string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leakedKey = r.Header.Get("apikey")
		writeJSON(w, `{"user_id":1,"name":"nope"}`)
	}))
	t.Cleanup(evil.Close)

	var hits int
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Location", evil.URL+"/stolen")
		w.WriteHeader(http.StatusFound)
	})
	client.APIKey = sentinelKey

	// Act
	_, err := client.Validate(context.Background())

	// Assert
	if err == nil {
		t.Fatal("Validate() followed a redirect, want a refusal")
	}
	if hits != 1 {
		t.Fatalf("origin hits = %d, want 1", hits)
	}
	if leakedKey != "" {
		t.Fatal("apikey was forwarded to the redirect target")
	}
	if !errors.Is(err, errRedirect) {
		t.Fatalf("error = %v, want errRedirect", err)
	}
	assertNoSecretLeak(t, err)
}

func TestClientStatusErrorRedactsSecretBody(t *testing.T) {
	// Arrange
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "rejected key="+sentinelQuery+" apikey="+sentinelKey)
	})
	client.APIKey = sentinelKey

	// Act
	_, err := client.Validate(context.Background())

	// Assert
	if err == nil {
		t.Fatal("Validate() succeeded, want an error")
	}
	assertNoSecretLeak(t, err)
}

func TestClientDownloadLinksSendsKeyQuery(t *testing.T) {
	// Arrange
	var gotQuery string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, `[]`)
	})

	// Act
	_, err := client.DownloadLinks(context.Background(), 3, 4, DownloadSecrets{
		Key:     "abc",
		Expires: "99",
	})

	// Assert
	if err != nil {
		t.Fatalf("DownloadLinks() = %v", err)
	}
	if !strings.Contains(gotQuery, "key=abc") || !strings.Contains(gotQuery, "expires=99") {
		t.Fatalf("query = %q, want key and expires", gotQuery)
	}
}

func assertNoSecretLeak(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	if strings.Contains(msg, sentinelKey) {
		t.Fatalf("error leaked API key SENTINEL: %v", err)
	}
	if strings.Contains(msg, sentinelQuery) {
		t.Fatalf("error leaked download key SENTINELQUERY: %v", err)
	}
	if strings.Contains(msg, "key=") {
		t.Fatalf("error leaked a key= parameter: %v", err)
	}
}
