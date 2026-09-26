//go:build !windows

package main

// Reports zero dimensions on non-Windows platforms.
// fitWindowSize falls back to preferred default dimensions when work dimensions are zero.
func primaryWorkArea() (int, int) {
	return 0, 0
}
