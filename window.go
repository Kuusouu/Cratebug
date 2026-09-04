package main

const (
	preferredWindowWidth  = 1400
	preferredWindowHeight = 950
	minWindowWidth        = 1000
	minWindowHeight       = 650
)

type windowSize struct {
	width     int
	height    int
	minWidth  int
	minHeight int
}

func defaultWindowSize() windowSize {
	return fitWindowSize(primaryWorkArea())
}

// Clamps the preferred 1400x950 window to the primary work area. On a 1080p
// display at 125% scale the logical work area is about 1536x824, so the
// hardcoded 950px height does not fit until the window is maximized. A work
// dimension of 0 means the query failed and the preferred size is kept.
func fitWindowSize(workWidth, workHeight int) windowSize {
	size := windowSize{
		width:     preferredWindowWidth,
		height:    preferredWindowHeight,
		minWidth:  minWindowWidth,
		minHeight: minWindowHeight,
	}

	if workWidth > 0 {
		size.width = min(size.width, workWidth)
		size.minWidth = min(size.minWidth, workWidth)
		if size.width < size.minWidth {
			size.width = size.minWidth
		}
	}

	if workHeight > 0 {
		size.height = min(size.height, workHeight)
		size.minHeight = min(size.minHeight, workHeight)
		if size.height < size.minHeight {
			size.height = size.minHeight
		}
	}

	return size
}
