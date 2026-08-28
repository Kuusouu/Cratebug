package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestValidateAssetURL(t *testing.T) {
	client := &Client{Owner: "Kuusouu", Repo: "Cratebug"}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid release download", url: "https://github.com/Kuusouu/Cratebug/releases/download/2026.08.27/Cratebug-amd64-installer.exe"},
		{name: "http instead of https", url: "http://github.com/Kuusouu/Cratebug/releases/download/2026.08.27/x.exe", wantErr: true},
		{name: "wrong host", url: "https://evil.example/Kuusouu/Cratebug/releases/download/2026.08.27/x.exe", wantErr: true},
		{name: "wrong repo", url: "https://github.com/Someone/Else/releases/download/2026.08.27/x.exe", wantErr: true},
		{name: "not a release download path", url: "https://github.com/Kuusouu/Cratebug/archive/refs/heads/master.zip", wantErr: true},
		{name: "malformed url", url: "://not a url", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.ValidateAssetURL(tt.url)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateAssetURL(%q) = nil, want error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateAssetURL(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

// ValidateAssetURL requires the asset host to be github.com, so a local
// httptest.Server (which never has that hostname) can't be pointed at
// directly. redirectToGithubHost rewrites the outgoing request's host back
// to the test server while leaving req.URL, and therefore the earlier
// ValidateAssetURL check against the original github.com URL, untouched.
type redirectToGithubHost struct {
	serverURL *url.URL
}

func (t redirectToGithubHost) RoundTrip(req *http.Request) (*http.Response, error) {
	target := req.Clone(req.Context())
	target.URL.Scheme = t.serverURL.Scheme
	target.URL.Host = t.serverURL.Host
	target.Host = t.serverURL.Host
	return http.DefaultTransport.RoundTrip(target)
}

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parsing test server URL: %v", err)
	}

	return &Client{
		Owner:      "Kuusouu",
		Repo:       "Cratebug",
		HTTPClient: &http.Client{Transport: redirectToGithubHost{serverURL: serverURL}},
	}
}

const testAssetURL = "https://github.com/Kuusouu/Cratebug/releases/download/2026.08.27/Cratebug-amd64-installer.exe"

// The regression this guards: the default download client once inherited
// httpClient()'s 15s metadata timeout, whose body-inclusive cap failed any
// installer download slower than size/15s with no way for retries to recover.
func TestDownloadClientHasNoOverallTimeout(t *testing.T) {
	client := &Client{Owner: "Kuusouu", Repo: "Cratebug"}

	if got := client.downloadHTTPClient().Timeout; got != 0 {
		t.Errorf("download client Timeout = %s, want 0: liveness is bounded by the header and read-idle timeouts instead", got)
	}
}

func TestDownloadOnceStalledBodyFailsAsRetryable(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-release
	}))
	t.Cleanup(server.Close)
	// Registered after server.Close so cleanup (LIFO) releases the blocked
	// handler before Close waits on it.
	t.Cleanup(func() { close(release) })

	client := &Client{Owner: "Kuusouu", Repo: "Cratebug"}
	destPath := filepath.Join(t.TempDir(), "installer.exe.download")

	err := client.downloadOnce(context.Background(), server.URL, destPath, 100*time.Millisecond, nil)

	if !errors.Is(err, errDownloadStalled) {
		t.Fatalf("downloadOnce error = %v, want errDownloadStalled", err)
	}
	if !isRetryable(err) {
		t.Error("a stalled download must be classified retryable, unlike a caller cancel")
	}
}

func TestDownloadSucceeds(t *testing.T) {
	content := strings.Repeat("cratebug-installer-bytes", 1000)
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		_, _ = w.Write([]byte(content))
	})
	asset := ReleaseAsset{Name: "Cratebug-amd64-installer.exe", URL: testAssetURL}
	destDir := t.TempDir()

	var lastDownloaded, lastTotal int64
	path, err := client.Download(context.Background(), asset, destDir, func(downloaded, total int64) {
		lastDownloaded, lastTotal = downloaded, total
	})
	if err != nil {
		t.Fatalf("Download returned unexpected error: %v", err)
	}
	if filepath.Base(path) != asset.Name {
		t.Errorf("Download path = %q, want it to end with %q", path, asset.Name)
	}
	if strings.HasSuffix(path, ".download") {
		t.Errorf("Download left the temp .download suffix on the final path: %q", path)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != content {
		t.Errorf("downloaded content mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
	if lastDownloaded != int64(len(content)) {
		t.Errorf("final progress downloaded = %d, want %d", lastDownloaded, len(content))
	}
	if lastTotal != int64(len(content)) {
		t.Errorf("final progress total = %d, want %d", lastTotal, len(content))
	}
	if _, err := os.Stat(path + ".download"); !os.IsNotExist(err) {
		t.Error("temp .download file was not cleaned up")
	}
}

func TestDownloadSanitizesAssetName(t *testing.T) {
	content := "safe-bytes"
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	})

	t.Run("traversal name is reduced to its base inside destDir", func(t *testing.T) {
		asset := ReleaseAsset{Name: `..\..\evil.exe`, URL: testAssetURL}
		destDir := t.TempDir()

		path, err := client.Download(context.Background(), asset, destDir, nil)
		if err != nil {
			t.Fatalf("Download returned unexpected error: %v", err)
		}
		if filepath.Dir(path) != destDir {
			t.Errorf("download path %q escaped the destination directory %q", path, destDir)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading downloaded file: %v", err)
		}
		if string(got) != content {
			t.Errorf("downloaded content = %q, want %q", got, content)
		}
	})

	t.Run("unusable name fails the download", func(t *testing.T) {
		asset := ReleaseAsset{Name: "..", URL: testAssetURL}

		if _, err := client.Download(context.Background(), asset, t.TempDir(), nil); err == nil {
			t.Fatal("Download succeeded for an unusable asset name, want an error")
		}
	})
}

func TestDownloadRetriesOnServerErrorThenSucceeds(t *testing.T) {
	var attempts int32
	content := "eventually-succeeds"
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(content))
	})
	asset := ReleaseAsset{Name: "installer.exe", URL: testAssetURL}

	path, err := client.Download(context.Background(), asset, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Download returned unexpected error after retries: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != content {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want exactly 3 (2 failures + 1 success)", attempts)
	}
}

func TestDownloadDoesNotRetryOnNotFound(t *testing.T) {
	var attempts int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
	})
	asset := ReleaseAsset{Name: "installer.exe", URL: testAssetURL}
	destDir := t.TempDir()

	_, err := client.Download(context.Background(), asset, destDir, nil)
	if err == nil {
		t.Fatal("Download returned nil error, want an error for a 404")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want exactly 1 (404 is not retryable)", attempts)
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "installer.exe.download")); !os.IsNotExist(statErr) {
		t.Error("failed download left a stray .download file behind")
	}
}
