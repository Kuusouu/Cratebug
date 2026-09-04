package main

import "testing"

func TestPrimaryWorkAreaIsUsableOnThisMachine(t *testing.T) {
	// Arrange / Act
	workWidth, workHeight := primaryWorkArea()

	// Assert
	if workWidth <= 0 || workHeight <= 0 {
		t.Fatalf("primaryWorkArea() = %d x %d, want a positive work area", workWidth, workHeight)
	}

	got := fitWindowSize(workWidth, workHeight)
	if got.width > workWidth || got.height > workHeight {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, which exceeds the work area", workWidth, workHeight, got)
	}
	t.Logf("work area %dx%d, fitted window %dx%d (min %dx%d)", workWidth, workHeight, got.width, got.height, got.minWidth, got.minHeight)
}
