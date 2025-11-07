package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type AuthoredRecord struct {
	LastIssueComment time.Time `json:"last_issue_comment"`
	LastReview       time.Time `json:"last_review"`
}

type State struct {
	Initialized bool                      `json:"initialized"`
	AssignedPRs map[string]time.Time      `json:"assigned_prs"`
	AuthoredPRs map[string]AuthoredRecord `json:"authored_prs"`
}

func NewState() *State {
	return &State{
		AssignedPRs: make(map[string]time.Time),
		AuthoredPRs: make(map[string]AuthoredRecord),
	}
}

func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewState(), nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}
	if len(data) == 0 {
		return NewState(), nil
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}
	if state.AssignedPRs == nil {
		state.AssignedPRs = make(map[string]time.Time)
	}
	if state.AuthoredPRs == nil {
		state.AuthoredPRs = make(map[string]AuthoredRecord)
	}
	return &state, nil
}

func Save(path string, state *State) error {
	if state == nil {
		return errors.New("state is nil")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure cache dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}
