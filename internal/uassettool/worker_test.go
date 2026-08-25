package uassettool

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const testSourceRevision = "test-revision-abc123"

// Points execCommand and execCommandContext at this same test binary,
// re-invoked as a fake UAssetTool.exe via TestHelperProcess below, so
// process-lifecycle behavior (launch, crash, hang, version mismatch) can be
// exercised without a live UAssetTool.exe worker process. Restored after the test.
func installFakeWorker(t *testing.T, mode string) {
	t.Helper()

	build := func(args []string) *exec.Cmd {
		helperArgs := append([]string{"-test.run=^TestHelperProcess$", "--"}, args...)
		cmd := exec.Command(os.Args[0], helperArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "UASSETTOOL_TEST_MODE="+mode)
		return cmd
	}

	originalCommand := execCommand
	originalCommandContext := execCommandContext
	execCommand = func(name string, args ...string) *exec.Cmd {
		return build(args)
	}
	execCommandContext = func(_ context.Context, name string, args ...string) *exec.Cmd {
		return build(args)
	}
	t.Cleanup(func() {
		execCommand = originalCommand
		execCommandContext = originalCommandContext
	})
}

// TestHelperProcess is not a real test: it is re-executed as a child process
// standing in for UAssetTool.exe. See installFakeWorker.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	mode := os.Getenv("UASSETTOOL_TEST_MODE")

	if len(args) > 0 && args[0] == "--version" {
		if mode == "version-mismatch" {
			fmt.Println("UAssetTool v1.0.0+wrong-revision")
		} else {
			fmt.Printf("UAssetTool v1.0.0+%s\n", testSourceRevision)
		}
		return
	}

	// Interactive mode: mirrors RunInteractiveMode's read-until-EOF loop so
	// closing stdin is a realistic graceful-shutdown trigger in these tests too.
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if line == "" && err != nil {
			return
		}

		switch mode {
		case "crash-on-call":
			os.Exit(1)
		case "hang-on-call":
			// A bare select{} here would trip Go's own deadlock detector
			// (no other goroutine could ever wake this process), killing it
			// instantly instead of simulating an unresponsive worker.
			time.Sleep(time.Hour)
		case "malformed-output":
			fmt.Println("not json")
		default:
			fmt.Println(`{"success":true,"message":"ok","data":{"echo":true}}`)
		}
	}
}

func TestNewWorkerRejectsVersionMismatch(t *testing.T) {
	// Arrange
	installFakeWorker(t, "version-mismatch")

	// Act
	worker, err := NewWorker(WorkerConfig{ExecutablePath: "fake-uassettool.exe", ExpectedSourceRevision: testSourceRevision})

	// Assert
	if worker != nil {
		t.Fatalf("NewWorker() worker = %v, want nil", worker)
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("NewWorker() error = %v, want ErrVersionMismatch", err)
	}
}

func TestNewWorkerReturnsLaunchFailedForMissingExecutable(t *testing.T) {
	// Arrange: leave execCommand/execCommandContext pointed at the real
	// os/exec functions and a path that does not exist.
	missing := filepath.Join(t.TempDir(), "does-not-exist.exe")

	// Act
	worker, err := NewWorker(WorkerConfig{ExecutablePath: missing, ExpectedSourceRevision: testSourceRevision})

	// Assert
	if worker != nil {
		t.Fatalf("NewWorker() worker = %v, want nil", worker)
	}
	if !errors.Is(err, ErrWorkerLaunchFailed) {
		t.Fatalf("NewWorker() error = %v, want ErrWorkerLaunchFailed", err)
	}
}

func TestWorkerCallRoundTripsThroughSupervisedProcess(t *testing.T) {
	// Arrange
	installFakeWorker(t, "functioning")
	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         "fake-uassettool.exe",
		ExpectedSourceRevision: testSourceRevision,
		CallTimeout:            2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	// Act
	var result struct {
		Echo bool `json:"echo"`
	}
	callErr := worker.Call("is_iostore_encrypted", map[string]any{"file_path": "mod.utoc"}, &result)

	// Assert
	if callErr != nil {
		t.Fatalf("Call() error = %v, want nil", callErr)
	}
	if !result.Echo {
		t.Errorf("result.Echo = false, want true")
	}
}

func TestWorkerCloseTerminatesProcessWithoutOrphan(t *testing.T) {
	// Arrange
	installFakeWorker(t, "functioning")
	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         "fake-uassettool.exe",
		ExpectedSourceRevision: testSourceRevision,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	// Act
	closeErr := worker.Close()

	// Assert
	if closeErr != nil {
		t.Fatalf("Close() error = %v, want nil", closeErr)
	}
	if worker.Alive() {
		t.Errorf("Alive() = true after Close(), want false: process was not reaped")
	}
}

func TestWorkerCallReturnsCrashedAndLeavesNoOrphan(t *testing.T) {
	// Arrange
	installFakeWorker(t, "crash-on-call")
	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         "fake-uassettool.exe",
		ExpectedSourceRevision: testSourceRevision,
		CallTimeout:            2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	// Act
	callErr := worker.Call("list_pak", nil, nil)

	// Assert
	if !errors.Is(callErr, ErrWorkerCrashed) {
		t.Fatalf("Call() error = %v, want ErrWorkerCrashed", callErr)
	}
	if worker.Alive() {
		t.Errorf("Alive() = true after crash, want false: process was not reaped")
	}
}

func TestWorkerCallReturnsTimeoutAndKillsHungProcess(t *testing.T) {
	// Arrange
	installFakeWorker(t, "hang-on-call")
	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         "fake-uassettool.exe",
		ExpectedSourceRevision: testSourceRevision,
		CallTimeout:            300 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	// Act
	callErr := worker.Call("list_pak", nil, nil)

	// Assert
	if !errors.Is(callErr, ErrWorkerTimeout) {
		t.Fatalf("Call() error = %v, want ErrWorkerTimeout", callErr)
	}
	if worker.Alive() {
		t.Errorf("Alive() = true after timeout kill, want false: process was not reaped")
	}
}

func TestWorkerCallReturnsMalformedResponse(t *testing.T) {
	// Arrange
	installFakeWorker(t, "malformed-output")
	worker, err := NewWorker(WorkerConfig{
		ExecutablePath:         "fake-uassettool.exe",
		ExpectedSourceRevision: testSourceRevision,
		CallTimeout:            2 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}
	defer worker.Close()

	// Act
	callErr := worker.Call("list_pak", nil, nil)

	// Assert
	if !errors.Is(callErr, ErrMalformedResponse) {
		t.Fatalf("Call() error = %v, want ErrMalformedResponse", callErr)
	}
}
