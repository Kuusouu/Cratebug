//go:build windows

package urlscheme

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistrarWindowsIntegration(t *testing.T) {
	// Arrange
	scheme := "cratebug-test-" + randomHex(t, 8)
	if strings.EqualFold(scheme, SchemeNXM) {
		t.Fatal("refusing to touch the nxm scheme")
	}
	exe := filepath.Join(t.TempDir(), "Cratebug.exe")
	if err := os.WriteFile(exe, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := New(scheme, exe)
	t.Cleanup(func() {
		if err := r.user.delete(scheme); err != nil {
			t.Errorf("cleanup delete %s: %v", scheme, err)
		}
	})

	// Act
	status, err := r.Status()

	// Assert
	if err != nil {
		t.Fatalf("Status() = %v", err)
	}
	if status.Ownership != OwnershipNone {
		t.Fatalf("Ownership before register = %q, want none", status.Ownership)
	}

	// Act
	if _, err := r.Register(false); err != nil {
		t.Fatalf("Register() = %v", err)
	}

	// Assert
	status, err = r.Status()
	if err != nil {
		t.Fatalf("Status() after register = %v", err)
	}
	if status.Ownership != OwnershipSelf {
		t.Fatalf("Ownership after register = %q, want self", status.Ownership)
	}
	wantCommand := `"` + exe + `" "%1"`
	if status.Snapshot.Command != wantCommand {
		t.Errorf("Command = %q, want %q", status.Snapshot.Command, wantCommand)
	}

	// Act
	if err := r.Unregister(Snapshot{}); err != nil {
		t.Fatalf("Unregister() = %v", err)
	}

	// Assert
	status, err = r.Status()
	if err != nil {
		t.Fatalf("Status() after unregister = %v", err)
	}
	if status.Ownership != OwnershipNone {
		t.Fatalf("Ownership after unregister = %q, want none", status.Ownership)
	}
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(buf)
}
