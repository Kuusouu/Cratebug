//go:build linux

package reveal

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	dbusRunner = func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		return cmd.Run()
	}

	openRunner = func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		if err := cmd.Start(); err != nil {
			return err
		}
		go func() {
			_ = cmd.Wait()
		}()
		return nil
	}

	lookPath = exec.LookPath
)

// openPath opens the file manager on Linux using D-Bus FileManager1 when selecting an item,
// falling back to xdg-open.
func openPath(target Target) error {
	if target.SelectItem {
		fileURI := "file://" + target.Path

		// Try gdbus first.
		if gdbusPath, err := lookPath("gdbus"); err == nil && gdbusPath != "" {
			escapedURI := escapeGVariantString(fileURI)
			args := []string{
				"call", "--session",
				"--dest", "org.freedesktop.FileManager1",
				"--object-path", "/org/freedesktop/FileManager1",
				"--method", "org.freedesktop.FileManager1.ShowItems",
				fmt.Sprintf("['%s']", escapedURI),
				"",
			}
			if err := dbusRunner(gdbusPath, args...); err == nil {
				return nil
			}
		}

		// Try dbus-send next, only if the URI has no commas (dbus-send array parsing splits on commas).
		if !strings.Contains(fileURI, ",") {
			if dbusSendPath, err := lookPath("dbus-send"); err == nil && dbusSendPath != "" {
				args := []string{
					"--session",
					"--dest=org.freedesktop.FileManager1",
					"--type=method_call",
					"/org/freedesktop/FileManager1",
					"org.freedesktop.FileManager1.ShowItems",
					fmt.Sprintf("array:string:%s", fileURI),
					"string:",
				}
				if err := dbusRunner(dbusSendPath, args...); err == nil {
					return nil
				}
			}
		}

		// Fall back to opening the containing directory with xdg-open.
		parentDir := filepath.Dir(target.Path)
		if err := openRunner("xdg-open", parentDir); err != nil {
			return fmt.Errorf("open directory %q with xdg-open: %w", parentDir, err)
		}
		return nil
	}

	if err := openRunner("xdg-open", target.Path); err != nil {
		return fmt.Errorf("open path %q with xdg-open: %w", target.Path, err)
	}
	return nil
}

// Escapes backslashes and single quotes for GVariant string literals in gdbus arguments.
func escapeGVariantString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}
