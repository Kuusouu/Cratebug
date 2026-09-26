//go:build !windows && !linux

package mutation

import "fmt"

func recycleFiles(paths []string) error {
	return fmt.Errorf("mutation: recycle is not supported on this platform")
}
