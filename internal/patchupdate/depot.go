package patchupdate

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kuusouu/Cratebug/internal/usmap"
)

const (
	// The community depot that publishes a Marvel Rivals mappings file for
	// each game build. BentoMod reads the same listing.
	DefaultDepotListURL = "https://api.github.com/repos/SpaceDepot/rivals-depot/contents/usmap?ref=main"
	depotDownloadHost   = "raw.githubusercontent.com"

	maxListingBytes  = 1 << 20
	maxMappingsBytes = 32 << 20
	depotTimeout     = 60 * time.Second
)

// Mappings names look like
// 5.3.2-3984769+++depot_marvel+S10.5_release-Marvel.usmap; the number after
// the engine version is the game build.
var mappingsNamePattern = regexp.MustCompile(`^[0-9.]+-([0-9]+)\+[^/\\]*\.usmap$`)

// ErrMappingsNotPublished means the depot has no mappings for the game build
// yet, which usually lasts a few hours after a patch.
var ErrMappingsNotPublished = errors.New("mappings for this game build are not published yet")

// MappingsFile is one mappings file the depot lists.
type MappingsFile struct {
	Name        string
	Build       int
	SHA         string
	DownloadURL string
}

// Depot lists and caches mappings files.
type Depot struct {
	HTTP     *http.Client
	ListURL  string
	CacheDir string
}

// List returns the depot's mappings files, newest build first.
func (d Depot) List(ctx context.Context) ([]MappingsFile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.ListURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := d.client().Do(request)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	defer func() {
		_ = response.Body.Close() // The body is fully read or abandoned on error; close cannot change the result.
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list mappings: %s", response.Status)
	}

	var entries []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		SHA         string `json:"sha"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxListingBytes)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}

	var files []MappingsFile
	for _, entry := range entries {
		build, ok := mappingsBuild(entry.Name)
		if entry.Type != "file" || !ok || !validDownloadURL(entry.DownloadURL) || len(entry.SHA) != sha1.Size*2 {
			continue
		}
		files = append(files, MappingsFile{Name: entry.Name, Build: build, SHA: strings.ToLower(entry.SHA), DownloadURL: entry.DownloadURL})
	}
	sortNewestFirst(files)
	return files, nil
}

// Cached returns the mappings files already downloaded, newest build first.
// They were verified when they were saved.
func (d Depot) Cached() ([]MappingsFile, error) {
	entries, err := os.ReadDir(d.CacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []MappingsFile
	for _, entry := range entries {
		if build, ok := mappingsBuild(entry.Name()); ok && entry.Type().IsRegular() {
			files = append(files, MappingsFile{Name: entry.Name(), Build: build})
		}
	}
	sortNewestFirst(files)
	return files, nil
}

// Fetch returns the cached path of file, downloading it first when needed.
// A download is kept only when its Git blob hash matches the listing.
func (d Depot) Fetch(ctx context.Context, file MappingsFile) (string, error) {
	if _, ok := mappingsBuild(file.Name); !ok {
		return "", fmt.Errorf("fetch mappings: unexpected name %q", file.Name)
	}
	path := filepath.Join(d.CacheDir, file.Name)
	if existing, err := os.ReadFile(path); err == nil && (file.SHA == "" || gitBlobSHA(existing) == file.SHA) {
		return path, nil
	}
	if !validDownloadURL(file.DownloadURL) {
		return "", fmt.Errorf("fetch mappings: unexpected download address for %s", file.Name)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, file.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	response, err := d.client().Do(request)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", file.Name, err)
	}
	defer func() {
		_ = response.Body.Close() // The body is fully read or abandoned on error; close cannot change the result.
	}()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", file.Name, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxMappingsBytes+1))
	if err != nil {
		return "", fmt.Errorf("download %s: %w", file.Name, err)
	}
	if len(data) > maxMappingsBytes {
		return "", fmt.Errorf("download %s: file is larger than %d bytes", file.Name, maxMappingsBytes)
	}
	if got := gitBlobSHA(data); got != file.SHA {
		return "", fmt.Errorf("download %s: content hash %s does not match the listing's %s", file.Name, got, file.SHA)
	}
	if _, err := usmap.Parse(data); err != nil {
		return "", fmt.Errorf("download %s: %w", file.Name, err)
	}

	if err := os.MkdirAll(d.CacheDir, 0o700); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(d.CacheDir, ".download-*")
	if err != nil {
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return "", err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(temp.Name())
		return "", err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		_ = os.Remove(temp.Name())
		return "", err
	}
	return path, nil
}

func (d Depot) client() *http.Client {
	if d.HTTP != nil {
		return d.HTTP
	}
	return &http.Client{Timeout: depotTimeout}
}

func mappingsBuild(name string) (int, bool) {
	match := mappingsNamePattern.FindStringSubmatch(name)
	if match == nil || filepath.Base(name) != name {
		return 0, false
	}
	build, err := strconv.Atoi(match[1])
	return build, err == nil
}

func validDownloadURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host == depotDownloadHost
}

func gitBlobSHA(data []byte) string {
	// Git names blobs by SHA-1, so this matches the hash the depot listing gives.
	hash := sha1.New()
	// hash.Hash writes never fail.
	hash.Write([]byte("blob " + strconv.Itoa(len(data)) + "\x00"))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

func sortNewestFirst(files []MappingsFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Build > files[j].Build })
}
