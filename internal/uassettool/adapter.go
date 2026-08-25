package uassettool

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
)

// Matches the worker's UAssetResponse envelope: every action reports success,
// a human-readable message, and an action-specific data payload.
type response struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Reported when a response line cannot be parsed as the worker's response
// envelope, or when its data field cannot be decoded into the caller's result type.
var ErrMalformedResponse = errors.New("uassettool: malformed response from worker")

// Reported by CheckVersion when the worker's reported version does not
// contain the pinned source revision.
var ErrVersionMismatch = errors.New("uassettool: worker version does not match the pinned release")

// Reports a call the worker understood but could not complete, distinct from a
// transport or protocol failure. Callers can match it with errors.As.
type ToolError struct {
	Action  string
	Message string
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("uassettool: %s failed: %s", e.Action, e.Message)
}

// Speaks the worker's newline-delimited JSON protocol over an already-connected
// transport. Adapter has no opinion on how that transport was obtained, so it can
// be driven by a pipe or buffer test double without a live worker process.
type Adapter struct {
	writer io.Writer
	reader *bufio.Reader
	logger *log.Logger
}

// Creates an adapter over the given request writer and response reader.
// logger may be nil to disable logging.
func NewAdapter(requests io.Writer, responses io.Reader, logger *log.Logger) *Adapter {
	return &Adapter{writer: requests, reader: bufio.NewReader(responses), logger: logger}
}

// Sends one request and decodes its data payload into result, which may be nil
// when the caller only needs to know the call succeeded. params becomes the
// request's fields alongside "action"; it may be nil for actions that take none.
func (a *Adapter) Call(action string, params map[string]any, result any) error {
	request := make(map[string]any, len(params)+1)
	for key, value := range params {
		request[key] = value
	}
	request["action"] = action

	line, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("uassettool: encode %s request: %w", action, err)
	}

	a.logf("uassettool: -> %s", action)
	if _, err := a.writer.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("uassettool: write %s request: %w", action, err)
	}

	responseLine, err := a.reader.ReadString('\n')
	if err != nil && responseLine == "" {
		return fmt.Errorf("uassettool: read %s response: %w", action, err)
	}

	var resp response
	if err := json.Unmarshal([]byte(responseLine), &resp); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrMalformedResponse, action, err)
	}

	if !resp.Success {
		a.logf("uassettool: <- %s failed: %s", action, resp.Message)
		return &ToolError{Action: action, Message: resp.Message}
	}
	a.logf("uassettool: <- %s ok", action)

	if result == nil || len(resp.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp.Data, result); err != nil {
		return fmt.Errorf("%w: %s data: %v", ErrMalformedResponse, action, err)
	}
	return nil
}

// Confirms a worker's self-reported "--version" output names the pinned source
// revision. This is a plain string check so it needs no running worker process;
// the caller is responsible for actually invoking "--version" before trusting
// any further call to the same worker. The check is a substring match because
// the worker embeds its build commit as an informational version suffix, for
// example "UAssetTool v1.0.0+952bd331976c6f28efb36ca320c82c27e2456023".
func CheckVersion(reportedVersion, expectedSourceRevision string) error {
	if !strings.Contains(reportedVersion, expectedSourceRevision) {
		return fmt.Errorf("%w: got %q, want a version containing %q", ErrVersionMismatch, reportedVersion, expectedSourceRevision)
	}
	return nil
}

func (a *Adapter) logf(format string, args ...any) {
	if a.logger != nil {
		a.logger.Printf(format, args...)
	}
}
