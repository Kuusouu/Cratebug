//go:build !windows && !linux

package secret

import "errors"

func protect(plaintext, entropy []byte) ([]byte, error) {
	return nil, errors.New("secret: encryption is not supported on this platform")
}

func unprotect(ciphertext, entropy []byte) ([]byte, error) {
	return nil, errors.New("secret: decryption is not supported on this platform")
}

func checkFilePermissions(string) error {
	return nil
}
