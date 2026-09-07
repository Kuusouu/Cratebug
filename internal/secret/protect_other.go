//go:build !windows

package secret

import "errors"

func protect(plaintext, entropy []byte) ([]byte, error) {
	return nil, errors.New("secret: DPAPI is only available on Windows")
}

func unprotect(ciphertext, entropy []byte) ([]byte, error) {
	return nil, errors.New("secret: DPAPI is only available on Windows")
}
