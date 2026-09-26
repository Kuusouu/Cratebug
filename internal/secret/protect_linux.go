//go:build linux

package secret

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const (
	secretServicePrefix = "cratebug-secret-v1:"
	localPrefix         = "cratebug-local-v1:"

	// Length in bytes of the AES-256 master key stored in Secret Service.
	masterKeyLength = 32

	// Permission mask covering all group and other read/write/execute bits.
	insecurePermMask os.FileMode = 0o077

	// Owner-only read/write permission required for secret storage files.
	expectedSecretPerm os.FileMode = 0o600
)

var (
	secretToolLookPath = exec.LookPath
	secretToolCommand  = exec.Command
)

func checkFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check file permissions: %w", err)
	}

	// Reject if group or other have any read, write, or execute permissions.
	if info.Mode().Perm()&insecurePermMask != 0 {
		return fmt.Errorf("secret: file %q has insecure permissions %04o; expected %04o", path, info.Mode().Perm(), expectedSecretPerm)
	}
	return nil
}

func protect(plaintext, entropy []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, errors.New("secret: cannot protect empty plaintext")
	}

	masterKey, err := getOrCreateSecretServiceKey()
	if err != nil {
		// Store.Set writes this fallback in an owner-only file.
		return append([]byte(localPrefix), plaintext...), nil
	}
	if len(masterKey) != masterKeyLength {
		return nil, errors.New("secret: Secret Service returned invalid key length")
	}

	ciphertext, err := encryptAESGCM(masterKey, plaintext, entropy)
	if err != nil {
		return nil, fmt.Errorf("secret: encrypt: %w", err)
	}
	return append([]byte(secretServicePrefix), ciphertext...), nil
}

func unprotect(ciphertext, entropy []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("secret: cannot unprotect empty ciphertext")
	}

	if bytes.HasPrefix(ciphertext, []byte(secretServicePrefix)) {
		payload := bytes.TrimPrefix(ciphertext, []byte(secretServicePrefix))
		masterKey, err := getOrCreateSecretServiceKey()
		if err != nil {
			return nil, fmt.Errorf("retrieve Secret Service master key: %w", err)
		}
		return decryptAESGCM(masterKey, payload, entropy)
	}

	if bytes.HasPrefix(ciphertext, []byte(localPrefix)) {
		return bytes.TrimPrefix(ciphertext, []byte(localPrefix)), nil
	}

	// Legacy or unadorned plaintext.
	return ciphertext, nil
}

func getOrCreateSecretServiceKey() ([]byte, error) {
	secretTool, err := secretToolLookPath("secret-tool")
	if err != nil || secretTool == "" {
		return nil, errors.New("secret-tool not available")
	}

	// Try lookup.
	lookupCmd := secretToolCommand(secretTool, "lookup", "service", "cratebug", "key", "master-key")
	out, err := lookupCmd.Output()
	if err == nil {
		trimmed := strings.TrimSpace(string(out))
		if keyBytes, decErr := hex.DecodeString(trimmed); decErr == nil && len(keyBytes) == masterKeyLength {
			return keyBytes, nil
		}
	}

	// Generate a new 32-byte key.
	newKey := make([]byte, masterKeyLength)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		return nil, fmt.Errorf("generate random master key: %w", err)
	}
	hexKey := hex.EncodeToString(newKey)

	// Store in Secret Service.
	storeCmd := secretToolCommand(secretTool, "store", "--label=Cratebug Master Key", "service", "cratebug", "key", "master-key")
	storeCmd.Stdin = strings.NewReader(hexKey)
	if err := storeCmd.Run(); err != nil {
		return nil, fmt.Errorf("store key in Secret Service: %w", err)
	}

	return newKey, nil
}

func encryptAESGCM(key, plaintext, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, additionalData)
	return ciphertext, nil
}

func decryptAESGCM(key, ciphertext, additionalData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("secret: ciphertext too short")
	}
	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, sealed, additionalData)
}
