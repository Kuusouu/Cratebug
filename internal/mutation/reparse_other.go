//go:build !windows

package mutation

func verifyNotReparsePoint(path, label string) error {
	return nil
}
