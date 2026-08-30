package modtype

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/discovery"
)

// Regression test for the crash reported when the pinned UAssetTool binary is
// missing: NewPinnedWorker returned a typed nil *Worker, the launcher wrapped
// it in the PoolWorker interface, and workerLoop called Alive()/Close() on it,
// panicking the process. The launcher below reproduces that exact shape: a
// typed nil concrete pointer wrapped in the interface, not an untyped nil.
func TestWorkerPoolSurvivesFailedLauncher(t *testing.T) {
	// Arrange
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Mod1_P.pak"), []byte("pak"), 0o600); err != nil {
		t.Fatal(err)
	}

	launcher := func() (PoolWorker, error) {
		return (*mockPoolWorker)(nil), errors.New("uassettool: worker executable not found")
	}

	pool := NewWorkerPool(1, launcher)
	defer pool.Close()

	results := make(chan classifyOutcome, 1)
	pool.Submit(classifyJob{
		root: root,
		entry: discovery.Entry{
			ID:           "mod-1",
			Kind:         discovery.EntryMod,
			BundleFormat: discovery.BundleFormatClassic,
			PrimaryPath:  "Mod1_P.pak",
		},
		table:  CharacterTable{},
		result: results,
	})

	// Act
	outcome := <-results

	// Assert
	if !outcome.failed {
		t.Errorf("outcome.failed = false, want true")
	}
	if outcome.identity.Category != CategoryUnknown {
		t.Errorf("outcome.identity.Category = %q, want %q", outcome.identity.Category, CategoryUnknown)
	}
}
