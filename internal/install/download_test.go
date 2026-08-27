package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadRemoteFile_UsesURLPathFileName(t *testing.T) {
	// Arrange
	content := "fake-zip-bytes"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	// Act
	path, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/MyMod.zip", server.Client(), nil)
	defer cleanup()

	// Assert
	if err != nil {
		t.Fatalf("DownloadRemoteFile returned unexpected error: %v", err)
	}
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

func TestDownloadRemoteFile_PrefersContentDispositionFileName(t *testing.T) {
	// Arrange: a redirect-style URL with no usable path, server names the file instead.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="RealModName.7z"`)
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	// Act
	path, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/download?id=123", server.Client(), nil)
	defer cleanup()

	// Assert
	if err != nil {
		t.Fatalf("DownloadRemoteFile returned unexpected error: %v", err)
	}
	if filepath.Base(path) != "RealModName.7z" {
		t.Errorf("downloaded file name = %q, want %q", filepath.Base(path), "RealModName.7z")
	}
}

func TestDownloadRemoteFile_AcceptsBareBundleExtension(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pak-bytes"))
	}))
	defer server.Close()

	// Act
	path, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/Example_9999999_P.pak", server.Client(), nil)
	defer cleanup()

	// Assert
	if err != nil {
		t.Fatalf("DownloadRemoteFile returned unexpected error: %v", err)
	}
	if filepath.Base(path) != "Example_9999999_P.pak" {
		t.Errorf("downloaded file name = %q, want the .pak name preserved", filepath.Base(path))
	}
}

func TestDownloadRemoteFile_RejectsHTTP(t *testing.T) {
	// Act
	_, cleanup, err := DownloadRemoteFile(context.Background(), "http://example.invalid/Mod.zip", nil, nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("DownloadRemoteFile succeeded for an http:// URL, want an error")
	}
}

func TestDownloadRemoteFile_RejectsUnrecognizableFileName(t *testing.T) {
	// Arrange: no extension in the URL path and no Content-Disposition header at all.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("bytes"))
	}))
	defer server.Close()

	// Act
	_, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/download?id=123", server.Client(), nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("DownloadRemoteFile succeeded with no determinable file name, want an error")
	}
}

func TestDownloadRemoteFile_RejectsNonOKStatus(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Act
	_, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/Mod.zip", server.Client(), nil)

	// Assert
	if err == nil {
		cleanup()
		t.Fatal("DownloadRemoteFile succeeded against a 404, want an error")
	}
}

func TestDownloadRemoteFile_ReportsProgress(t *testing.T) {
	// Arrange
	content := strings.Repeat("x", 1000)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	var lastProgress Progress
	calls := 0

	// Act
	_, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/Mod.zip", server.Client(), func(p Progress) {
		calls++
		lastProgress = p
	})
	defer cleanup()

	// Assert
	if err != nil {
		t.Fatalf("DownloadRemoteFile returned unexpected error: %v", err)
	}
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

func TestDownloadRemoteFile_CleanupRemovesTempDirectory(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	path, cleanup, err := DownloadRemoteFile(context.Background(), server.URL+"/Mod.zip", server.Client(), nil)
	if err != nil {
		t.Fatalf("DownloadRemoteFile returned unexpected error: %v", err)
	}
	dir := filepath.Dir(path)

	// Act
	cleanup()

	// Assert
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Errorf("cleanup did not remove the temporary download directory %q", dir)
	}
}
