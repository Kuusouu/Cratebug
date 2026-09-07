package nexus

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/install"
)

func testDownload(ctx context.Context, server *httptest.Server, file FileInfo, onProgress func(install.Progress)) (string, func(), error) {
	return Download(ctx, DownloadLink{URI: server.URL + "/file"}, file, server.Client(), onProgress)
}

func TestDownloadUsesFileInfoName(t *testing.T) {
	// Arrange
	content := "fake-zip-bytes"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	// Act
	path, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "MyMod.zip"}, nil)

	// Assert
	if err != nil {
		t.Fatalf("Download() = %v, want no error", err)
	}
	defer cleanup()
	if filepath.Base(path) != "MyMod.zip" {
		t.Errorf("downloaded file name = %q, want %q", filepath.Base(path), "MyMod.zip")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(got) != content {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
}

func TestDownloadIgnoresURLAndContentDispositionNames(t *testing.T) {
	// Arrange: the CDN path and header would have been the old guess; FileInfo wins.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="FromHeader.7z"`)
		_, _ = w.Write([]byte("content"))
	}))
	t.Cleanup(server.Close)

	// Act
	path, cleanup, err := Download(
		context.Background(),
		DownloadLink{URI: server.URL + "/download?id=123"},
		FileInfo{FileName: "RealModName.7z"},
		server.Client(),
		nil,
	)

	// Assert
	if err != nil {
		t.Fatalf("Download() = %v, want no error", err)
	}
	defer cleanup()
	if filepath.Base(path) != "RealModName.7z" {
		t.Errorf("downloaded file name = %q, want %q", filepath.Base(path), "RealModName.7z")
	}
}

func TestDownloadAcceptsBareBundleExtension(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pak-bytes"))
	}))
	t.Cleanup(server.Close)

	// Act
	path, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Example_9999999_P.pak"}, nil)

	// Assert
	if err != nil {
		t.Fatalf("Download() = %v, want no error", err)
	}
	defer cleanup()
	if filepath.Base(path) != "Example_9999999_P.pak" {
		t.Errorf("downloaded file name = %q, want the .pak name preserved", filepath.Base(path))
	}
}

func TestDownloadRejectsHTTP(t *testing.T) {
	// Act
	_, cleanup, err := Download(context.Background(), DownloadLink{URI: "http://example.invalid/Mod.zip"}, FileInfo{FileName: "Mod.zip"}, nil, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded for an http:// URL, want an error")
	}
}

func TestDownloadRejectsUnsupportedFileNameBeforeRequest(t *testing.T) {
	// Arrange
	var hits int
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("bytes"))
	}))
	t.Cleanup(server.Close)

	// Act
	_, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "readme.txt"}, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded for an unsupported name, want an error")
	}
	if hits != 0 {
		t.Fatalf("handler called %d times, want 0", hits)
	}
}

func TestDownloadRejectsNonOKStatus(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	// Act
	_, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Mod.zip"}, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded against a 404, want an error")
	}
}

func TestDownloadStalledServer(t *testing.T) {
	// Arrange
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })

	// Act
	_, _, err := download(
		context.Background(),
		DownloadLink{URI: server.URL + "/Mod.zip"},
		FileInfo{FileName: "Mod.zip"},
		server.Client(),
		nil,
		100*time.Millisecond,
	)

	// Assert: the failure path self-cleans (nil cleanup).
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("download() error = %v, want a stall failure", err)
	}
}

func TestDownloadReportsProgress(t *testing.T) {
	// Arrange
	content := strings.Repeat("x", 1000)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	var lastProgress install.Progress
	calls := 0

	// Act
	_, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Mod.zip"}, func(p install.Progress) {
		calls++
		lastProgress = p
	})

	// Assert
	if err != nil {
		t.Fatalf("Download() = %v, want no error", err)
	}
	defer cleanup()
	if calls == 0 {
		t.Fatal("onProgress was never called")
	}
	if lastProgress.Phase != "downloading" {
		t.Errorf("Progress.Phase = %q, want %q", lastProgress.Phase, "downloading")
	}
	if lastProgress.Percent != 100.0 {
		t.Errorf("final Progress.Percent = %v, want 100", lastProgress.Percent)
	}
}

func TestDownloadCleanupRemovesTempDirectory(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	t.Cleanup(server.Close)

	path, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Mod.zip"}, nil)
	if err != nil {
		t.Fatalf("Download() = %v, want no error", err)
	}
	dir := filepath.Dir(path)

	// Act
	cleanup()

	// Assert
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("cleanup did not remove the temporary download directory %q", dir)
	}
}

func TestDownloadRejectsHTTPRedirect(t *testing.T) {
	// Arrange
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("downgraded"))
	}))
	t.Cleanup(insecure.Close)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL+"/Mod.zip", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	// Act
	_, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Mod.zip"}, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() followed an HTTP redirect, want an error")
	}
	if !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("error = %v, want a non-HTTPS redirect refusal", err)
	}
}

func TestDownloadRejectsSizeMismatch(t *testing.T) {
	// Arrange: Nexus said ~1 KiB; the CDN offered 20 KiB.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "20000")
		_, _ = w.Write(make([]byte, 20000))
	}))
	t.Cleanup(server.Close)

	// Act
	_, cleanup, err := testDownload(context.Background(), server, FileInfo{FileName: "Mod.zip", SizeKB: 1}, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded with a gross size mismatch, want an error")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want a size-mismatch failure", err)
	}
}

func TestDownloadCallerCancellation(t *testing.T) {
	// Arrange
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	// Act
	_, cleanup, err := testDownload(ctx, server, FileInfo{FileName: "Mod.zip"}, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded after cancel, want an error")
	}
	if strings.Contains(err.Error(), "stalled") {
		t.Fatalf("error = %v, want caller cancellation, not a stall", err)
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "cancel") {
		t.Fatalf("error = %v, want a cancellation failure", err)
	}
}

func TestDownloadErrorsRedactQueryParameters(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	// Act
	_, cleanup, err := Download(
		context.Background(),
		DownloadLink{URI: server.URL + "/file?key=SENTINEL&expires=SENTINEL"},
		FileInfo{FileName: "Mod.zip"},
		server.Client(),
		nil,
	)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("Download() succeeded against a 404, want an error")
	}
	if strings.Contains(err.Error(), "SENTINEL") || strings.Contains(err.Error(), "key=") {
		t.Fatalf("error leaked query secrets: %v", err)
	}
}

func TestSanitizeDownloadFileNameStripsPaths(t *testing.T) {
	// Act
	got := sanitizeDownloadFileName(`..\..\evil.zip`)

	// Assert
	if got != "evil.zip" {
		t.Fatalf("sanitizeDownloadFileName() = %q, want %q", got, "evil.zip")
	}
}

func TestWrapDownloadErrorRedactsQuery(t *testing.T) {
	// Arrange
	err := &url.Error{Op: "Get", URL: "https://cdn.example/file?key=SENTINEL", Err: io.ErrUnexpectedEOF}

	// Act
	got := wrapDownloadError("https://cdn.example/file?key=SENTINEL", err)

	// Assert
	if strings.Contains(got.Error(), "SENTINEL") || strings.Contains(got.Error(), "key=") {
		t.Fatalf("wrapDownloadError leaked query secrets: %v", got)
	}
}
