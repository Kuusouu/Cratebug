package main

import "testing"

func TestDeliberateCIFailure(t *testing.T) {
	t.Fatal("deliberate task 0.5 CI failure")
}
