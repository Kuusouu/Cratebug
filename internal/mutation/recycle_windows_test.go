//go:build windows

package mutation

import "testing"

// Confirms the double-null-terminated string layout required by SHFileOperationW,
// calling the user's real Recycle Bin.
func TestJoinShellPaths(t *testing.T) {
	// Arrange
	paths := []string{"one.pak", "two.utoc"}

	// Act
	got := joinShellPaths(paths)

	// Assert
	want := "one.pak\x00two.utoc\x00\x00"
	if got != want {
		t.Errorf("joinShellPaths() = %q, want %q", got, want)
	}
}
