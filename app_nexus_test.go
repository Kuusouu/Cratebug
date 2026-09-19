package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/modtype"
	"github.com/Kuusouu/Cratebug/internal/nexus"
	"github.com/Kuusouu/Cratebug/internal/secret"
)

const (
	adultFixtureModID    = 9
	adultFixtureFileID   = 2
	adultNameSentinel    = "NSFW Adult Skin SENTINEL"
	adultAuthorSentinel  = "Adult Author SENTINEL"
	adultPictureSentinel = "https://example.invalid/adult-sentinel.png"
	safeModName          = "Safe Skin"
)

type nexusMockAPIConfig struct {
	adultMod  bool
	gqlAdult  bool
	adultPref bool
	blocking  bool
	gqlHide   bool
	gqlHTTP   int
	prefsHTTP int
}

type nexusMockAPI struct {
	URL      string
	mod      int
	prefs    int
	files    int
	file     int
	download int
}

func TestSetNexusAPIKeyRejectsInvalidKeys(t *testing.T) {
	tooLong := strings.Repeat("a", 513)
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty", key: ""},
		{name: "whitespace only", key: "   "},
		{name: "tab", key: "abc\tdef"},
		{name: "newline", key: "abc\ndef"},
		{name: "carriage return", key: "abc\rdef"},
		{name: "control character", key: "abc\x01def"},
		{name: "too long", key: tooLong},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			app := testApp(t, false)

			// Act
			state, err := app.SetNexusAPIKey(test.key)

			// Assert
			if err == nil {
				t.Fatal("SetNexusAPIKey() succeeded, want an error")
			}
			if state.Configured {
				t.Fatal("Configured = true after a rejected key, want false")
			}
			if app.NexusKeyState().Configured {
				t.Fatal("NexusKeyState().Configured = true after a rejected key, want false")
			}
		})
	}
}

func TestNexusKeyStateTracksConfiguration(t *testing.T) {
	// Arrange
	server := newNexusValidateServer(t, true)
	app := testApp(t, false)
	app.nexusBaseURL = server.URL

	if app.NexusKeyState().Configured {
		t.Fatal("NexusKeyState().Configured = true on a fresh store, want false")
	}

	// Act
	if _, err := app.SetNexusAPIKey("valid-test-key"); err != nil {
		t.Fatalf("SetNexusAPIKey() = %v", err)
	}

	// Assert
	if !app.NexusKeyState().Configured {
		t.Fatal("NexusKeyState().Configured = false after Set, want true")
	}

	// Act
	if _, err := app.ClearNexusAPIKey(); err != nil {
		t.Fatalf("ClearNexusAPIKey() = %v", err)
	}

	// Assert
	if app.NexusKeyState().Configured {
		t.Fatal("NexusKeyState().Configured = true after Clear, want false")
	}
}

func TestSetNexusAPIKeyRejectsUnverifiedKeys(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)
	app := testApp(t, false)
	app.nexusBaseURL = server.URL

	// Act
	state, err := app.SetNexusAPIKey("looks-valid-but-rejected")

	// Assert
	if !errors.Is(err, nexus.ErrInvalidKey) {
		t.Fatalf("SetNexusAPIKey() error = %v, want ErrInvalidKey", err)
	}
	if state.Configured {
		t.Fatal("Configured = true after an unverified key, want false")
	}
}

func TestTakePendingNexusLinkIsTakeOnce(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.setLaunchURL("nxm://marvelrivals/mods/12/files/34")

	// Act
	first, err := app.TakePendingNexusLink()
	if err != nil {
		t.Fatalf("first TakePendingNexusLink() = %v", err)
	}
	second, err := app.TakePendingNexusLink()
	if err != nil {
		t.Fatalf("second TakePendingNexusLink() = %v", err)
	}

	// Assert
	if !first.Present {
		t.Fatal("first TakePendingNexusLink().Present = false, want true")
	}
	if first.ModID != 12 || first.FileID != 34 {
		t.Errorf("first link = mod %d file %d, want 12/34", first.ModID, first.FileID)
	}
	if second.Present {
		t.Fatal("second TakePendingNexusLink().Present = true, want false")
	}
}

func TestPendingNexusLinkConcurrentSetAndTake(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// Act
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			app.setLaunchURL("nxm://marvelrivals/mods/1/files/2?key=k&expires=1")
		}()
		go func() {
			defer wg.Done()
			_, _ = app.TakePendingNexusLink()
		}()
	}
	wg.Wait()

	// Assert — drain whatever the last writer left; the race detector is
	// the real check that Set/Take do not share unsynchronised state.
	if _, err := app.TakePendingNexusLink(); err != nil {
		t.Fatalf("final TakePendingNexusLink() = %v", err)
	}
}

func TestNexusLinkPayloadOmitsSecrets(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	app.setLaunchURL("nxm://marvelrivals/mods/1/files/2?key=SENTINEL&expires=SENTINEL")

	// Act
	app.pendingURLMu.Lock()
	if app.pendingLink == nil {
		app.pendingURLMu.Unlock()
		t.Fatal("pendingLink is nil after setLaunchURL")
	}
	pendingJSON, err := json.Marshal(app.pendingLink)
	if err != nil {
		app.pendingURLMu.Unlock()
		t.Fatal(err)
	}
	secrets := app.linkSecrets[linkSecretKey(1, 2)]
	app.pendingURLMu.Unlock()

	link, err := app.TakePendingNexusLink()
	if err != nil {
		t.Fatalf("TakePendingNexusLink() = %v", err)
	}
	linkJSON, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if secrets.Key != "SENTINEL" || secrets.Expires != "SENTINEL" {
		t.Fatalf("stored secrets = %+v, want SENTINEL/SENTINEL (kept off the payload)", secrets)
	}
	if strings.Contains(string(pendingJSON), "SENTINEL") {
		t.Fatalf("nexus:link payload %s contains SENTINEL", pendingJSON)
	}
	if strings.Contains(string(linkJSON), "SENTINEL") {
		t.Fatalf("TakePendingNexusLink JSON %s contains SENTINEL", linkJSON)
	}
}

func TestNexusAccountReportsVerifiedPremiumUser(t *testing.T) {
	// Arrange
	server := newNexusValidateServer(t, true)
	app := testApp(t, false)
	app.nexusBaseURL = server.URL
	if _, err := app.SetNexusAPIKey("valid-test-key"); err != nil {
		t.Fatalf("SetNexusAPIKey() = %v", err)
	}

	// Act
	account := app.NexusAccount()

	// Assert
	if !account.Configured || !account.Verified {
		t.Fatalf("account = %+v, want configured and verified", account)
	}
	if account.Name != "TestUser" {
		t.Errorf("Name = %q, want TestUser", account.Name)
	}
	if !account.IsPremium {
		t.Fatal("IsPremium = false, want true")
	}
}

func TestPrepareNexusInstallRequiresAPIKey(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	_, err := app.PrepareNexusInstall(t.TempDir(), 1, 2, "")

	// Assert
	if !errors.Is(err, nexus.ErrNoAPIKey) {
		t.Fatalf("PrepareNexusInstall() error = %v, want ErrNoAPIKey", err)
	}
}

func TestNexusAccountWithoutKeyIsEmpty(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act
	account := app.NexusAccount()

	// Assert
	if account != (NexusAccountState{}) {
		t.Fatalf("NexusAccount() = %+v, want the zero value", account)
	}
}

func TestNexusAccountUnreadableKeyIsUnverified(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "nexus.key")
	if err := os.WriteFile(path, []byte("not a valid key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyTable := modtype.CharacterTable{}
	app := newApp(
		staticGameRunningChecker{},
		testMetadataStore(t),
		nil,
		&emptyTable,
		nil,
		secret.NewPlainStore(path, []byte("test-entropy")),
	)

	// Act
	account := app.NexusAccount()

	// Assert
	if !account.Configured {
		t.Fatal("Configured = false when a key file exists, want true")
	}
	if account.Verified {
		t.Fatal("Verified = true for an unreadable key, want false")
	}
}

func TestResolveNexusModPageAdultGate(t *testing.T) {
	pageURL := fmt.Sprintf("https://www.nexusmods.com/marvelrivals/mods/%d", adultFixtureModID)

	tests := []struct {
		name      string
		adultMod  bool
		gqlAdult  bool
		adultPref bool
		blocking  bool
		gqlHide   bool
		gqlHTTP   int
		prefsHTTP int
		wantErr   error
		wantName  string
		wantPrefs int
		wantFiles int
	}{
		{
			name:      "blocks adult when preference is off",
			adultMod:  true,
			wantErr:   nexus.ErrAdultContentBlocked,
			wantPrefs: 1,
		},
		{
			name:    "blocks adult REST missed when GraphQL hides it",
			gqlHide: true,
			wantErr: nexus.ErrAdultContentBlocked,
		},
		{
			name:      "blocks GraphQL-adult when REST omitted the flag",
			gqlAdult:  true,
			wantErr:   nexus.ErrAdultContentBlocked,
			wantPrefs: 1,
		},
		{
			name:      "blocks when GraphQL fails and preference is off",
			gqlHTTP:   http.StatusInternalServerError,
			wantErr:   nexus.ErrAdultContentBlocked,
			wantPrefs: 1,
		},
		{
			name:      "blocks when content blocking is on",
			adultMod:  true,
			adultPref: true,
			blocking:  true,
			wantErr:   nexus.ErrAdultContentBlocked,
			wantPrefs: 1,
		},
		{
			name:      "allows adult when preference is on",
			adultMod:  true,
			adultPref: true,
			wantName:  adultNameSentinel,
			wantPrefs: 1,
			wantFiles: 1,
		},
		{
			name:      "allows non-adult when preferences fail",
			prefsHTTP: http.StatusInternalServerError,
			wantName:  safeModName,
			wantFiles: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			api := newNexusMockAPI(t, nexusMockAPIConfig{
				adultMod:  test.adultMod,
				gqlAdult:  test.gqlAdult,
				adultPref: test.adultPref,
				blocking:  test.blocking,
				gqlHide:   test.gqlHide,
				gqlHTTP:   test.gqlHTTP,
				prefsHTTP: test.prefsHTTP,
			})
			app := connectedNexusApp(t, api.URL)

			// Act
			link, err := app.ResolveNexusModPage(pageURL)

			// Assert
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("ResolveNexusModPage() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				assertBlockedNexusLink(t, link)
			} else if link.ModName != test.wantName {
				t.Fatalf("ModName = %q, want %q", link.ModName, test.wantName)
			}
			if api.prefs != test.wantPrefs || api.files != test.wantFiles {
				t.Fatalf("hits prefs=%d files=%d, want prefs=%d files=%d", api.prefs, api.files, test.wantPrefs, test.wantFiles)
			}
		})
	}
}

func TestTakePendingNexusLinkAdultGate(t *testing.T) {
	nxmURL := fmt.Sprintf(
		"nxm://marvelrivals/mods/%d/files/%d?key=SENTINEL&expires=SENTINEL",
		adultFixtureModID,
		adultFixtureFileID,
	)

	tests := []struct {
		name      string
		adultMod  bool
		gqlAdult  bool
		adultPref bool
		gqlHTTP   int
		wantErr   error
		wantName  string
		wantFile  int
		keepKey   bool
	}{
		{
			name:     "blocks adult without metadata or secrets",
			adultMod: true,
			wantErr:  nexus.ErrAdultContentBlocked,
		},
		{
			name:     "blocks GraphQL-adult nxm when REST omitted the flag",
			gqlAdult: true,
			wantErr:  nexus.ErrAdultContentBlocked,
		},
		{
			name:    "blocks nxm when GraphQL fails and preference is off",
			gqlHTTP: http.StatusInternalServerError,
			wantErr: nexus.ErrAdultContentBlocked,
		},
		{
			name:      "allows adult when preference is on",
			adultMod:  true,
			adultPref: true,
			wantName:  adultNameSentinel,
			wantFile:  1,
			keepKey:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			api := newNexusMockAPI(t, nexusMockAPIConfig{
				adultMod:  test.adultMod,
				gqlAdult:  test.gqlAdult,
				adultPref: test.adultPref,
				gqlHTTP:   test.gqlHTTP,
			})
			app := connectedNexusApp(t, api.URL)
			app.setLaunchURL(nxmURL)

			// Act
			link, err := app.TakePendingNexusLink()

			// Assert
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("TakePendingNexusLink() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr != nil {
				assertBlockedNexusLink(t, link)
			} else if link.ModName != test.wantName {
				t.Fatalf("ModName = %q, want %q", link.ModName, test.wantName)
			}
			if api.file != test.wantFile || api.download != 0 {
				t.Fatalf("hits file=%d download=%d, want file=%d download=0", api.file, api.download, test.wantFile)
			}
			_, hasSecrets := app.peekLinkSecrets(adultFixtureModID, adultFixtureFileID)
			if hasSecrets != test.keepKey {
				t.Fatalf("secrets present = %v, want %v", hasSecrets, test.keepKey)
			}
		})
	}
}

func TestPrepareNexusInstallAdultGate(t *testing.T) {
	tests := []struct {
		name         string
		adultMod     bool
		gqlAdult     bool
		adultPref    bool
		blocking     bool
		gqlHide      bool
		gqlHTTP      int
		wantErr      error
		wantFile     int
		wantDownload int
	}{
		{
			name:     "blocks adult before file and download",
			adultMod: true,
			wantErr:  nexus.ErrAdultContentBlocked,
		},
		{
			name:    "blocks when REST omits the adult flag",
			gqlHide: true,
			wantErr: nexus.ErrAdultContentBlocked,
		},
		{
			name:     "blocks GraphQL-adult when REST omitted the flag",
			gqlAdult: true,
			wantErr:  nexus.ErrAdultContentBlocked,
		},
		{
			name:    "blocks when GraphQL fails and preference is off",
			gqlHTTP: http.StatusInternalServerError,
			wantErr: nexus.ErrAdultContentBlocked,
		},
		{
			name:      "blocks when content blocking is on",
			adultMod:  true,
			adultPref: true,
			blocking:  true,
			wantErr:   nexus.ErrAdultContentBlocked,
		},
		{
			name:         "allows adult past the gate",
			adultMod:     true,
			adultPref:    true,
			wantFile:     1,
			wantDownload: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			api := newNexusMockAPI(t, nexusMockAPIConfig{
				adultMod:  test.adultMod,
				gqlAdult:  test.gqlAdult,
				adultPref: test.adultPref,
				blocking:  test.blocking,
				gqlHide:   test.gqlHide,
				gqlHTTP:   test.gqlHTTP,
			})
			app := connectedNexusApp(t, api.URL)

			// Act
			_, err := app.PrepareNexusInstall(t.TempDir(), adultFixtureModID, adultFixtureFileID, "")

			// Assert
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("PrepareNexusInstall() error = %v, want %v", err, test.wantErr)
				}
			} else if errors.Is(err, nexus.ErrAdultContentBlocked) {
				t.Fatal("PrepareNexusInstall() blocked adult content when preference is on")
			}
			if api.file != test.wantFile || api.download != test.wantDownload {
				t.Fatalf("hits file=%d download=%d, want file=%d download=%d", api.file, api.download, test.wantFile, test.wantDownload)
			}
		})
	}
}

func newNexusValidateServer(t *testing.T, premium bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/validate.json" {
			http.NotFound(w, r)
			return
		}
		writeNexusJSON(w, `{"user_id":1,"name":"TestUser","is_premium":`+boolJSON(premium)+`,"key":"should-not-leak"}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func boolJSON(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func writeNexusJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RL-Hourly-Remaining", "100")
	w.Header().Set("X-RL-Daily-Remaining", "1000")
	_, _ = w.Write([]byte(body))
}

func newNexusMockAPI(t *testing.T, cfg nexusMockAPIConfig) *nexusMockAPI {
	t.Helper()

	mod := nexus.ModInfo{
		Name:    safeModName,
		Author:  "Author",
		Version: "1.0",
		ModID:   adultFixtureModID,
	}
	if cfg.adultMod {
		mod.Name = adultNameSentinel
		mod.Author = adultAuthorSentinel
		mod.PictureURL = adultPictureSentinel
		mod.ContainsAdultContent = true
	}
	file := nexus.FileInfo{
		FileID:       adultFixtureFileID,
		Name:         "Main",
		FileName:     "mod.zip",
		Version:      "1.0",
		SizeKB:       4,
		CategoryName: "MAIN",
		IsPrimary:    true,
	}

	modBody := marshalJSON(t, mod)
	filesBody := marshalJSON(t, struct {
		Files []nexus.FileInfo `json:"files"`
	}{Files: []nexus.FileInfo{file}})
	fileBody := marshalJSON(t, file)
	prefsBody := fmt.Sprintf(
		`{"data":{"preferences":{"adult":%s,"isBlockingContent":%s}}}`,
		boolJSON(cfg.adultPref),
		boolJSON(cfg.blocking),
	)

	api := &nexusMockAPI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch path := r.URL.Path; {
		case path == "/v1/users/validate.json":
			writeNexusJSON(w, `{"user_id":1,"name":"TestUser","is_premium":true,"key":"should-not-leak"}`)
		case path == "/v2/graphql":
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "preferences") {
				api.prefs++
				if cfg.prefsHTTP != 0 {
					w.WriteHeader(cfg.prefsHTTP)
					return
				}
				writeNexusJSON(w, prefsBody)
				return
			}
			if cfg.gqlHTTP != 0 {
				w.WriteHeader(cfg.gqlHTTP)
				return
			}
			if cfg.gqlHide {
				writeNexusJSON(w, `{"errors":[{"message":"Adult content blocked","extensions":{"code":"ADULT_CONTENT_BLOCKED"}}]}`)
				return
			}
			adultFlag := boolJSON(cfg.adultMod || cfg.gqlAdult)
			writeNexusJSON(w, `{"data":{"legacyModsByDomain":{"nodes":[{"modId":9,"adult":`+adultFlag+`,"adultContent":`+adultFlag+`}]}}}`)
		case strings.HasSuffix(path, "/download_link.json"):
			api.download++
			writeNexusJSON(w, "[]")
		case strings.Contains(path, "/files/") && strings.HasSuffix(path, ".json"):
			api.file++
			writeNexusJSON(w, fileBody)
		case strings.HasSuffix(path, "/files.json"):
			api.files++
			writeNexusJSON(w, filesBody)
		case strings.Contains(path, "/mods/") && strings.HasSuffix(path, ".json"):
			api.mod++
			writeNexusJSON(w, modBody)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	api.URL = server.URL
	return api
}

func marshalJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func connectedNexusApp(t *testing.T, baseURL string) *App {
	t.Helper()
	app := testApp(t, false)
	app.nexusBaseURL = baseURL
	if _, err := app.SetNexusAPIKey("valid-test-key"); err != nil {
		t.Fatalf("SetNexusAPIKey() = %v", err)
	}
	return app
}

func assertBlockedNexusLink(t *testing.T, link NexusLink) {
	t.Helper()
	if link.ModName != "" || link.Author != "" || link.PictureURL != "" || len(link.Files) != 0 {
		t.Fatalf("blocked NexusLink still has metadata: %+v", link)
	}
	raw, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SENTINEL") {
		t.Fatalf("blocked NexusLink JSON leaked adult metadata: %s", raw)
	}
}
