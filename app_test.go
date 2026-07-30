package main

import "testing"

func TestRuntimeStatus(t *testing.T) {
	app := NewApp()

	const want = "Go backend connected"
	if got := app.RuntimeStatus(); got != want {
		t.Fatalf("RuntimeStatus() = %q, want %q", got, want)
	}
}
