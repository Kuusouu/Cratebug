package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kuusouu/Cratebug/internal/install"
	"github.com/Kuusouu/Cratebug/internal/uassettool"
)

type stagedCompanionCaller struct {
	listing  string
	failList bool
}

func (s *stagedCompanionCaller) Call(action string, params map[string]any, result any) error {
	switch action {
	case "list_pak":
		if s.failList {
			return &uassettool.ToolError{Action: action, Message: "list failed"}
		}
		body := s.listing
		if body == "" {
			body = `{"files":[]}`
		}
		return json.Unmarshal([]byte(body), result)
	case "is_iostore_encrypted":
		return json.Unmarshal([]byte(`{"encrypted":false}`), result)
	case "create_pak":
		output, _ := params["output_path"].(string)
		return os.WriteFile(output, []byte("rewritten-companion"), 0o600)
	default:
		return nil
	}
}

func TestStripStagedCompanionPaksRewritesDirtyPrimary(t *testing.T) {
	// Arrange
	staging := t.TempDir()
	pak := filepath.Join(staging, "Hero_9999999_P.pak")
	if err := os.WriteFile(pak, []byte("dirty-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := &install.StagedSession{
		Dir: staging,
		Mods: []install.StagedMod{{
			ID:                  "mod-1",
			RelativePrimaryPath: "Hero_9999999_P.pak",
			DisplayName:         "Hero",
		}},
	}
	caller := &stagedCompanionCaller{
		listing: `{"files":[{"path":"chunknames","size":1}]}`,
	}

	// Act
	err := stripStagedCompanionPaks(session, []install.ApplyItem{{ID: "mod-1"}}, caller)

	// Assert
	if err != nil {
		t.Fatalf("stripStagedCompanionPaks() error = %v", err)
	}
	body, err := os.ReadFile(pak)
	if err != nil {
		t.Fatalf("read staged pak: %v", err)
	}
	if string(body) != "rewritten-companion" {
		t.Errorf("staged pak = %q, want rewritten-companion", body)
	}
}

func TestStripStagedCompanionPaksLeavesUnlistablePak(t *testing.T) {
	// Arrange
	staging := t.TempDir()
	pak := filepath.Join(staging, "Hero_9999999_P.pak")
	if err := os.WriteFile(pak, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := &install.StagedSession{
		Dir: staging,
		Mods: []install.StagedMod{{
			ID:                  "mod-1",
			RelativePrimaryPath: "Hero_9999999_P.pak",
			DisplayName:         "Hero",
		}},
	}

	// Act
	err := stripStagedCompanionPaks(session, []install.ApplyItem{{ID: "mod-1"}}, &stagedCompanionCaller{failList: true})

	// Assert
	if err != nil {
		t.Fatalf("stripStagedCompanionPaks() error = %v", err)
	}
	body, err := os.ReadFile(pak)
	if err != nil {
		t.Fatalf("read staged pak: %v", err)
	}
	if string(body) != "original" {
		t.Errorf("unlistable pak was rewritten: %q", body)
	}
}
