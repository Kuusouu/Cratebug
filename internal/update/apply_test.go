package update

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWriteApplyScriptRendersExpectedContent(t *testing.T) {
	exePath := `C:\Users\test\AppData\Local\Programs\Cratebug\Cratebug.exe`
	installerPath := `C:\Users\test\AppData\Local\Temp\cratebug-update\Cratebug-amd64-installer.exe`
	const pid = 4242

	scriptPath, err := writeApplyScript(exePath, installerPath, pid)
	if err != nil {
		t.Fatalf("writeApplyScript returned unexpected error: %v", err)
	}
	if filepath.Base(scriptPath) != "apply.bat" {
		t.Fatalf("scriptPath = %q, want a file named apply.bat", scriptPath)
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading rendered script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		`set "TARGET_PID=` + strconv.Itoa(pid) + `"`,
		`set "EXE_PATH=` + exePath + `"`,
		`set "INSTALLER_PATH=` + installerPath + `"`,
		`"%INSTALLER_PATH%" ` + nsisSilentInstallFlag,
		`start "" "%EXE_PATH%"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("rendered script missing expected content %q\nfull script:\n%s", want, script)
		}
	}
}

func TestWriteApplyScriptClearsPreviousStagingDirectory(t *testing.T) {
	dir := filepath.Join(os.TempDir(), applyStagingDirName)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("seeding staging directory: %v", err)
	}
	stalePath := filepath.Join(dir, "stale-installer.exe")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seeding stale file: %v", err)
	}

	if _, err := writeApplyScript("exe.exe", "installer.exe", 1); err != nil {
		t.Fatalf("writeApplyScript returned unexpected error: %v", err)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("writeApplyScript did not clear a stale file from a previous attempt")
	}
}
