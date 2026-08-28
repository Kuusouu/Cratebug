package install

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Prefix for the temporary directory a remote download is written into,
// kept separate from staging session directories (SessionManager.CreateSession)
// since a download's own temp file is cleaned up as soon as staging has
// copied out of it, well before the staging session itself is done with.
const remoteDownloadDirPrefix = "cratebug-remote-download-"

const (
	// Size of each chunk read from the response body while streaming a
	// download to disk.
	downloadChunkSize = 32 * 1024

	// Bounds server response headers on a download: a server that accepts
	// the connection but never responds fails after this long instead of
	// hanging. Dial and TLS setup keep the transport's own defaults.
	downloadResponseHeaderTimeout = 30 * time.Second

	// Bounds a stalled body rather than the download as a whole: the
	// download fails only after this long with no new bytes. There is
	// deliberately no overall timeout -- a slow but progressing download of
	// a large mod must be allowed to finish.
	downloadReadIdleTimeout = 30 * time.Second
)

// Downloads the mod archive or bundle at rawURL into a fresh temporary
// directory and returns its local path in the same form CreateSession
// already accepts for a locally-selected file, so the rest of the staged-
// install pipeline treats a remote download exactly like a local one.
// cleanup removes the temporary download directory; callers should defer it
// once the returned path has been handed to StageFiles, whether staging
// succeeds or fails.
//
// HTTPS is required: a plain HTTP download is vulnerable to a
// man-in-the-middle substituting a different archive in transit, on top of
// the mod content itself already being treated as untrusted regardless of
// origin.
//
// httpClient is nil in production (defaulting to a client bounded by
// downloadResponseHeaderTimeout); tests pass an httptest.Server's own client
// so the HTTPS requirement can be exercised against a real TLS connection
// instead of only ever failing it.
func DownloadRemoteFile(ctx context.Context, rawURL string, httpClient *http.Client, onProgress func(Progress)) (downloadPath string, cleanup func(), err error) {
	return downloadRemoteFile(ctx, rawURL, httpClient, onProgress, downloadReadIdleTimeout)
}

func downloadRemoteFile(ctx context.Context, rawURL string, httpClient *http.Client, onProgress func(Progress), idleTimeout time.Duration) (downloadPath string, cleanup func(), err error) {
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = downloadResponseHeaderTimeout
		httpClient = &http.Client{Transport: transport}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("%q is not a valid URL: %w", rawURL, err)
	}
	if parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("download URL must be HTTPS, got %q", rawURL)
	}

	// The idle timer interrupts a blocked body read by cancelling this
	// download's context. stalled distinguishes that from the caller
	// cancelling, so a stall reports as its own failure.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("User-Agent", "Cratebug")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("download %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download %q returned %s", rawURL, resp.Status)
	}

	// Armed only once the body starts: dial, TLS, and header latency have
	// their own transport bound, and together they can legitimately exceed
	// the idle timeout.
	idle := time.AfterFunc(idleTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	fileName, ok := remoteDownloadFileName(resp.Header.Get("Content-Disposition"), parsed)
	if !ok {
		return "", nil, fmt.Errorf(
			"could not determine a file name with a supported extension from %q; use a direct download link ending in one of the supported archive or mod file types",
			rawURL,
		)
	}

	destDir, err := os.MkdirTemp("", remoteDownloadDirPrefix)
	if err != nil {
		return "", nil, fmt.Errorf("create temporary download directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(destDir) }

	destPath := filepath.Join(destDir, fileName)
	out, err := os.Create(destPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("create downloaded file %q: %w", destPath, err)
	}
	defer out.Close()

	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	writer := io.Writer(out)
	if onProgress != nil {
		writer = &progressWriter{w: out, total: total, onProgress: onProgress}
	}

	buf := make([]byte, downloadChunkSize)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			idle.Reset(idleTimeout)
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				cleanup()
				return "", nil, fmt.Errorf("write downloaded file %q: %w", destPath, writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanup()
			if stalled.Load() {
				return "", nil, fmt.Errorf("download %q stalled: no bytes received for %s", rawURL, idleTimeout)
			}
			return "", nil, fmt.Errorf("read download stream from %q: %w", rawURL, readErr)
		}
	}

	return destPath, cleanup, nil
}

// Wraps an io.Writer to report Progress in the "downloading" phase as bytes
// are written, matching StageFiles' own Progress reporting so App can feed
// both into the same "install:progress" event stream.
type progressWriter struct {
	w          io.Writer
	total      int64
	written    int64
	onProgress func(Progress)
}

func (p *progressWriter) Write(chunk []byte) (int, error) {
	n, err := p.w.Write(chunk)
	p.written += int64(n)

	percent := 0.0
	if p.total > 0 {
		percent = float64(p.written) / float64(p.total) * 100.0
	}
	p.onProgress(Progress{
		Phase:   "downloading",
		Current: 1,
		Total:   1,
		Message: "Downloading mod from URL...",
		Percent: percent,
	})
	return n, err
}

// Picks a local file name for a remote download, preferring the server's
// own Content-Disposition filename (most direct download links set one)
// over the URL's path segment, since a redirect-style link's path rarely
// carries a real name. Reports false when neither source yields a name with
// a supported extension, rather than guessing one that later staging steps
// (which decide archive-vs-bundle handling by extension) could get wrong.
func remoteDownloadFileName(contentDisposition string, parsed *url.URL) (string, bool) {
	if contentDisposition != "" {
		if _, params, err := mime.ParseMediaType(contentDisposition); err == nil {
			if name := sanitizeDownloadFileName(params["filename"]); name != "" && hasSupportedInstallExtension(name) {
				return name, true
			}
		}
	}

	if name := sanitizeDownloadFileName(path.Base(parsed.Path)); name != "" && hasSupportedInstallExtension(name) {
		return name, true
	}

	return "", false
}

// Reports whether fileName has one of the archive or bundle extensions
// Cratebug already knows how to stage, the same set SelectFilesForInstall's
// file picker offers.
func hasSupportedInstallExtension(fileName string) bool {
	return IsArchiveFile(fileName) || hasBundleExtension(fileName)
}

// Strips path separators from a server- or URL-provided name so it can
// never be used to escape the temporary download directory it's joined
// into.
func sanitizeDownloadFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" || name == "\\" {
		return ""
	}
	return filepath.Base(filepath.FromSlash(name))
}
