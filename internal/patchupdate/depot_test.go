package patchupdate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	currentName = "5.3.2-3984769+++depot_marvel+S10.5_release-Marvel.usmap"
	olderName   = "5.3.2-3962284+++depot_marvel+S10.5_release-Marvel.usmap"
)

// Serves a depot listing and file downloads. Downloads arrive through the
// real host name so validDownloadURL accepts them; the transport reroutes.
type fakeDepot struct {
	files  map[string][]byte
	shas   map[string]string
	server *httptest.Server
}

func newFakeDepot(t *testing.T, files map[string][]byte) *fakeDepot {
	t.Helper()
	depot := &fakeDepot{files: files, shas: map[string]string{}}
	for name, data := range files {
		depot.shas[name] = gitBlobSHA(data)
	}
	depot.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/listing" {
			var entries []map[string]string
			for name := range depot.files {
				entries = append(entries, map[string]string{
					"name": name, "type": "file", "sha": depot.shas[name],
					"download_url": "https://" + depotDownloadHost + "/files/" + name,
				})
			}
			entries = append(entries, map[string]string{"name": "README.md", "type": "file", "sha": strings.Repeat("0", 40), "download_url": "https://" + depotDownloadHost + "/README.md"})
			_ = json.NewEncoder(w).Encode(entries)
			return
		}
		data, ok := depot.files[strings.TrimPrefix(r.URL.Path, "/files/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(depot.server.Close)
	return depot
}

func (f *fakeDepot) depot(cacheDir string) Depot {
	client := &http.Client{Transport: toServer{target: f.server.URL, base: http.DefaultTransport}}
	return Depot{HTTP: client, ListURL: f.server.URL + "/listing", CacheDir: cacheDir}
}

// Sends every request to the test server, keeping its path.
type toServer struct {
	target string
	base   http.RoundTripper
}

func (s toServer) RoundTrip(request *http.Request) (*http.Response, error) {
	target, err := url.Parse(s.target)
	if err != nil {
		return nil, err
	}
	clone := request.Clone(request.Context())
	clone.URL.Scheme, clone.URL.Host, clone.Host = target.Scheme, target.Host, target.Host
	return s.base.RoundTrip(clone)
}

func TestDepotListSkipsNonMappingsAndSortsNewestFirst(t *testing.T) {
	// Arrange
	fake := newFakeDepot(t, map[string][]byte{olderName: []byte("a"), currentName: []byte("b")})

	// Act
	files, err := fake.depot(t.TempDir()).List(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(files) != 2 || files[0].Build != 3984769 || files[1].Build != 3962284 {
		t.Fatalf("List() = %+v, want the two mappings newest first", files)
	}
}

func TestDepotFetchKeepsOnlyDownloadsMatchingTheListedHash(t *testing.T) {
	// Arrange
	fake := newFakeDepot(t, map[string][]byte{currentName: []byte("tampered")})
	depot := fake.depot(t.TempDir())
	files, err := depot.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	files[0].SHA = gitBlobSHA([]byte("what the listing promised"))

	// Act
	_, err = depot.Fetch(context.Background(), files[0])

	// Assert
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("Fetch() error = %v, want a hash mismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(depot.CacheDir, currentName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("a mismatched download was cached")
	}
}

func TestDepotFetchRejectsUnexpectedNamesAndHosts(t *testing.T) {
	// Arrange
	depot := Depot{CacheDir: t.TempDir()}
	escaping := MappingsFile{Name: "../" + currentName, DownloadURL: "https://" + depotDownloadHost + "/x"}
	elsewhere := MappingsFile{Name: currentName, DownloadURL: "https://example.com/" + currentName}

	// Act
	_, nameErr := depot.Fetch(context.Background(), escaping)
	_, hostErr := depot.Fetch(context.Background(), elsewhere)

	// Assert
	if nameErr == nil || hostErr == nil {
		t.Errorf("Fetch() errors = %v, %v; want both rejected", nameErr, hostErr)
	}
}

func TestGitBlobSHAMatchesGit(t *testing.T) {
	// Arrange
	content := []byte("hello\n")

	// Act
	sha := gitBlobSHA(content)

	// Assert: `printf 'hello\n' | git hash-object --stdin`
	if sha != "ce013625030ba8dba906f756967f9e9ca394464a" {
		t.Errorf("gitBlobSHA() = %s", sha)
	}
}

func TestGameBuildReadsTheNewestPatchContainer(t *testing.T) {
	// Arrange
	paks := t.TempDir()
	for _, name := range []string{"global.utoc", "Patch_-Windows_1.1.3962284_P.utoc", "Patch_-Windows_1.1.3984769_P.utoc", "Patch_-Windows_1.1.3984769_P.sig", "pakchunk0-Windows.utoc"} {
		if err := os.WriteFile(filepath.Join(paks, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mods := filepath.Join(paks, "~mods", "Hero")
	if err := os.MkdirAll(mods, 0o700); err != nil {
		t.Fatal(err)
	}

	// Act
	found, findErr := FindGamePaks(mods)
	build, buildErr := GameBuild(paks)
	_, emptyErr := GameBuild(mods)

	// Assert
	if findErr != nil || found != paks {
		t.Errorf("FindGamePaks() = %q, %v; want %q", found, findErr, paks)
	}
	if buildErr != nil || build != 3984769 {
		t.Errorf("GameBuild() = %d, %v; want 3984769", build, buildErr)
	}
	if !errors.Is(emptyErr, ErrGameBuildUnknown) {
		t.Errorf("GameBuild(no patches) error = %v, want ErrGameBuildUnknown", emptyErr)
	}
}

// The smallest valid mappings file: no names, enums, or structs.
func emptyMappings() []byte {
	return []byte{0xC4, 0x30, 4, 0, 0, 0, 0, 0, 12, 0, 0, 0, 12, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
}

func TestDepotFetchCachesAVerifiedDownloadAndReusesIt(t *testing.T) {
	// Arrange
	fake := newFakeDepot(t, map[string][]byte{currentName: emptyMappings()})
	cache := t.TempDir()
	depot := fake.depot(cache)
	files, err := depot.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Act
	path, err := depot.Fetch(context.Background(), files[0])
	fake.server.Close()
	again, againErr := depot.Fetch(context.Background(), files[0])

	// Assert
	if err != nil || path != filepath.Join(cache, currentName) {
		t.Fatalf("Fetch() = %q, %v", path, err)
	}
	if againErr != nil || again != path {
		t.Errorf("second Fetch() = %q, %v; want the cached file without the network", again, againErr)
	}
	cached, err := depot.Cached()
	if err != nil || len(cached) != 1 || cached[0].Build != 3984769 {
		t.Errorf("Cached() = %+v, %v", cached, err)
	}
}
