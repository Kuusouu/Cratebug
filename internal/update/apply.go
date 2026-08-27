package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/template"

	"golang.org/x/sys/windows"
)

const (
	// Silent-install flag the Wails-generated per-user NSIS template accepts.
	nsisSilentInstallFlag = "/S"

	// How often the helper script polls for the running Cratebug process to
	// have exited, and how long it waits in total before giving up rather
	// than running the installer over a still-running instance.
	applyHelperPollIntervalSeconds = 1
	applyHelperMaxWaitSeconds      = 30

	// Fixed staging subdirectory under the OS temp dir. Reused (and cleared)
	// across attempts rather than made unique per call, since only one
	// update apply is ever in flight at a time.
	applyStagingDirName = "cratebug-update"
)

// Renders the apply helper as a Windows batch script rather than building it
// with fmt.Sprintf, since batch's own %VAR% syntax would otherwise collide
// with Sprintf's % verb escaping on every substitution.
//
// External commands (tasklist, timeout) are called through their full
// %SystemRoot%\System32 path rather than by bare name, since a PATH entry
// ahead of System32 (Git for Windows, MSYS, Cygwin) can substitute a
// different program with the same name.
//
// The still-running check writes tasklist's CSV output to a file and reads
// it back with `for /f`, rather than piping to a second process and
// branching on its errorlevel: that form put its retry `goto` inside a
// parenthesized `if ( ... )` block, a self-referential goto-inside-parens
// pattern that's erratic in cmd.exe, so every `if` below is a flat,
// single-line branch instead. The snapshot file also sidesteps `for /f`
// itself failing to parse a command string that contains its own quotes
// (the exe path, the /FI filter value).
var applyScriptTemplate = template.Must(template.New("apply").Parse(`@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "SYS=%SystemRoot%\System32"
set "TARGET_PID={{.TargetPID}}"
set "EXE_PATH={{.ExePath}}"
set "INSTALLER_PATH={{.InstallerPath}}"
set "SNAPSHOT=%TEMP%\cratebug-update-tasklist.tmp"
set /a "WAITED=0"

:waitloop
set "FOUND_PID="
"%SYS%\tasklist.exe" /FI "PID eq %TARGET_PID%" /FO CSV /NH > "%SNAPSHOT%" 2>NUL
for /f "tokens=2 delims=," %%P in (%SNAPSHOT%) do set "FOUND_PID=%%~P"
if not "!FOUND_PID!"=="%TARGET_PID%" goto waitdone
if !WAITED! GEQ {{.MaxWaitSeconds}} goto waitfailed
set /a "WAITED+=1"
"%SYS%\timeout.exe" /T {{.PollIntervalSeconds}} /NOBREAK >NUL
goto waitloop

:waitdone
echo Running installer silently...
start /wait "" "%INSTALLER_PATH%" {{.SilentFlag}}
if errorlevel 1 goto installfailed

echo Relaunching Cratebug...
start "" "%EXE_PATH%"
goto cleanup

:waitfailed
echo Cratebug did not exit in time; leaving the update unapplied.
goto cleanup

:installfailed
echo Installer reported a failure; not relaunching Cratebug.
goto cleanup

:cleanup
del "%SNAPSHOT%" 2>nul
del "%INSTALLER_PATH%" 2>nul
(goto) 2>nul & del "%~f0" & exit
`))

type applyScriptData struct {
	TargetPID           int
	ExePath             string
	InstallerPath       string
	SilentFlag          string
	MaxWaitSeconds      int
	PollIntervalSeconds int
}

// Launches a detached helper that waits for the running Cratebug process to
// exit, silently runs installerPath, and relaunches Cratebug from its
// current location. Returns once the helper has been spawned; it does not
// wait for the helper to finish, since Cratebug is expected to quit itself
// immediately after this returns successfully.
//
// A running executable can't replace itself on Windows, which is why this
// exists as a separate detached process instead of Cratebug performing the
// install directly.
func ApplyUpdate(installerPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: resolve running executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("update: resolve running executable path: %w", err)
	}

	scriptPath, err := writeApplyScript(exePath, installerPath, os.Getpid())
	if err != nil {
		return err
	}

	cmd := exec.Command(systemCommandPath("cmd.exe"), "/C", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Windows doesn't kill child processes when their parent exits, so
		// nothing else is needed to keep this running after Cratebug quits.
		//
		// CREATE_NO_WINDOW suppresses the console Windows would otherwise
		// auto-allocate for cmd.exe spawned from a console-less GUI parent.
		// It must not be combined with DETACHED_PROCESS or
		// CREATE_NEW_CONSOLE: per Microsoft's Process Creation Flags docs,
		// it's silently ignored when combined with either.
		//
		// CREATE_NEW_PROCESS_GROUP isolates it from Cratebug's process
		// group so it isn't sent the same termination signals.
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("update: launch apply helper: %w", err)
	}
	return nil
}

// Resolves name to its full path under the OS's System32 directory instead
// of relying on exec.Command's own PATH search, so a PATH entry ahead of
// System32 (Git for Windows, MSYS, Cygwin, ...) can never substitute a
// different program with the same bare name. Falls back to the bare name
// only if SystemRoot somehow isn't set, which is exec.Command's prior
// behavior, not a regression.
func systemCommandPath(name string) string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		return name
	}
	return filepath.Join(systemRoot, "System32", name)
}

// Renders the apply helper script into a fresh staging directory and
// returns its path. Separated from ApplyUpdate so the rendered output can
// be verified by tests without spawning a real process.
func writeApplyScript(exePath, installerPath string, pid int) (string, error) {
	dir := filepath.Join(os.TempDir(), applyStagingDirName)
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("update: clear previous apply staging directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("update: create apply staging directory: %w", err)
	}

	scriptPath := filepath.Join(dir, "apply.bat")
	file, err := os.Create(scriptPath)
	if err != nil {
		return "", fmt.Errorf("update: create apply helper script: %w", err)
	}
	defer file.Close()

	data := applyScriptData{
		TargetPID:           pid,
		ExePath:             exePath,
		InstallerPath:       installerPath,
		SilentFlag:          nsisSilentInstallFlag,
		MaxWaitSeconds:      applyHelperMaxWaitSeconds,
		PollIntervalSeconds: applyHelperPollIntervalSeconds,
	}
	if err := applyScriptTemplate.Execute(file, data); err != nil {
		return "", fmt.Errorf("update: render apply helper script: %w", err)
	}
	return scriptPath, nil
}
