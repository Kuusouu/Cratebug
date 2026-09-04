//go:build !windows

package main

func primaryWorkArea() (int, int) {
	return 0, 0
}
