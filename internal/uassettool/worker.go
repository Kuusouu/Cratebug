package uassettool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultCallTimeout   = 30 * time.Second
	defaultShutdownGrace = 5 * time.Second
	versionCheckTimeout  = 10 * time.Second

	// Bounds how long Call waits for the exit-reaping goroutine to observe a
	// process death after a pipe error, before falling back to that raw
	// error. The process closing its pipes and cmd.Wait() returning are two
	// separate events; this closes the gap between them.
	crashDetectionGrace = 2 * time.Second
)

// Overridable so tests can substitute a fake worker process without a live
// UAssetTool.exe; production code always uses the real os/exec functions.
var (
	execCommand        = exec.Command
	execCommandContext = exec.CommandContext
)

// Reported when the worker executable cannot be started, whether the path is
// wrong, the file is missing, or the OS refuses to launch it.
var ErrWorkerLaunchFailed = errors.New("uassettool: failed to launch worker process")

// Reported when the worker process exits while a call is outstanding, or is
// already dead when a new call is attempted.
var ErrWorkerCrashed = errors.New("uassettool: worker process exited unexpectedly")

// Reported when a call does not complete within its configured timeout. The
// worker is killed and reaped before this error is returned, so it never
// outlives the call it failed to answer.
var ErrWorkerTimeout = errors.New("uassettool: worker did not respond in time")

// Configures how a Worker launches and supervises the pinned UAssetTool.exe process.
type WorkerConfig struct {
	// Path to the pinned worker executable; see fetch-uassettool.ps1 and
	// docs/decisions/0004-pin-uassettool-worker.md.
	ExecutablePath string
	// Source revision the worker's own "--version" output must contain
	// before its output is trusted.
	ExpectedSourceRevision string
	// Bounds how long a single Call may take before the worker is treated as
	// hung and killed. Defaults to 30s when zero.
	CallTimeout time.Duration
	// Bounds how long Close waits for the worker to exit on its own after its
	// stdin is closed before killing it. Defaults to 5s when zero.
	ShutdownGrace time.Duration
	// Receives adapter and lifecycle log lines. May be nil to disable logging.
	Logger *log.Logger
}

// Supervises one launched UAssetTool.exe process: its version, its stdin and
// stdout pipes, and its exit.
type Worker struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	adapter *Adapter

	callTimeout   time.Duration
	shutdownGrace time.Duration
	logger        *log.Logger

	exited  chan struct{}
	exitErr error

	closeOnce sync.Once
}

// Verifies the pinned worker's version, then launches it in interactive JSON
// mode and starts supervising it. Returns ErrVersionMismatch without
// launching the interactive process if the reported version does not
// contain config.ExpectedSourceRevision, and ErrWorkerLaunchFailed if the
// executable cannot be started at all.
func NewWorker(config WorkerConfig) (*Worker, error) {
	if err := verifyWorkerVersion(config.ExecutablePath, config.ExpectedSourceRevision); err != nil {
		return nil, err
	}

	cmd := execCommand(config.ExecutablePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stderr = &stderrLogWriter{logger: config.Logger}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: open stdin: %v", ErrWorkerLaunchFailed, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: open stdout: %v", ErrWorkerLaunchFailed, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWorkerLaunchFailed, err)
	}

	callTimeout := config.CallTimeout
	if callTimeout <= 0 {
		callTimeout = defaultCallTimeout
	}
	shutdownGrace := config.ShutdownGrace
	if shutdownGrace <= 0 {
		shutdownGrace = defaultShutdownGrace
	}

	worker := &Worker{
		cmd:           cmd,
		stdin:         stdin,
		adapter:       NewAdapter(stdin, stdout, config.Logger),
		callTimeout:   callTimeout,
		shutdownGrace: shutdownGrace,
		logger:        config.Logger,
		exited:        make(chan struct{}),
	}

	go func() {
		worker.exitErr = cmd.Wait()
		close(worker.exited)
	}()

	return worker, nil
}

// Runs "--version" as a one-shot process, independent of the interactive
// worker, and confirms it names the pinned source revision.
func verifyWorkerVersion(executablePath, expectedSourceRevision string) error {
	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	cmd := execCommandContext(ctx, executablePath, "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%w: --version: %v", ErrWorkerLaunchFailed, err)
	}

	return CheckVersion(string(output), expectedSourceRevision)
}

// Sends one request to the supervised worker, killing and reaping it if it
// does not respond within its configured call timeout, and reporting a crash
// distinctly from a protocol-level failure if it exits mid-call.
func (w *Worker) Call(action string, params map[string]any, result any) error {
	select {
	case <-w.exited:
		return fmt.Errorf("%w: %v", ErrWorkerCrashed, w.exitErr)
	default:
	}

	done := make(chan error, 1)
	go func() {
		done <- w.adapter.Call(action, params, result)
	}()

	select {
	case err := <-done:
		if err == nil {
			return nil
		}

		// A malformed response or a worker-reported tool failure both mean
		// the process was alive and answered; only a lower-level transport
		// error (for example a broken pipe) can mean it died mid-call.
		var toolErr *ToolError
		if errors.Is(err, ErrMalformedResponse) || errors.As(err, &toolErr) {
			return err
		}

		// The exit-reaping goroutine may not have closed w.exited yet even
		// though the process is already gone, so wait briefly for it before
		// falling back to the raw transport error.
		select {
		case <-w.exited:
			return fmt.Errorf("%w: %v", ErrWorkerCrashed, w.exitErr)
		case <-time.After(crashDetectionGrace):
			return err
		}
	case <-w.exited:
		return fmt.Errorf("%w: %v", ErrWorkerCrashed, w.exitErr)
	case <-time.After(w.callTimeout):
		w.kill()
		<-w.exited
		return fmt.Errorf("%w: %q did not complete within %s", ErrWorkerTimeout, action, w.callTimeout)
	}
}

// Reports whether the worker process is still running, for callers deciding
// whether to replace this Worker before issuing another Call.
func (w *Worker) Alive() bool {
	select {
	case <-w.exited:
		return false
	default:
		return true
	}
}

// Closes the worker's stdin so its interactive read loop exits on its own,
// then waits up to the configured shutdown grace period before killing it.
// Close is safe to call more than once and always leaves the process reaped
// by the time it returns.
func (w *Worker) Close() error {
	w.closeOnce.Do(func() {
		_ = w.stdin.Close()

		select {
		case <-w.exited:
		case <-time.After(w.shutdownGrace):
			w.kill()
			<-w.exited
		}
	})
	return nil
}

func (w *Worker) kill() {
	if w.cmd.Process == nil {
		return
	}
	if err := w.cmd.Process.Kill(); err != nil && w.logger != nil {
		w.logger.Printf("uassettool: kill worker process: %v", err)
	}
}

// Forwards the worker's stderr to the configured logger a line at a time so
// diagnostic output is visible without ever blocking the process on a full pipe.
type stderrLogWriter struct {
	logger *log.Logger
}

func (w *stderrLogWriter) Write(p []byte) (int, error) {
	if w.logger != nil {
		w.logger.Printf("uassettool: stderr: %s", strings.TrimRight(string(p), "\r\n"))
	}
	return len(p), nil
}
