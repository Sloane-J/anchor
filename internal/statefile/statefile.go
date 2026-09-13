// Package statefile persists a running anchor start session's status to
// disk, so a separate `anchor status` invocation can report on it without
// talking to the running process directly.
package statefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ServiceStatus is one service's last known state, as of the last time it
// changed.
type ServiceStatus struct {
	State string    `json:"state"`
	Since time.Time `json:"since"`
}

// State is the full snapshot written while `anchor start` runs.
type State struct {
	// PID is the anchor start process's own process ID, used by status to
	// determine whether this state file reflects a still-running session
	// or a stale leftover from a crash.
	PID int `json:"pid"`

	// ConfigPath is the dev.yaml this session was started from.
	ConfigPath string `json:"config_path"`

	// StartedAt is when this session began.
	StartedAt time.Time `json:"started_at"`

	// Services maps service name to its last known status.
	Services map[string]ServiceStatus `json:"services"`
}

// pathFor returns the state file path for a given dev.yaml config path:
// a .anchor directory next to the config file.
func pathFor(configPath string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, ".anchor", "state.json")
}

// Write saves state to disk next to configPath, creating the .anchor
// directory if needed. Write is safe to call repeatedly as state changes.
func Write(configPath string, state State) error {
	path := pathFor(configPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}

// Read loads the state file associated with configPath. It returns
// (State{}, false, nil) if no state file exists — that is not an error,
// it means no session has been started from this config, or it was
// already cleanly removed on shutdown.
func Read(configPath string) (State, bool, error) {
	path := pathFor(configPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, false, nil
		}
		return State{}, false, fmt.Errorf("read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("parse state file: %w", err)
	}
	return state, true, nil
}

// Remove deletes the state file for configPath. It is not an error if the
// file does not exist.
func Remove(configPath string) error {
	path := pathFor(configPath)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove state file: %w", err)
	}
	return nil
}