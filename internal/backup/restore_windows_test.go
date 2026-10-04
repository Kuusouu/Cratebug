package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestApplyRollbackFailurePreservesRecoveryAndBlocksRetry(t *testing.T) {
	// Arrange
	source := t.TempDir()
	writeFile(t, filepath.Join(source, "A_P.pak"), []byte("A"))
	writeFile(t, filepath.Join(source, "B_P.pak"), []byte("B"))
	manager := NewSessionManager()
	preview, err := manager.Preview(context.Background(), makeBackup(t, source, nil))
	if err != nil {
		t.Fatal(err)
	}
	stagingDir := manager.sessions[preview.Token].stagingDir
	defer os.RemoveAll(stagingDir)
	root := filepath.Join(t.TempDir(), "library")
	writeFile(t, filepath.Join(root, "A_P.pak"), []byte("original A"))
	writeFile(t, filepath.Join(root, "Keep_P.pak"), []byte("original Keep"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var handle windows.Handle
	var lockErr error

	// Act
	result, applyErr := manager.Apply(ctx, root, "", preview.Token, func(p Progress) {
		if p.Current == 3 {
			path, err := windows.UTF16PtrFromString(filepath.Join(root, "A_P.pak"))
			if err != nil {
				lockErr = err
				cancel()
				return
			}
			// Without FILE_SHARE_DELETE, Windows refuses the rollback rename.
			handle, lockErr = windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
			cancel()
		}
	})
	if lockErr != nil {
		t.Fatal(lockErr)
	}
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	_, retryErr := manager.Apply(context.Background(), root, "", preview.Token, nil)

	// Assert
	if applyErr == nil || !strings.Contains(applyErr.Error(), "rollback also failed") || result.Cancelled {
		t.Fatalf("rollback failure = %+v, error = %v", result, applyErr)
	}
	if retryErr == nil || manager.CanRetry(preview.Token) {
		t.Fatal("an incomplete staged backup remained eligible for retry")
	}
	if got := readTree(t, root+".restore-park")["A_P.pak"]; got != "original A" {
		t.Errorf("parked original = %q, want original A", got)
	}
	if got := readTree(t, root)["Keep_P.pak"]; got != "original Keep" {
		t.Errorf("independent recovery = %q, want original Keep", got)
	}
	if got := readTree(t, root)["A_P.pak"]; got != "A" {
		t.Errorf("placed file after rejected retry = %q, want A", got)
	}
	if got := readTree(t, filepath.Join(stagingDir, "tree"))["B_P.pak"]; got != "B" {
		t.Errorf("remaining staged file = %q, want B", got)
	}
}
