package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state, err := Load(path)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if state == nil {
		t.Fatal("expected non-nil state")
	}
	if len(state.AssignedPRs) != 0 || len(state.AuthoredPRs) != 0 {
		t.Fatalf("expected empty state, got %#v", state)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := NewState()
	state.Initialized = true
	state.AssignedPRs["org/repo#1"] = time.Unix(100, 0).UTC()
	state.AuthoredPRs["org/repo#2"] = AuthoredRecord{
		LastIssueComment: time.Unix(200, 0).UTC(),
		LastReview:       time.Unix(300, 0).UTC(),
	}

	if err := Save(path, state); err != nil {
		t.Fatalf("Save error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected data to be written")
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load error = %v", err)
	}
	if !reloaded.Initialized {
		t.Errorf("Initialized flag lost")
	}
	if got := reloaded.AssignedPRs["org/repo#1"]; got.IsZero() {
		t.Errorf("AssignedPR timestamp missing")
	}
	if got := reloaded.AuthoredPRs["org/repo#2"]; got.LastReview.IsZero() {
		t.Errorf("AuthoredPR record missing: %#v", got)
	}
}
