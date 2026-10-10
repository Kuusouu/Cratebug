package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestUninstallCleanupRequested(t *testing.T) {
	// Arrange
	const nxm = "nxm://marvelrivals/mods/1/files/2?key=SENTINEL&expires=1"
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "empty"},
		{name: "cleanup", args: []string{uninstallCleanupFlag}, want: true},
		{name: "legacy download ignored", args: []string{nxm}},
		{name: "cleanup with legacy download", args: []string{nxm, uninstallCleanupFlag}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Act
			got := uninstallCleanupRequested(test.args)

			// Assert
			if got != test.want {
				t.Errorf("uninstallCleanupRequested() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSecondInstanceBeforeStartupIgnoresLegacyDownload(t *testing.T) {
	// Arrange
	app := testApp(t, false)

	// Act: A legacy URL must not create a pending download or require a runtime.
	app.onSecondInstanceLaunch(options.SecondInstanceData{
		Args: []string{"nxm://marvelrivals/mods/9/files/8?key=SENTINEL&expires=1"},
	})

	// Assert
	if app.ctx != nil {
		t.Fatal("second-instance launch changed the runtime context")
	}
}
