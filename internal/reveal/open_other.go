//go:build !windows

package reveal

import "fmt"

func openPath(_ Target) error {
	return fmt.Errorf("opening File Explorer is not supported on this platform")
}
