package secret

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testStore(t *testing.T) Store {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "nexus.key"), []byte("test-entropy"))
	store.protect = fakeProtect
	store.unprotect = fakeUnprotect
	return store
}

func fakeProtect(plaintext, entropy []byte) ([]byte, error) {
	out := make([]byte, len(plaintext))
	copy(out, plaintext)
	return out, nil
}

func fakeUnprotect(ciphertext, entropy []byte) ([]byte, error) {
	out := make([]byte, len(ciphertext))
	copy(out, ciphertext)
	return out, nil
}

func TestSetGetRoundTrip(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	if err := store.Set("api-key"); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get()

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if got != "api-key" {
		t.Errorf("Get() = %q, want %q", got, "api-key")
	}
}

func TestGetMissingFileIsNotConfigured(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	_, err := store.Get()

	// Assert
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Get() error = %v, want ErrNotConfigured", err)
	}
}

func TestConfiguredTracksFilePresence(t *testing.T) {
	// Arrange
	store := testStore(t)
	store.unprotect = func(ciphertext, entropy []byte) ([]byte, error) {
		t.Error("Configured() must not decrypt")
		return nil, errors.New("Configured must not decrypt")
	}

	// Act / Assert — existence is checked after each mutation
	if store.Configured() {
		t.Fatal("Configured() = true, want false when the file is missing")
	}

	store.protect = fakeProtect
	if err := store.Set("api-key"); err != nil {
		t.Fatal(err)
	}
	if !store.Configured() {
		t.Fatal("Configured() = false, want true after Set")
	}

	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if store.Configured() {
		t.Fatal("Configured() = true, want false after Clear")
	}
}

func TestClearAbsentFileIsNotAnError(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	err := store.Clear()

	// Assert
	if err != nil {
		t.Fatalf("Clear() = %v, want nil when the file is absent", err)
	}
}

func TestGetCorruptedCiphertextReturnsError(t *testing.T) {
	// Arrange
	store := testStore(t)
	if err := os.WriteFile(store.path, []byte("not-a-valid-blob"), secretFileMode); err != nil {
		t.Fatal(err)
	}
	store.unprotect = func(ciphertext, entropy []byte) ([]byte, error) {
		return nil, errors.New("unprotect failed")
	}

	// Act
	_, err := store.Get()

	// Assert
	if err == nil {
		t.Fatal("Get() succeeded for corrupted ciphertext, want an error")
	}
}

func TestSetRejectsEmptyValue(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	err := store.Set("")

	// Assert
	if err == nil {
		t.Fatal("Set(\"\") succeeded, want an error")
	}
	if store.Configured() {
		t.Fatal("Set(\"\") must not create a secret file")
	}
}

func TestSetLeavesNoTempFile(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	if err := store.Set("api-key"); err != nil {
		t.Fatal(err)
	}

	// Assert
	entries, err := os.ReadDir(filepath.Dir(store.path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		matched, err := filepath.Match("*.tmp-*", entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if matched {
			t.Errorf("Set left temporary file %q", entry.Name())
		}
	}
}

func TestSetWritesOwnerOnlyMode(t *testing.T) {
	// Arrange
	store := testStore(t)

	// Act
	if err := store.Set("api-key"); err != nil {
		t.Fatal(err)
	}

	// Assert
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows ACLs do not surface as Unix permission bits; CreateTemp still
	// requests 0600, but Mode().Perm() is not meaningful there.
	if runtime.GOOS == "windows" {
		return
	}
	if info.Mode().Perm() != secretFileMode {
		t.Errorf("file mode = %04o, want 0600", info.Mode().Perm())
	}
}
