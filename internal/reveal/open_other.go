//go:build !windows && !linux

package reveal

import "fmt"

func openPath(_ Target) error {
	return fmt.Errorf("opening file manager is not supported on this platform")
}
