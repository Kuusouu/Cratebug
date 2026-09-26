//go:build linux

package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFilePermissionsRejectsWiderPermissions(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	securePath := filepath.Join(dir, "secure.key")
	if err := os.WriteFile(securePath, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	insecurePath := filepath.Join(dir, "insecure.key")
	if err := os.WriteFile(insecurePath, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	errSecure := checkFilePermissions(securePath)
	errInsecure := checkFilePermissions(insecurePath)

	// Assert
	if errSecure != nil {
		t.Errorf("checkFilePermissions(0600) unexpected error: %v", errSecure)
	}
	if errInsecure == nil {
		t.Errorf("checkFilePermissions(0644) expected error, got nil")
	}
}

func TestStoreUsesOwnerOnlyFileWhenSecretServiceUnavailable(t *testing.T) {
	// Arrange
	origLookPath := secretToolLookPath
	defer func() { secretToolLookPath = origLookPath }()
	secretToolLookPath = func(string) (string, error) {
		return "", errors.New("secret-tool not available")
	}

	keyPath := filepath.Join(t.TempDir(), "cratebug", "nexus.key")
	store := NewStore(keyPath, []byte("entropy-token"))
	key := "my-nexus-api-key-value"

	// Act
	err := store.Set(key)

	// Assert
	if err != nil {
		t.Fatalf("Store.Set() = %v", err)
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %04o, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != localPrefix+key {
		t.Errorf("key file does not have the local fallback format")
	}
	got, err := store.Get()
	if err != nil || got != key {
		t.Errorf("Store.Get() = %q, %v, want saved key", got, err)
	}
}

func TestUnprotectSupportsLegacyLocalPrefix(t *testing.T) {
	// Arrange
	plaintext := []byte("my-legacy-key")
	payload := append([]byte("cratebug-local-v1:"), plaintext...)
	entropy := []byte("entropy-token")

	// Act
	decrypted, err := unprotect(payload, entropy)

	// Assert
	if err != nil {
		t.Fatalf("unprotect() unexpected error: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestAESGCMEncryptionAndDecryption(t *testing.T) {
	// Arrange
	key := []byte("01234567890123456789012345678901") // 32 bytes
	plaintext := []byte("secret-token-payload")
	additionalData := []byte("nexus-entropy")

	// Act
	ciphertext, err := encryptAESGCM(key, plaintext, additionalData)
	if err != nil {
		t.Fatalf("encryptAESGCM unexpected error: %v", err)
	}

	decrypted, err := decryptAESGCM(key, ciphertext, additionalData)

	// Assert
	if err != nil {
		t.Fatalf("decryptAESGCM unexpected error: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestStoreGetRejectsInsecureFilePermissionsOnLinux(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "nexus.key")
	if err := os.WriteFile(keyPath, []byte("cratebug-local-v1:key-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(keyPath, nil)

	// Act
	_, err := store.Get()

	// Assert
	if err == nil {
		t.Fatal("store.Get() expected permission error for 0644 file, got nil")
	}
}
