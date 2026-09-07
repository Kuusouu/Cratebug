//go:build windows

package secret

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

func protect(plaintext, entropy []byte) ([]byte, error) {
	// Taking &plain[0] panics when the slice is empty.
	if len(plaintext) == 0 {
		return nil, errors.New("secret: cannot protect empty plaintext")
	}

	in := windows.DataBlob{
		Size: uint32(len(plaintext)),
		Data: &plaintext[0],
	}
	var out windows.DataBlob
	// name is stored in cleartext inside the blob, so it stays nil. Flags are
	// user-scoped: LOCAL_MACHINE would let any account on this PC decrypt.
	err := windows.CryptProtectData(
		&in,
		nil,
		optionalEntropyBlob(entropy),
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&out,
	)
	runtime.KeepAlive(plaintext)
	runtime.KeepAlive(entropy)
	if err != nil {
		return nil, fmt.Errorf("protect secret: %w", err)
	}
	return copyAndFreeBlob(out)
}

func unprotect(ciphertext, entropy []byte) ([]byte, error) {
	// Taking &cipher[0] panics when the slice is empty.
	if len(ciphertext) == 0 {
		return nil, errors.New("secret: cannot unprotect empty ciphertext")
	}

	in := windows.DataBlob{
		Size: uint32(len(ciphertext)),
		Data: &ciphertext[0],
	}
	var out windows.DataBlob
	err := windows.CryptUnprotectData(
		&in,
		nil,
		optionalEntropyBlob(entropy),
		0,
		nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN,
		&out,
	)
	runtime.KeepAlive(ciphertext)
	runtime.KeepAlive(entropy)
	if err != nil {
		return nil, fmt.Errorf("unprotect secret: %w", err)
	}
	return copyAndFreeBlob(out)
}

func optionalEntropyBlob(entropy []byte) *windows.DataBlob {
	if len(entropy) == 0 {
		return nil
	}
	return &windows.DataBlob{
		Size: uint32(len(entropy)),
		Data: &entropy[0],
	}
}

func copyAndFreeBlob(out windows.DataBlob) ([]byte, error) {
	if out.Data == nil || out.Size == 0 {
		return nil, errors.New("secret: DPAPI returned an empty blob")
	}
	// Copy before LocalFree: out.Data is an OS-owned LocalAlloc buffer.
	copied := make([]byte, out.Size)
	copy(copied, unsafe.Slice(out.Data, int(out.Size)))
	if _, err := windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))); err != nil {
		return nil, fmt.Errorf("free DPAPI blob: %w", err)
	}
	return copied, nil
}
