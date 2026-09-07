package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestParseLaunchArgs(t *testing.T) {
	const nxm = "nxm://marvelrivals/mods/1/files/2"
	tests := []struct {
		name        string
		args        []string
		wantURL     string
		wantCleanup bool
	}{
		{name: "empty"},
		{name: "cleanup flag", args: []string{uninstallCleanupFlag}, wantCleanup: true},
		{name: "nxm url", args: []string{nxm}, wantURL: nxm},
		{name: "windows path then nxm", args: []string{`C:\Mods\file.pak`, nxm}, wantURL: nxm},
		{name: "cleanup wins over nxm", args: []string{uninstallCleanupFlag, nxm}, wantCleanup: true},
		{name: "unrelated flag", args: []string{"--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Act
			gotURL, gotCleanup := parseLaunchArgs(test.args)

			// Assert
			if gotURL != test.wantURL {
				t.Errorf("url = %q, want %q", gotURL, test.wantURL)
			}
			if gotCleanup != test.wantCleanup {
				t.Errorf("cleanup = %v, want %v", gotCleanup, test.wantCleanup)
			}
		})
	}
}

func TestOnSecondInstanceLaunchBuffersWhenContextIsNotReady(t *testing.T) {
	// Arrange
	app := testApp(t, false)
	if app.ctxReady {
		t.Fatal("ctxReady = true on a fresh test app, want false")
	}

	// Act
	app.onSecondInstanceLaunch(options.SecondInstanceData{
		Args: []string{`nxm://marvelrivals/mods/9/files/8?key=SENTINEL&expires=SENTINEL`},
	})

	// Assert
	link, err := app.TakePendingNexusLink()
	if err != nil {
		t.Fatalf("TakePendingNexusLink() = %v", err)
	}
	if !link.Present || link.ModID != 9 || link.FileID != 8 {
		t.Fatalf("pending link = %+v, want mod 9 file 8", link)
	}
	payload, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "SENTINEL") {
		t.Fatalf("second-instance payload %s contains SENTINEL", payload)
	}
}
