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
		`"%SYS%\tasklist.exe"`,
		`"%SYS%\timeout.exe"`,
		`for /f "tokens=2 delims=," %%P in (%SNAPSHOT%)`,
		`/FO CSV /NH`,
		`if not "%FOUND_PID%"=="%TARGET_PID%" goto waitdone`,
		`if %WAITED% GEQ`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("rendered script missing expected content %q\nfull script:\n%s", want, script)
		}
	}

	// A bare `find`/`findstr` here can resolve to GNU findutils' find
	// instead of Windows' own tool on a machine with Git for Windows/MSYS
	// ahead of System32 on PATH.
	if strings.Contains(script, `| find `) || strings.Contains(script, `| find/`) || strings.Contains(script, `findstr`) {
		t.Errorf("rendered script pipes to find/findstr instead of parsing tasklist output directly:\n%s", script)
	}

	// The still-running check previously branched on a piped command's
	// errorlevel from inside a parenthesized `if ( ... goto waitloop ... )`
	// block -- a self-referential goto-inside-parens that worked fine typed
	// interactively but hung when the script was actually invoked (spawned
	// detached via CreateProcess). Every branch must be a single-line,
	// no-parens `if` now.
	if strings.Contains(script, "errorlevel 1 (") {
		t.Errorf("rendered script branches on pipe errorlevel from inside a parenthesized block:\n%s", script)
	}

	// Delayed expansion would corrupt any literal ! or % in the embedded
	// paths (user profile names, release asset names), so the script must
	// keep working with per-line expansion only.
	if strings.Contains(script, "EnableDelayedExpansion") || strings.Contains(script, "!FOUND_PID!") || strings.Contains(script, "!WAITED!") {
		t.Errorf("rendered script uses delayed expansion, which corrupts literal ! and %% in embedded paths:\n%s", script)
	}

	// cmd.exe batch parsing has documented failure modes with LF-only files,
	// so every line ending must be CRLF.
	if bareLF := strings.Count(script, "\n") - strings.Count(script, "\r\n"); bareLF != 0 {
		t.Errorf("rendered script has %d bare LF line endings; batch files must be CRLF:\n%s", bareLF, script)
	}
}

func TestWriteApplyScriptEscapesPercentInPaths(t *testing.T) {
	exePath := `C:\Users\50%off\Programs\Cratebug\Cratebug.exe`
	installerPath := `C:\Users\test\AppData\Local\Temp\cratebug-update\Cratebug!-installer.exe`

	scriptPath, err := writeApplyScript(exePath, installerPath, 1)
	if err != nil {
		t.Fatalf("writeApplyScript returned unexpected error: %v", err)
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading rendered script: %v", err)
	}
	script := string(content)

	for _, want := range []string{
		// % must be doubled to survive the set line's expansion pass; ! must
		// survive verbatim now that delayed expansion is off.
		`set "EXE_PATH=C:\Users\50%%off\Programs\Cratebug\Cratebug.exe"`,
		`set "INSTALLER_PATH=C:\Users\test\AppData\Local\Temp\cratebug-update\Cratebug!-installer.exe"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("rendered script missing expected escaped content %q\nfull script:\n%s", want, script)
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
