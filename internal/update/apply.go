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
	// Confirmed against that template as part of Phase 10 end-to-end testing
	// (TASKS.md 10.6); if a future NSIS template ever drops support for it,
	// ApplyUpdate needs to change alongside the installer.
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
// Every external command (tasklist, findstr, timeout -- not the cmd.exe
// built-ins like set/if/goto/start/del, which aren't resolved via PATH at
// all) is called through its full %SystemRoot%\System32 path, not by bare
// name. A bare `find` here previously resolved to GNU findutils' `find` on
// a machine with Git for Windows/MSYS ahead of System32 on PATH -- which
// treats "/I" as a starting directory rather than a flag and recursively
// scans the filesystem from there instead of searching a pipe. Confirmed
// live during Phase 10 testing, not a hypothetical.
var applyScriptTemplate = template.Must(template.New("apply").Parse(`@echo off
setlocal

set "SYS=%SystemRoot%\System32"
set "TARGET_PID={{.TargetPID}}"
set "EXE_PATH={{.ExePath}}"
set "INSTALLER_PATH={{.InstallerPath}}"
set "WAITED=0"

:waitloop
"%SYS%\tasklist.exe" /FI "PID eq %TARGET_PID%" 2>NUL | "%SYS%\findstr.exe" /I "%TARGET_PID%" >NUL
if not errorlevel 1 (
    if %WAITED% GEQ {{.MaxWaitSeconds}} goto waitfailed
    set /a WAITED+=1
    "%SYS%\timeout.exe" /T {{.PollIntervalSeconds}} /NOBREAK >NUL
    goto waitloop
)

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
		// DETACHED_PROCESS: survives Cratebug exiting: without it, Windows
		// treats the helper as a child of a console-less GUI app and may not
		// keep it alive independently.
		// CREATE_NEW_PROCESS_GROUP: isolates it from Cratebug's process
		// group so it isn't sent the same termination signals.
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
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
