//go:build windows

package secret

import (
	"bytes"
	"testing"
)

func TestProtectRoundTrip(t *testing.T) {
	// Arrange
	plain := []byte("nexus-api-key")
	entropy := []byte("cratebug-entropy")

	// Act
	cipher, err := protect(plain, entropy)
	if err != nil {
		t.Fatal(err)
	}
	got, err := unprotect(cipher, entropy)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	if !bytes.Equal(got, plain) {
		t.Errorf("unprotect(protect(%q)) = %q, want %q", plain, got, plain)
	}
}

func TestUnprotectRejectsWrongEntropy(t *testing.T) {
	// Arrange
	cipher, err := protect([]byte("secret"), []byte("right"))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = unprotect(cipher, []byte("wrong"))

	// Assert
	if err == nil {
		t.Fatal("unprotect() succeeded with wrong entropy, want an error")
	}
}

func TestUnprotectRejectsTruncatedCiphertext(t *testing.T) {
	// Arrange
	cipher, err := protect([]byte("secret"), []byte("entropy"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cipher) < 2 {
		t.Fatal("protected blob is too small to truncate")
	}
	truncated := cipher[:len(cipher)/2]

	// Act
	_, err = unprotect(truncated, []byte("entropy"))

	// Assert
	if err == nil {
		t.Fatal("unprotect() succeeded with truncated ciphertext, want an error")
	}
}
