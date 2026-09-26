//go:build linux

package urlscheme

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// Directory permissions for application and configuration directories (rwxr-xr-x).
	standardDirMode os.FileMode = 0o755

	// File permissions for desktop entries and configuration files (rw-r--r--).
	standardFileMode os.FileMode = 0o644
)

var (
	applicationsDirOverride string
	configDirOverride       string
	xdgMimeCommand          = exec.Command
)

func defaultApplicationsDir() (string, error) {
	if applicationsDirOverride != "" {
		return applicationsDirOverride, nil
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "applications"), nil
}

func defaultConfigDir() (string, error) {
	if configDirOverride != "" {
		return configDirOverride, nil
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return configHome, nil
}

type userHive struct{}

func (userHive) read(scheme string) (Snapshot, bool, error) {
	mimeType := "x-scheme-handler/" + scheme

	// 1. Try xdg-mime query default.
	var desktopFileName string
	cmd := xdgMimeCommand("xdg-mime", "query", "default", mimeType)
	if out, err := cmd.Output(); err == nil {
		desktopFileName = strings.TrimSpace(string(out))
	}

	// 2. If xdg-mime was empty or unavailable, inspect ~/.config/mimeapps.list.
	if desktopFileName == "" {
		if configDir, err := defaultConfigDir(); err == nil {
			mimeappsPath := filepath.Join(configDir, "mimeapps.list")
			desktopFileName = readMimeDefault(mimeappsPath, mimeType)
		}
	}

	if desktopFileName == "" {
		return Snapshot{}, false, nil
	}
	if !validDesktopFileName(desktopFileName) {
		return Snapshot{}, false, fmt.Errorf("invalid desktop file name %q", desktopFileName)
	}

	// 3. Locate the desktop file across standard FreeDesktop application locations.
	desktopFilePath, err := findDesktopFile(desktopFileName)
	if err != nil {
		return Snapshot{}, false, err
	}
	if desktopFilePath == "" {
		return Snapshot{DesktopFile: desktopFileName}, true, nil
	}

	// 4. Parse the Exec line from the desktop file.
	content, err := os.ReadFile(desktopFilePath)
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("read desktop file %q: %w", desktopFilePath, err)
	}

	execLine := parseDesktopExec(string(content))
	if execLine == "" {
		return Snapshot{DesktopFile: desktopFileName}, true, nil
	}

	snapshot := Snapshot{
		Command:     execLine,
		DesktopFile: desktopFileName,
	}
	return snapshot, true, nil
}

func (userHive) write(scheme string, snapshot Snapshot) error {
	desktopFileName := snapshot.DesktopFile
	if desktopFileName != "" {
		if !validDesktopFileName(desktopFileName) {
			return fmt.Errorf("invalid desktop file name %q", desktopFileName)
		}
		path, err := findDesktopFile(desktopFileName)
		if err != nil {
			return err
		}
		if path == "" {
			return fmt.Errorf("desktop file %q is no longer installed", desktopFileName)
		}
	} else {
		exe := commandExecutablePath(snapshot.Command)
		if exe == "" {
			return errors.New("urlscheme: empty command executable")
		}
		appDir, err := defaultApplicationsDir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(appDir, standardDirMode); err != nil {
			return fmt.Errorf("create applications directory: %w", err)
		}
		desktopFileName = fmt.Sprintf("cratebug-%s.desktop", scheme)
		desktopFilePath := filepath.Join(appDir, desktopFileName)
		desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Cratebug %s Handler
Exec="%s" %%u
Terminal=false
NoDisplay=true
MimeType=x-scheme-handler/%s;
`, strings.ToUpper(scheme), exe, scheme)
		if err := os.WriteFile(desktopFilePath, []byte(desktopContent), standardFileMode); err != nil {
			return fmt.Errorf("write desktop entry %q: %w", desktopFilePath, err)
		}
	}

	mimeType := "x-scheme-handler/" + scheme
	cmd := xdgMimeCommand("xdg-mime", "default", desktopFileName, mimeType)
	_ = cmd.Run()

	configDir, err := defaultConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, standardDirMode); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	mimeappsPath := filepath.Join(configDir, "mimeapps.list")
	if err := updateMimeappsList(mimeappsPath, mimeType, desktopFileName, true); err != nil {
		return fmt.Errorf("set default handler in %q: %w", mimeappsPath, err)
	}

	return nil
}

func (userHive) delete(scheme string) error {
	mimeType := "x-scheme-handler/" + scheme
	desktopFileName := fmt.Sprintf("cratebug-%s.desktop", scheme)

	if configDir, err := defaultConfigDir(); err == nil {
		mimeappsPath := filepath.Join(configDir, "mimeapps.list")
		if _, err := os.Stat(mimeappsPath); err == nil {
			if err := updateMimeappsList(mimeappsPath, mimeType, desktopFileName, false); err != nil {
				return fmt.Errorf("remove default handler from %q: %w", mimeappsPath, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check default handler in %q: %w", mimeappsPath, err)
		}
	} else {
		return err
	}

	appDir, err := defaultApplicationsDir()
	if err != nil {
		return err
	}
	desktopPath := filepath.Join(appDir, desktopFileName)
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove desktop file %q: %w", desktopPath, err)
	}

	return nil
}

func (userHive) exists(scheme string) (bool, error) {
	_, exists, err := userHive{}.read(scheme)
	return exists, err
}

type machineHive struct{}

func (machineHive) read(string) (Snapshot, bool, error) { return Snapshot{}, false, nil }
func (machineHive) write(string, Snapshot) error {
	return errors.New("urlscheme: refusing to write machine-wide on Linux")
}
func (machineHive) delete(string) error {
	return errors.New("urlscheme: refusing to delete machine-wide on Linux")
}
func (machineHive) exists(string) (bool, error) { return false, nil }

func readUserChoice(string) (bool, error) { return false, nil }

func notifyAssocChanged() {
	if appDir, err := defaultApplicationsDir(); err == nil {
		if path, err := exec.LookPath("update-desktop-database"); err == nil {
			_ = exec.Command(path, appDir).Run()
		}
	}
}

func parseDesktopExec(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Exec=") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Exec="))
		}
	}
	return ""
}

func validDesktopFileName(name string) bool {
	return name == filepath.Base(name) && strings.HasSuffix(name, ".desktop") && !strings.ContainsAny(name, "\\/\r\n")
}

func findDesktopFile(name string) (string, error) {
	var candidates []string
	if appDir, err := defaultApplicationsDir(); err == nil {
		candidates = append(candidates, filepath.Join(appDir, name))
	}
	if dataDirs := os.Getenv("XDG_DATA_DIRS"); dataDirs != "" {
		for _, dataDir := range strings.Split(dataDirs, ":") {
			if dataDir != "" {
				candidates = append(candidates, filepath.Join(dataDir, "applications", name))
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, ".local", "share", "applications", name),
			filepath.Join(home, ".local", "share", "flatpak", "exports", "share", "applications", name),
		)
	}
	candidates = append(candidates,
		filepath.Join("/usr", "share", "applications", name),
		filepath.Join("/usr", "local", "share", "applications", name),
		filepath.Join("/var", "lib", "flatpak", "exports", "share", "applications", name),
	)

	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", nil
}

func readMimeDefault(mimeappsPath, mimeType string) string {
	data, err := os.ReadFile(mimeappsPath)
	if err != nil {
		return ""
	}

	targetPrefix := mimeType + "="
	currentSection := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed
			continue
		}
		if currentSection == "[Default Applications]" || currentSection == "[Added Associations]" {
			if strings.HasPrefix(trimmed, targetPrefix) {
				val := strings.TrimPrefix(trimmed, targetPrefix)
				val = strings.Trim(val, ";")
				if i := strings.IndexByte(val, ';'); i >= 0 {
					val = val[:i]
				}
				return strings.TrimSpace(val)
			}
		}
	}
	return ""
}

func updateMimeappsList(mimeappsPath, mimeType, desktopFile string, add bool) error {
	existing, _ := os.ReadFile(mimeappsPath)
	lines := strings.Split(string(existing), "\n")

	var newLines []string
	targetPrefix := mimeType + "="
	foundInDefault := false
	foundInAdded := false
	currentSection := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = trimmed
			newLines = append(newLines, line)
			continue
		}

		if (currentSection == "[Default Applications]" || currentSection == "[Added Associations]") && strings.HasPrefix(trimmed, targetPrefix) {
			if !add {
				values := strings.Split(strings.TrimPrefix(trimmed, targetPrefix), ";")
				var remaining []string
				for _, value := range values {
					if value != "" && value != desktopFile {
						remaining = append(remaining, value)
					}
				}
				if len(remaining) > 0 {
					newLines = append(newLines, targetPrefix+strings.Join(remaining, ";")+";")
				}
				continue
			}
			newLines = append(newLines, fmt.Sprintf("%s=%s", mimeType, desktopFile))
			if currentSection == "[Default Applications]" {
				foundInDefault = true
			} else {
				foundInAdded = true
			}
			continue
		}

		if trimmed != "" || len(newLines) > 0 {
			newLines = append(newLines, line)
		}
	}

	if add {
		if !foundInDefault {
			newLines = appendSectionEntry(newLines, "[Default Applications]", fmt.Sprintf("%s=%s", mimeType, desktopFile))
		}
		if !foundInAdded {
			newLines = appendSectionEntry(newLines, "[Added Associations]", fmt.Sprintf("%s=%s", mimeType, desktopFile))
		}
	}

	var buf bytes.Buffer
	for _, l := range newLines {
		buf.WriteString(l)
		buf.WriteByte('\n')
	}
	return os.WriteFile(mimeappsPath, buf.Bytes(), standardFileMode)
}

func appendSectionEntry(lines []string, section, entry string) []string {
	sectionIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == section {
			sectionIndex = i
			break
		}
	}

	if sectionIndex >= 0 {
		// Insert right after the section header.
		result := make([]string, 0, len(lines)+1)
		result = append(result, lines[:sectionIndex+1]...)
		result = append(result, entry)
		result = append(result, lines[sectionIndex+1:]...)
		return result
	}

	// Append section and entry at end.
	result := append(lines, "", section, entry)
	return result
}
