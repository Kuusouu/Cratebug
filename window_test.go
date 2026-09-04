package main

import "testing"

func TestFitWindowSizeKeepsThePreferredSizeWhenTheWorkAreaIsLarger(t *testing.T) {
	// Arrange
	const workWidth = 1920
	const workHeight = 1040

	// Act
	got := fitWindowSize(workWidth, workHeight)

	// Assert
	want := windowSize{
		width:     preferredWindowWidth,
		height:    preferredWindowHeight,
		minWidth:  minWindowWidth,
		minHeight: minWindowHeight,
	}
	if got != want {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, want %+v", workWidth, workHeight, got, want)
	}
}

func TestFitWindowSizeClampsHeightOn1080pAt125Percent(t *testing.T) {
	// Arrange
	// 1920x1080 at 125% scale, 48 physical-pixel taskbar: logical work area
	// is 1536 x 825. Width 1400 still fits; height 950 does not.
	const workWidth = 1536
	const workHeight = 825

	// Act
	got := fitWindowSize(workWidth, workHeight)

	// Assert
	want := windowSize{
		width:     preferredWindowWidth,
		height:    workHeight,
		minWidth:  minWindowWidth,
		minHeight: minWindowHeight,
	}
	if got != want {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, want %+v", workWidth, workHeight, got, want)
	}
}

func TestFitWindowSizeClampsWidthAndHeightOnASmallLaptop(t *testing.T) {
	// Arrange
	const workWidth = 1366
	const workHeight = 728

	// Act
	got := fitWindowSize(workWidth, workHeight)

	// Assert
	want := windowSize{
		width:     workWidth,
		height:    workHeight,
		minWidth:  minWindowWidth,
		minHeight: minWindowHeight,
	}
	if got != want {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, want %+v", workWidth, workHeight, got, want)
	}
}

func TestFitWindowSizeLowersTheMinimumWhenTheWorkAreaIsSmaller(t *testing.T) {
	// Arrange
	const workWidth = 800
	const workHeight = 600

	// Act
	got := fitWindowSize(workWidth, workHeight)

	// Assert
	want := windowSize{
		width:     workWidth,
		height:    workHeight,
		minWidth:  workWidth,
		minHeight: workHeight,
	}
	if got != want {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, want %+v", workWidth, workHeight, got, want)
	}
}

func TestFitWindowSizeKeepsThePreferredSizeWhenTheWorkAreaIsUnknown(t *testing.T) {
	// Arrange
	const workWidth = 0
	const workHeight = 0

	// Act
	got := fitWindowSize(workWidth, workHeight)

	// Assert
	want := windowSize{
		width:     preferredWindowWidth,
		height:    preferredWindowHeight,
		minWidth:  minWindowWidth,
		minHeight: minWindowHeight,
	}
	if got != want {
		t.Fatalf("fitWindowSize(%d, %d) = %+v, want %+v", workWidth, workHeight, got, want)
	}
}
