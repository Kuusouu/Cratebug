// Package secret is a one-value store that encrypts at rest with DPAPI on Windows.
package secret

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Reports that no secret has been stored yet.
var ErrNotConfigured = errors.New("secret: not configured")

const (
	// Owner-only so another account on the same machine cannot read the blob
	// from the filesystem. DPAPI still binds decryption to this Windows user.
	secretFileMode os.FileMode = 0o600
	secretDirMode  os.FileMode = 0o700
)

// Holds one encrypted value at a filesystem path.
type Store struct {
	path    string
	entropy []byte
	// protect and unprotect are the DPAPI calls. Tests replace them so no test
	// depends on the machine's DPAPI master key.
	protect   func(plaintext, entropy []byte) ([]byte, error)
	unprotect func(ciphertext, entropy []byte) ([]byte, error)
}

// Binds a store to path and optional DPAPI entropy. Callers choose the path so
// tests can use disposable directories instead of the real per-user location.
func NewStore(path string, entropy []byte) Store {
	return Store{
		path:      path,
		entropy:   entropy,
		protect:   protect,
		unprotect: unprotect,
	}
}

// Binds a store that writes the value as-is. App-layer tests use this so they
// do not depend on the machine's DPAPI master key.
func NewPlainStore(path string, entropy []byte) Store {
	identity := func(value, _ []byte) ([]byte, error) {
		out := make([]byte, len(value))
		copy(out, value)
		return out, nil
	}
	return Store{
		path:      path,
		entropy:   entropy,
		protect:   identity,
		unprotect: identity,
	}
}

// Encrypts value and writes it atomically. An empty value is rejected so
// callers use Clear to remove a stored secret.
func (s Store) Set(value string) error {
	if value == "" {
		return errors.New("secret: empty value is rejected; use Clear")
	}

	ciphertext, err := s.protect([]byte(value), s.entropy)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, secretDirMode); err != nil {
		return fmt.Errorf("create secret directory: %w", err)
	}

	if err := writeFileAtomically(dir, s.path, ciphertext); err != nil {
		return fmt.Errorf("write secret file: %w", err)
	}
	return nil
}

// Decrypts and returns the stored value. A missing file is ErrNotConfigured.
func (s Store) Get() (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotConfigured
		}
		return "", fmt.Errorf("read secret file: %w", err)
	}

	plaintext, err := s.unprotect(data, s.entropy)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}
	return string(plaintext), nil
}

// Reports whether a secret file exists without decrypting it.
func (s Store) Configured() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Deletes the stored secret. A missing file is not an error.
func (s Store) Clear() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove secret file: %w", err)
	}
	return nil
}

// Returns the default per-user path for the Nexus API key blob.
func DefaultNexusKeyPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "Cratebug", "nexus.key"), nil
}

// Writes data to destination by creating a temporary file in dir and renaming
// it into place, so a failure partway through writing never leaves
// destination in a partially written state.
func writeFileAtomically(dir, destination string, data []byte) error {
	temp, err := os.CreateTemp(dir, filepath.Base(destination)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(secretFileMode); err != nil {
		temp.Close()
		return fmt.Errorf("set temporary file mode: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tempPath, destination); err != nil {
		return fmt.Errorf("rename temporary file into place: %w", err)
	}
	return nil
}
