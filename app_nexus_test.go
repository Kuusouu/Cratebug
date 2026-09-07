package main

import (
	"encoding/json"
	"errors"
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

func newNexusValidateServer(t *testing.T, premium bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/validate.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RL-Hourly-Remaining", "100")
		w.Header().Set("X-RL-Daily-Remaining", "1000")
		_, _ = w.Write([]byte(`{"user_id":1,"name":"TestUser","is_premium":` + boolJSON(premium) + `,"key":"should-not-leak"}`))
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
