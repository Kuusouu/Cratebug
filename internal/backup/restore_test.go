package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Kuusouu/Cratebug/internal/metadata"
)

// Stands in for a transient Windows file lock in retry tests.
var errTransientLock = errors.New("file is being used by another process")

const restoreFixtureMetadata = `{"schemaVersion":1,"settings":{},"mods":{"mod-1":{"scannerID":"mod::hero"}},"tags":[]}`

// Backs up root with metadataBytes and returns the zip path.
func makeBackup(t *testing.T, root string, metadataBytes []byte) string {
	t.Helper()
	var metadataPath string
	if metadataBytes != nil {
		metadataPath = filepath.Join(t.TempDir(), "metadata.json")
		writeFile(t, metadataPath, metadataBytes)
	}
	dest := filepath.Join(t.TempDir(), "backup.zip")
	if _, err := Create(context.Background(), root, metadataPath, dest, nil); err != nil {
		t.Fatal(err)
	}
	return dest
}

// Writes a raw zip archive from name-to-contents entries for cases Create
// cannot produce (missing metadata, corrupt metadata, traversal names).
func makeRawZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "backup.zip")
	out, err := os.Create(dest)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(out)
	for name, contents := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return dest
}

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[filepath.ToSlash(rel)] = string(contents)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestRestoreRoundTripReplacesLibrary(t *testing.T) {
	// Arrange.
	source := mixedLibrary(t)
	zipPath := makeBackup(t, source, []byte(restoreFixtureMetadata))
	library := t.TempDir()
	writeFile(t, filepath.Join(library, "Stale_9_P.pak"), []byte("stale"))
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	manager := NewSessionManager()

	// Act.
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Apply(context.Background(), library, metadataPath, preview.Token, nil)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts.Mods != 5 || result.Counts.Classic != 2 || result.Counts.IoStore != 1 || result.Counts.Invalid != 2 {
		t.Errorf("counts = %+v, want 5 mods (2 classic, 1 iostore, 2 invalid)", result.Counts)
	}
	if !result.MetadataRestored {
		t.Errorf("MetadataRestored = false, want true with note %q", result.MetadataNote)
	}
	if got, want := readTree(t, library), readTree(t, source); !reflect.DeepEqual(got, want) {
		t.Errorf("restored tree = %v, want %v", got, want)
	}
	// Compares normalized JSON: the store's omitempty fields make a strict
	// DeepEqual see a difference between an absent list and an empty one.
	var gotDoc, wantDoc metadata.Document
	contents, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(contents, &gotDoc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(restoreFixtureMetadata), &wantDoc); err != nil {
		t.Fatal(err)
	}
	wantDoc.SchemaVersion = metadata.CurrentSchemaVersion
	gotJSON, err := json.Marshal(gotDoc)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(wantDoc)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("restored metadata = %s, want %s", gotJSON, wantJSON)
	}
	if _, err := os.Stat(library + ".restore-park"); !os.IsNotExist(err) {
		t.Errorf("restore park was left behind")
	}
}

func TestPreviewReportsLiveCounts(t *testing.T) {
	// Arrange.
	zipPath := makeBackup(t, mixedLibrary(t), []byte(restoreFixtureMetadata))
	manager := NewSessionManager()

	// Act.
	preview, err := manager.Preview(context.Background(), zipPath)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if preview.Token == "" || !preview.MetadataPresent || preview.ZipModified == "" {
		t.Errorf("preview = %+v, want a token, metadata present, and a zip date", preview)
	}
	if preview.Counts.Mods != 5 {
		t.Errorf("preview counts = %+v, want 5 mods", preview.Counts)
	}
	manager.DiscardSession(preview.Token)
}

func TestRestoreWithoutMetadataKeepsCurrent(t *testing.T) {
	// Arrange.
	zipPath := makeRawZip(t, map[string]string{"Hero_1_P.pak": "hero"})
	library := t.TempDir()
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	writeFile(t, metadataPath, []byte(`{"schemaVersion":1,"settings":{"theme":"dark"}}`))
	manager := NewSessionManager()

	// Act.
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Apply(context.Background(), library, metadataPath, preview.Token, nil)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if preview.MetadataPresent || result.MetadataRestored {
		t.Errorf("preview = %+v, result = %+v, want no metadata handling", preview, result)
	}
	if result.MetadataNote == "" {
		t.Errorf("expected a note explaining why metadata was kept")
	}
	contents, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"schemaVersion":1,"settings":{"theme":"dark"}}` {
		t.Errorf("current metadata was modified: %s", contents)
	}
	if got := readTree(t, library); !reflect.DeepEqual(got, map[string]string{"Hero_1_P.pak": "hero"}) {
		t.Errorf("restored tree = %v, want the bare bundle", got)
	}
}

func TestRestoreWithCorruptMetadataKeepsCurrent(t *testing.T) {
	// Arrange.
	zipPath := makeRawZip(t, map[string]string{
		"Hero_1_P.pak":  "hero",
		"metadata.json": "not json at all",
	})
	library := t.TempDir()
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	writeFile(t, metadataPath, []byte(`{"schemaVersion":1}`))
	manager := NewSessionManager()

	// Act.
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := manager.Apply(context.Background(), library, metadataPath, preview.Token, nil)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if result.MetadataRestored || result.MetadataNote == "" {
		t.Errorf("result = %+v, want metadata kept with an explanatory note", result)
	}
	if got := readTree(t, library); !reflect.DeepEqual(got, map[string]string{"Hero_1_P.pak": "hero"}) {
		t.Errorf("restored tree = %v, want the bundle despite bad metadata", got)
	}
}

func TestRestoreRejectsTraversalArchive(t *testing.T) {
	// Arrange.
	zipPath := makeRawZip(t, map[string]string{"../evil.txt": "evil"})
	library := t.TempDir()
	before := readTree(t, library)
	manager := NewSessionManager()

	// Act.
	_, err := manager.Preview(context.Background(), zipPath)

	// Assert.
	if err == nil {
		t.Errorf("expected a rejection for the traversal entry")
	}
	if got := readTree(t, library); !reflect.DeepEqual(got, before) {
		t.Errorf("library changed despite the rejection: %v", got)
	}
}

func TestPreviewRejectsNonZip(t *testing.T) {
	// Arrange.
	notZip := filepath.Join(t.TempDir(), "backup.txt")
	writeFile(t, notZip, []byte("nope"))
	manager := NewSessionManager()

	// Act.
	_, err := manager.Preview(context.Background(), notZip)

	// Assert.
	if err == nil {
		t.Errorf("expected a rejection for a non-zip file")
	}
}

func TestApplyRejectsUnknownToken(t *testing.T) {
	// Arrange.
	manager := NewSessionManager()

	// Act.
	_, err := manager.Apply(context.Background(), t.TempDir(), "", "missing-token", nil)

	// Assert.
	if err == nil {
		t.Errorf("expected an error for an unknown session token")
	}
}

func TestDiscardSessionDropsStaging(t *testing.T) {
	// Arrange.
	zipPath := makeBackup(t, mixedLibrary(t), []byte(restoreFixtureMetadata))
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}

	// Act.
	manager.DiscardSession(preview.Token)

	// Assert.
	_, err = manager.Apply(context.Background(), t.TempDir(), "", preview.Token, nil)
	if err == nil {
		t.Errorf("expected the discarded session to be gone")
	}
}

func TestApplyRefusesLeftoverPark(t *testing.T) {
	// Arrange.
	zipPath := makeBackup(t, mixedLibrary(t), []byte(restoreFixtureMetadata))
	library := t.TempDir()
	if err := os.Mkdir(library+".restore-park", 0o700); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}

	// Act.
	_, err = manager.Apply(context.Background(), library, "", preview.Token, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected a refusal while a previous park exists")
	}
}

func TestApplyRollsBackOnMetadataWriteFailure(t *testing.T) {
	// Arrange.
	source := mixedLibrary(t)
	zipPath := makeBackup(t, source, []byte(restoreFixtureMetadata))
	library := t.TempDir()
	writeFile(t, filepath.Join(library, "Keep_1_P.pak"), []byte("keep"))
	// A directory where the metadata file belongs makes the safe write fail
	// after the tree was already replaced, exercising the rollback.
	metadataDir := t.TempDir()
	metadataPath := filepath.Join(metadataDir, "metadata.json")
	if err := os.Mkdir(metadataPath, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}

	// Act.
	_, err = manager.Apply(context.Background(), library, metadataPath, preview.Token, nil)

	// Assert.
	if err == nil {
		t.Errorf("expected the metadata write failure")
	}
	if got, want := readTree(t, library), map[string]string{"Keep_1_P.pak": "keep"}; !reflect.DeepEqual(got, want) {
		t.Errorf("library after rollback = %v, want %v", got, want)
	}
	if _, statErr := os.Stat(library + ".restore-park"); !os.IsNotExist(statErr) {
		t.Errorf("restore park was left behind")
	}
}

func TestRetryOperationRidesOutTransientFailures(t *testing.T) {
	// Arrange.
	calls := 0

	// Act.
	err := retryOperation(5, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errTransientLock
		}
		return nil
	})

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 attempts before success", calls)
	}
}

func TestRetryOperationSurfacesPersistentFailures(t *testing.T) {
	// Arrange.
	calls := 0

	// Act.
	err := retryOperation(3, time.Millisecond, func() error {
		calls++
		return errTransientLock
	})

	// Assert.
	if err == nil {
		t.Errorf("expected the persistent failure to surface")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want all 3 attempts used", calls)
	}
}

func TestApplyFailureKeepsSessionForRetry(t *testing.T) {
	// Arrange.
	source := mixedLibrary(t)
	zipPath := makeBackup(t, source, []byte(restoreFixtureMetadata))
	library := t.TempDir()
	writeFile(t, filepath.Join(library, "Keep_1_P.pak"), []byte("keep"))
	metadataDir := t.TempDir()
	blockedPath := filepath.Join(metadataDir, "metadata.json")
	if err := os.Mkdir(blockedPath, 0o700); err != nil {
		t.Fatal(err)
	}
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}

	// Act: first attempt fails on the metadata write and rolls back.
	_, err = manager.Apply(context.Background(), library, blockedPath, preview.Token, nil)
	if err == nil {
		t.Fatalf("expected the metadata write failure")
	}

	// Act: retry with a usable metadata path reuses the staged session.
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	result, err := manager.Apply(context.Background(), library, metadataPath, preview.Token, nil)

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts.Mods != 5 {
		t.Errorf("counts = %+v, want 5 retried mods", result.Counts)
	}
	if got, want := readTree(t, library), readTree(t, source); !reflect.DeepEqual(got, want) {
		t.Errorf("restored tree = %v, want %v", got, want)
	}
}

func TestApplyReportsProgress(t *testing.T) {
	// Arrange.
	zipPath := makeBackup(t, mixedLibrary(t), nil)
	library := t.TempDir()
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), zipPath)
	if err != nil {
		t.Fatal(err)
	}
	var progress []Progress

	// Act.
	_, err = manager.Apply(context.Background(), library, "", preview.Token, func(p Progress) {
		progress = append(progress, p)
	})

	// Assert.
	if err != nil {
		t.Fatal(err)
	}
	if len(progress) == 0 {
		t.Errorf("expected progress events during apply")
	}
	last := progress[len(progress)-1]
	if last.Current != last.Total {
		t.Errorf("final progress = %+v, want a completed total", last)
	}
}

func TestApplyOwnsSessionUntilItFinishes(t *testing.T) {
	// Arrange
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "A_P.pak"), []byte("A"))
	writeFile(t, filepath.Join(source, "B_P.pak"), []byte("B"))
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), makeBackup(t, source, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.DiscardSession(preview.Token)
	root, otherRoot := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(root, "Keep_P.pak"), []byte("original"))
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)

	// Act
	go func() {
		_, err := manager.Apply(context.Background(), root, "", preview.Token, func(p Progress) {
			if p.Current == 2 {
				close(entered)
				<-release
			}
		})
		done <- err
	}()
	<-entered
	_, secondErr := manager.Apply(context.Background(), otherRoot, "", preview.Token, nil)
	manager.DiscardSession(preview.Token)
	retryable := manager.CanRetry(preview.Token)
	close(release)
	firstErr := <-done

	// Assert
	if secondErr == nil || retryable {
		t.Errorf("active session accepted another apply or retry")
	}
	if firstErr != nil {
		t.Fatalf("active discard interrupted restore: %v", firstErr)
	}
	if got, want := readTree(t, root), readTree(t, source); !reflect.DeepEqual(got, want) {
		t.Errorf("restored tree = %v, want %v", got, want)
	}
	if len(readTree(t, otherRoot)) != 0 {
		t.Error("rejected apply changed the second library")
	}
}

func TestApplyCancellationRollsBackAndRetainsCompleteStage(t *testing.T) {
	// Arrange
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "A_P.pak"), []byte("A"))
	writeFile(t, filepath.Join(source, "B_P.pak"), []byte("B"))
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), makeBackup(t, source, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.DiscardSession(preview.Token)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Keep_P.pak"), []byte("original"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	result, err := manager.Apply(ctx, root, "", preview.Token, func(p Progress) {
		if p.Current == 2 {
			cancel()
		}
	})

	// Assert
	if err != nil || !result.Cancelled || !manager.CanRetry(preview.Token) {
		t.Fatalf("cancelled restore = %+v, error = %v", result, err)
	}
	if got, want := readTree(t, root), map[string]string{"Keep_P.pak": "original"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cancelled library = %v, want %v", got, want)
	}

	// Act
	result, err = manager.Apply(context.Background(), root, "", preview.Token, nil)

	// Assert
	if err != nil || result.Cancelled || result.Counts.Mods != 2 {
		t.Fatalf("retry result = %+v, error = %v", result, err)
	}
	if got, want := readTree(t, root), readTree(t, source); !reflect.DeepEqual(got, want) {
		t.Errorf("retried library = %v, want %v", got, want)
	}
}

func TestApplyLateCancellationReportsCompletedRestore(t *testing.T) {
	// Arrange
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "A_P.pak"), []byte("A"))
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), makeBackup(t, source, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer manager.DiscardSession(preview.Token)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Act
	result, err := manager.Apply(ctx, root, "", preview.Token, func(p Progress) {
		if p.Current == p.Total {
			cancel()
		}
	})

	// Assert
	if err != nil || result.Cancelled || result.Counts.Mods != 1 {
		t.Fatalf("completed restore = %+v, error = %v", result, err)
	}
	if manager.CanRetry(preview.Token) {
		t.Error("completed restore retained its session")
	}
}
