package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/metadata"
	"github.com/Kuusouu/Cratebug/internal/mutation"
)

func TestMetadataGateKeepsConcurrentWrites(t *testing.T) {
	// Arrange
	const writers = 16
	gate := newMetadataGate(testMetadataStore(t))
	var wg sync.WaitGroup
	errs := make(chan error, writers)

	// Act
	for i := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- gate.update(func(doc *metadata.Document) error {
				_, err := doc.CreateTag(fmt.Sprintf("tag-%02d", i))
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)

	// Assert
	for err := range errs {
		if err != nil {
			t.Fatalf("update() = %v, want nil", err)
		}
	}
	if got := len(gate.read().Tags); got != writers {
		t.Fatalf("persisted %d tags, want %d: concurrent writes were lost", got, writers)
	}
}

func TestMetadataGateUnchangedSkipsSave(t *testing.T) {
	// Arrange
	gate := newMetadataGate(failingMetadataStore(t))

	// Act
	err := gate.update(func(*metadata.Document) error {
		return errMetadataUnchanged
	})

	// Assert
	if err != nil {
		t.Fatalf("update() = %v, want nil when nothing changed", err)
	}
}

func TestMetadataGateDoesNotSaveFailedChange(t *testing.T) {
	// Arrange
	gate := newMetadataGate(testMetadataStore(t))
	refused := errors.New("refused")

	// Act
	err := gate.update(func(doc *metadata.Document) error {
		doc.Settings.Theme = "dark"
		return refused
	})

	// Assert
	if !errors.Is(err, refused) {
		t.Fatalf("update() = %v, want %v", err, refused)
	}
	if theme := gate.read().Settings.Theme; theme != "" {
		t.Fatalf("theme = %q after a failed change, want it unsaved", theme)
	}
}

func TestRuntimeHostBeforeStartup(t *testing.T) {
	// Arrange
	host := &runtimeHost{}

	// Act
	host.emit("library:fs-changed")
	current := host.current()
	fallback := host.context()

	// Assert
	if current != nil {
		t.Fatalf("current() = %v before startup, want nil", current)
	}
	if fallback == nil {
		t.Fatal("context() = nil before startup, want a background context")
	}
}

func TestRuntimeHostReturnsStartupContext(t *testing.T) {
	// Arrange
	host := &runtimeHost{}
	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "startup")

	// Act
	host.set(ctx)

	// Assert
	if got := host.context().Value(key{}); got != "startup" {
		t.Fatalf("context() value = %v, want the startup context", got)
	}
}

func TestRequireGameNotRunning(t *testing.T) {
	tests := []struct {
		name    string
		game    mutation.GameRunningChecker
		wantErr error
	}{
		{name: "no checker", game: nil, wantErr: nil},
		{name: "not running", game: staticGameRunningChecker{gameRunning: false}, wantErr: nil},
		{name: "running", game: staticGameRunningChecker{gameRunning: true}, wantErr: mutation.ErrGameRunning},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Arrange
			c := &core{game: test.game}

			// Act
			err := c.requireGameNotRunning()

			// Assert
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("requireGameNotRunning() = %v, want %v", err, test.wantErr)
			}
		})
	}
}
