package update

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
)

const (
	// Number of times Download retries a transient (429/5xx) failure before giving up.
	maxDownloadAttempts = 3

	// Size of each chunk read from the response body while streaming a
	// download to disk.
	downloadChunkSize = 32 * 1024

	// Base unit for the retry backoff: attempt N waits N of these before retrying.
	downloadRetryBackoffUnit = time.Second

	// Bounds server response headers on a download attempt: a server that
	// accepts the connection but never responds fails the attempt after this
	// long instead of hanging. Dial and TLS setup keep the transport's own
	// defaults.
	downloadResponseHeaderTimeout = 30 * time.Second

	// Bounds a stalled body rather than the download as a whole: an attempt
	// is cancelled only after this long with no new bytes. There is
	// deliberately no overall attempt timeout -- a slow but progressing
	// multi-megabyte installer must be allowed to finish, so only inactivity
	// counts as failure.
	downloadReadIdleTimeout = 30 * time.Second
)

// Returned when an attempt made no progress for downloadReadIdleTimeout, so
// isRetryable can classify it as a transient network failure even though the
// underlying interruption is the context cancellation used to unblock the
// read.
var errDownloadStalled = errors.New("update: download stalled: no bytes received")

// Builds a client for installer downloads. It deliberately does not reuse
// httpClient()'s overall Timeout: that limit bounds small release-metadata
// requests, and applying it to a multi-megabyte body fails any download
// slower than size/15s no matter how many times Download retries. Liveness
// is bounded instead by the transport's response-header timeout plus
// downloadOnce's read-idle check, so a slow-but-progressing body is allowed.
func (c *Client) downloadHTTPClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = downloadResponseHeaderTimeout
	return &http.Client{Transport: transport}
}

// Carries a non-2xx HTTP response status so isRetryable can distinguish a
// transient server failure from a permanent one.
type httpStatusError struct {
	statusCode int
	status     string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("server returned %s", e.status)
}

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, errDownloadStalled) {
		return true
	}
	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.statusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}
	// A network-level failure (timeout, connection reset, DNS) is worth retrying.
	return true
}

// Checks that a release asset URL actually points at a GitHub release
// download for this repo, rather than trusting whatever the API response
// said. A browser_download_url always has the fixed shape
// https://github.com/{owner}/{repo}/releases/download/{tag}/{filename}.
func (c *Client) ValidateAssetURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("update: asset URL %q is not a valid URL: %w", rawURL, err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("update: asset URL %q is not HTTPS", rawURL)
	}
	if parsed.Host != "github.com" {
		return fmt.Errorf("update: asset URL %q is not hosted on github.com", rawURL)
	}
	wantPrefix := fmt.Sprintf("/%s/%s/releases/download/", c.Owner, c.Repo)
	if !strings.HasPrefix(parsed.Path, wantPrefix) {
		return fmt.Errorf("update: asset URL %q does not point at a %s/%s release download", rawURL, c.Owner, c.Repo)
	}
	return nil
}

// Fetches a release asset into destDir, retrying transient (429, 5xx)
// failures with a short backoff. Returns the full path to the downloaded
// file on success.
//
// The file is written under a ".download" suffix and only renamed to its
// final name once fully and successfully written, so a failed or cancelled
// download never leaves something that looks like a finished, executable
// asset on disk.
//
// onProgress, if non-nil, is called after each chunk with bytes downloaded
// so far and the total from Content-Length (0 when the server didn't send
// one).
func (c *Client) Download(ctx context.Context, asset ReleaseAsset, destDir string, onProgress func(downloaded, total int64)) (string, error) {
	if err := c.ValidateAssetURL(asset.URL); err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("update: creating download directory %s: %w", destDir, err)
	}

	finalPath := filepath.Join(destDir, asset.Name)
	tempPath := finalPath + ".download"

	var lastErr error
	for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
		lastErr = c.downloadOnce(ctx, asset.URL, tempPath, downloadReadIdleTimeout, onProgress)
		if lastErr == nil {
			break
		}
		if !isRetryable(lastErr) || attempt == maxDownloadAttempts {
			break
		}
		select {
		case <-ctx.Done():
			_ = os.Remove(tempPath)
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt) * downloadRetryBackoffUnit):
		}
	}
	if lastErr != nil {
		_ = os.Remove(tempPath)
		return "", lastErr
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("update: finalizing downloaded file: %w", err)
	}
	return finalPath, nil
}

func (c *Client) downloadOnce(ctx context.Context, assetURL, tempPath string, idleTimeout time.Duration, onProgress func(downloaded, total int64)) error {
	// The idle timer interrupts a blocked body read by cancelling this
	// attempt's context. stalled distinguishes that from the caller
	// cancelling, so a stall stays retryable while a deliberate cancel does
	// not.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled atomic.Bool

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return fmt.Errorf("update: building download request: %w", err)
	}
	req.Header.Set("User-Agent", "Cratebug")

	resp, err := c.downloadHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("update: downloading %s: %w", assetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &httpStatusError{statusCode: resp.StatusCode, status: resp.Status}
	}

	// Armed only once the body starts: dial, TLS, and header latency have
	// their own transport bound, and together they can legitimately exceed
	// the idle timeout.
	idle := time.AfterFunc(idleTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	out, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("update: creating temp download file %s: %w", tempPath, err)
	}
	defer out.Close()

	// resp.ContentLength is -1, not 0, when the server didn't send a length
	// (e.g. a chunked response); normalize so callers can rely on "0 means
	// unknown" as documented on Download.
	total := resp.ContentLength
	if total < 0 {
		total = 0
	}
	var downloaded int64
	buf := make([]byte, downloadChunkSize)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			idle.Reset(idleTimeout)
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("update: writing downloaded data: %w", writeErr)
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if stalled.Load() {
				return fmt.Errorf("%w for %s after %d bytes", errDownloadStalled, idleTimeout, downloaded)
			}
			return fmt.Errorf("update: reading download stream: %w", readErr)
		}
	}
	return nil
}
