package uassettool

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Verifies a successful call both encodes the request envelope correctly and
// decodes the response's data payload into the caller's result type.
func TestCallSendsRequestAndDecodesResult(t *testing.T) {
	// Arrange
	var sent bytes.Buffer
	responses := strings.NewReader(`{"success":true,"message":"ok","data":{"encrypted":true}}` + "\n")
	adapter := NewAdapter(&sent, responses, nil)

	// Act
	var result struct {
		Encrypted bool `json:"encrypted"`
	}
	err := adapter.Call("is_iostore_encrypted", map[string]any{"file_path": "mod.utoc"}, &result)

	// Assert
	if err != nil {
		t.Fatalf("Call() error = %v, want nil", err)
	}
	if !result.Encrypted {
		t.Errorf("result.Encrypted = false, want true")
	}

	var request map[string]any
	if err := json.Unmarshal(bytes.TrimRight(sent.Bytes(), "\n"), &request); err != nil {
		t.Fatalf("decode sent request: %v", err)
	}
	if request["action"] != "is_iostore_encrypted" {
		t.Errorf("request[action] = %v, want is_iostore_encrypted", request["action"])
	}
	if request["file_path"] != "mod.utoc" {
		t.Errorf("request[file_path] = %v, want mod.utoc", request["file_path"])
	}
}

// A call the worker rejects must surface the worker's own message through a
// ToolError rather than a generic protocol error.
func TestCallReturnsToolErrorOnFailure(t *testing.T) {
	// Arrange
	responses := strings.NewReader(`{"success":false,"message":"UTOC file not found: mod.utoc","data":null}` + "\n")
	adapter := NewAdapter(&bytes.Buffer{}, responses, nil)

	// Act
	err := adapter.Call("is_iostore_encrypted", map[string]any{"file_path": "mod.utoc"}, nil)

	// Assert
	var toolErr *ToolError
	if !errors.As(err, &toolErr) {
		t.Fatalf("Call() error = %v, want *ToolError", err)
	}
	if toolErr.Message != "UTOC file not found: mod.utoc" {
		t.Errorf("toolErr.Message = %q, want the worker's message", toolErr.Message)
	}
}

// A response line that is not valid JSON must be reported distinctly from a
// worker-reported failure, since it signals a protocol or process problem.
func TestCallReturnsMalformedResponseOnInvalidJSON(t *testing.T) {
	// Arrange
	responses := strings.NewReader("not json\n")
	adapter := NewAdapter(&bytes.Buffer{}, responses, nil)

	// Act
	err := adapter.Call("is_iostore_encrypted", nil, nil)

	// Assert
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("Call() error = %v, want ErrMalformedResponse", err)
	}
}

// A successful response whose data payload does not match the caller's result
// type is also a malformed response: the worker kept its contract, but this
// adapter's caller assumed the wrong shape for it.
func TestCallReturnsMalformedResponseOnUnexpectedDataShape(t *testing.T) {
	// Arrange
	responses := strings.NewReader(`{"success":true,"message":"ok","data":"unexpected string"}` + "\n")
	adapter := NewAdapter(&bytes.Buffer{}, responses, nil)

	// Act
	var result struct {
		Encrypted bool `json:"encrypted"`
	}
	err := adapter.Call("is_iostore_encrypted", nil, &result)

	// Assert
	if !errors.Is(err, ErrMalformedResponse) {
		t.Fatalf("Call() error = %v, want ErrMalformedResponse", err)
	}
}

// Two calls over the same transport must each read exactly one line, so a
// second call is not left decoding leftover bytes from the first response.
func TestCallReadsOneResponseLinePerCall(t *testing.T) {
	// Arrange
	responses := strings.NewReader(
		`{"success":true,"message":"first","data":null}` + "\n" +
			`{"success":true,"message":"second","data":null}` + "\n",
	)
	adapter := NewAdapter(&bytes.Buffer{}, responses, nil)

	// Act
	firstErr := adapter.Call("list_pak", nil, nil)
	secondErr := adapter.Call("list_pak", nil, nil)

	// Assert
	if firstErr != nil || secondErr != nil {
		t.Fatalf("Call() errors = %v, %v, want nil, nil", firstErr, secondErr)
	}
}

func TestCheckVersionAcceptsMatchingRevision(t *testing.T) {
	// Act
	err := CheckVersion("UAssetTool v1.0.0+"+PinnedSourceRevision, PinnedSourceRevision)

	// Assert
	if err != nil {
		t.Errorf("CheckVersion() error = %v, want nil", err)
	}
}

func TestCheckVersionRejectsMismatchedRevision(t *testing.T) {
	// Act
	err := CheckVersion("UAssetTool v1.0.0+deadbeef", PinnedSourceRevision)

	// Assert
	if !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("CheckVersion() error = %v, want ErrVersionMismatch", err)
	}
}
