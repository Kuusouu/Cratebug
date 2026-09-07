package nexus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Kuusouu/Cratebug/internal/install"
)

const (
	remoteDownloadDirPrefix = "cratebug-nexus-download-"

	downloadChunkSize             = 32 * 1024
	downloadResponseHeaderTimeout = 30 * time.Second
	downloadReadIdleTimeout       = 30 * time.Second

	// Throttle progress events. A 32 KiB write callback on a 500 MB file
	// would otherwise emit tens of thousands of EventsEmit calls.
	downloadProgressInterval = 100 * time.Millisecond

	// size_kb is approximate. A factor of two either way is still the same
	// file; outside that the CDN response is not the file Nexus described.
	sizeMismatchFactor = 2

	// Marvel Rivals mods are hundreds of megabytes at most. Two gibibytes
	// is well above that and stops an unbounded stream if Content-Length
	// is missing or wrong.
	maxDownloadBytes int64 = 2 << 30
)

// Streams the file at link.URI into a temporary directory. link.URI must
// come from a Nexus download_link response, not a user-typed URL. The
// local name is FileInfo.file_name, checked before the request starts.
// cleanup removes the temporary directory; callers should defer it once
// the path has been handed to staging.
func Download(ctx context.Context, link DownloadLink, file FileInfo, httpClient *http.Client, onProgress func(install.Progress)) (downloadPath string, cleanup func(), err error) {
	return download(ctx, link, file, httpClient, onProgress, downloadReadIdleTimeout)
}

func download(ctx context.Context, link DownloadLink, file FileInfo, httpClient *http.Client, onProgress func(install.Progress), idleTimeout time.Duration) (downloadPath string, cleanup func(), err error) {
	rawURL := link.URI
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("nexus: %s is not a valid URL: %w", redactURL(rawURL), err)
	}
	if parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("nexus: download URL must be HTTPS, got %s", redactURL(rawURL))
	}

	fileName := sanitizeDownloadFileName(file.FileName)
	if fileName == "" || !install.IsSupportedInstallFile(fileName) {
		return "", nil, fmt.Errorf("nexus: %q is not a supported archive or bundle name", file.FileName)
	}

	expected := expectedDownloadBytes(file)
	if expected > maxDownloadBytes {
		return "", nil, fmt.Errorf("nexus: file %q is larger than the %d byte download limit", fileName, maxDownloadBytes)
	}

	httpClient = downloadHTTPClient(httpClient)

	// The idle timer interrupts a blocked body read by cancelling this
	// download's context. stalled distinguishes that from the caller
	// cancelling, so a stall reports as its own failure.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("nexus: build download request: %w", err)
	}
	req.Header.Set("User-Agent", "Cratebug")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, wrapDownloadError(rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("nexus: download %s returned %s", redactURL(rawURL), resp.Status)
	}

	if err := checkDownloadSize(resp.ContentLength, expected); err != nil {
		return "", nil, err
	}

	// Armed only once the body starts: dial, TLS, and header latency have
	// their own transport bound, and together they can legitimately exceed
	// the idle timeout.
	idle := time.AfterFunc(idleTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	destDir, err := os.MkdirTemp("", remoteDownloadDirPrefix)
	if err != nil {
		return "", nil, fmt.Errorf("nexus: create temporary download directory: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(destDir) }

	destPath := filepath.Join(destDir, fileName)
	out, err := os.Create(destPath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("nexus: create downloaded file %q: %w", destPath, err)
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

	var written int64
	buf := make([]byte, downloadChunkSize)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			idle.Reset(idleTimeout)
			written += int64(n)
			if written > maxDownloadBytes {
				cleanup()
				return "", nil, fmt.Errorf("nexus: download %s exceeded the %d byte limit", redactURL(rawURL), maxDownloadBytes)
			}
			if _, writeErr := writer.Write(buf[:n]); writeErr != nil {
				cleanup()
				return "", nil, fmt.Errorf("nexus: write downloaded file %q: %w", destPath, writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanup()
			if stalled.Load() {
				return "", nil, fmt.Errorf("nexus: download %s stalled: no bytes received for %s", redactURL(rawURL), idleTimeout)
			}
			return "", nil, wrapDownloadError(rawURL, readErr)
		}
	}

	return destPath, cleanup, nil
}

func downloadHTTPClient(httpClient *http.Client) *http.Client {
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = downloadResponseHeaderTimeout
		return &http.Client{Transport: transport, CheckRedirect: httpsRedirectsOnly}
	}
	clone := *httpClient
	clone.CheckRedirect = httpsRedirectsOnly
	return &clone
}

func httpsRedirectsOnly(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("nexus: too many redirects")
	}
	if req.URL.Scheme != "https" {
		return fmt.Errorf("nexus: refusing non-HTTPS redirect to %s", redactURL(req.URL.String()))
	}
	return nil
}

func expectedDownloadBytes(file FileInfo) int64 {
	if file.Size > 0 {
		return file.Size
	}
	if file.SizeKB > 0 {
		return int64(file.SizeKB) * 1024
	}
	return 0
}

func checkDownloadSize(contentLength, expected int64) error {
	if contentLength > maxDownloadBytes {
		return fmt.Errorf("nexus: response is larger than the %d byte download limit", maxDownloadBytes)
	}
	if contentLength > 0 && expected > 0 {
		if contentLength > expected*sizeMismatchFactor || contentLength*sizeMismatchFactor < expected {
			return fmt.Errorf("nexus: response size %d does not match expected %d bytes", contentLength, expected)
		}
	}
	return nil
}

func wrapDownloadError(rawURL string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = redactURL(urlErr.URL)
	}
	msg := err.Error()
	if strings.Contains(msg, "key=") {
		return fmt.Errorf("nexus: download %s: %s", redactURL(rawURL), "<redacted>")
	}
	return fmt.Errorf("nexus: download %s: %w", redactURL(rawURL), err)
}

type progressWriter struct {
	w          io.Writer
	total      int64
	written    int64
	lastEmit   time.Time
	onProgress func(install.Progress)
}

func (p *progressWriter) Write(chunk []byte) (int, error) {
	n, err := p.w.Write(chunk)
	p.written += int64(n)

	percent := 0.0
	if p.total > 0 {
		percent = float64(p.written) / float64(p.total) * 100.0
	}
	now := time.Now()
	final := p.total > 0 && p.written >= p.total
	if p.lastEmit.IsZero() || final || now.Sub(p.lastEmit) >= downloadProgressInterval {
		p.lastEmit = now
		p.onProgress(install.Progress{
			Phase:   "downloading",
			Current: 1,
			Total:   1,
			Message: "Downloading from Nexus Mods...",
			Percent: percent,
		})
	}
	return n, err
}

func sanitizeDownloadFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" || name == "\\" {
		return ""
	}
	return filepath.Base(filepath.FromSlash(name))
}
