package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &Client{Owner: "Kuusouu", Repo: "Cratebug", BaseURL: server.URL}
}

func TestClientLatestParsesRelease(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/Kuusouu/Cratebug/releases/latest" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "2026.08.27",
			"html_url": "https://github.com/Kuusouu/Cratebug/releases/tag/2026.08.27",
			"body": "### Added\n- Automatic updates",
			"assets": [
				{"name": "Cratebug-amd64-installer.exe", "browser_download_url": "https://github.com/Kuusouu/Cratebug/releases/download/2026.08.27/Cratebug-amd64-installer.exe"}
			]
		}`))
	})

	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest returned unexpected error: %v", err)
	}
	if release.Version.Tag != "2026.08.27" {
		t.Errorf("Version.Tag = %q, want %q", release.Version.Tag, "2026.08.27")
	}
	if release.Notes != "### Added\n- Automatic updates" {
		t.Errorf("Notes = %q, unexpected", release.Notes)
	}
	if release.Asset.Name != "Cratebug-amd64-installer.exe" {
		t.Errorf("Asset.Name = %q, want the installer exe", release.Asset.Name)
	}
	if !strings.HasSuffix(release.Asset.URL, "Cratebug-amd64-installer.exe") {
		t.Errorf("Asset.URL = %q, want it to point at the installer", release.Asset.URL)
	}
}

func TestClientTagRequestsSpecificTag(t *testing.T) {
	var requestedPath string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "2026.08.20-rc1",
			"body": "Testing",
			"assets": [{"name": "Cratebug-amd64-installer.exe", "browser_download_url": "https://example.invalid/x.exe"}]
		}`))
	})

	release, err := client.Tag(context.Background(), "2026.08.20-rc1")
	if err != nil {
		t.Fatalf("Tag returned unexpected error: %v", err)
	}
	if want := "/repos/Kuusouu/Cratebug/releases/tags/2026.08.20-rc1"; requestedPath != want {
		t.Errorf("requested path = %q, want %q", requestedPath, want)
	}
	if release.Version.Prerelease != "rc1" {
		t.Errorf("Prerelease = %q, want %q", release.Version.Prerelease, "rc1")
	}
}

func TestClientNotFoundReturnsErrNoRelease(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.Latest(context.Background())
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("Latest error = %v, want ErrNoRelease", err)
	}
}

func TestClientServerErrorReturnsDescriptiveError(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("something broke"))
	})

	_, err := client.Latest(context.Background())
	if err == nil {
		t.Fatal("Latest returned nil error, want a descriptive failure")
	}
	if errors.Is(err, ErrNoRelease) {
		t.Fatal("a 500 should not be reported as ErrNoRelease")
	}
}

func TestClientRejectsReleaseWithoutInstallerAsset(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "2026.08.27", "assets": [{"name": "source.zip", "browser_download_url": "https://example.invalid/source.zip"}]}`))
	})

	_, err := client.Latest(context.Background())
	if err == nil {
		t.Fatal("Latest returned nil error, want an error for a release with no .exe asset")
	}
}

func TestClientRejectsUnparseableTag(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "not-a-calver-tag", "assets": [{"name": "installer.exe", "browser_download_url": "https://example.invalid/installer.exe"}]}`))
	})

	_, err := client.Latest(context.Background())
	if err == nil {
		t.Fatal("Latest returned nil error, want an error for an unparseable release tag")
	}
}
