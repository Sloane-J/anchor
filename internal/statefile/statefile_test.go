package statefile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	want := State{
		PID:        1234,
		ConfigPath: configPath,
		StartedAt:  time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Services: map[string]ServiceStatus{
			"api": {State: "running", Since: time.Date(2026, 1, 1, 12, 0, 5, 0, time.UTC)},
		},
	}

	if err := Write(configPath, want); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, ok, err := Read(configPath)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if !ok {
		t.Fatal("Read() ok = false, want true")
	}
	if got.PID != want.PID {
		t.Errorf("PID = %d, want %d", got.PID, want.PID)
	}
	if got.ConfigPath != want.ConfigPath {
		t.Errorf("ConfigPath = %q, want %q", got.ConfigPath, want.ConfigPath)
	}
	if !got.StartedAt.Equal(want.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", got.StartedAt, want.StartedAt)
	}
	apiStatus, ok := got.Services["api"]
	if !ok {
		t.Fatal(`Services["api"] missing`)
	}
	if apiStatus.State != "running" {
		t.Errorf(`Services["api"].State = %q, want "running"`, apiStatus.State)
	}
}

func TestReadReturnsFalseWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	state, ok, err := Read(configPath)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if ok {
		t.Fatal("Read() ok = true, want false when no state file exists")
	}
		if state.PID != 0 || state.ConfigPath != "" || len(state.Services) != 0 {
		t.Fatalf("Read() state = %+v, want zero value", state)
	}
}

func TestWriteCreatesAnchorDirectory(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	if err := Write(configPath, State{PID: 1}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, ".anchor"))
	if err != nil {
		t.Fatalf("stat .anchor directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".anchor exists but is not a directory")
	}
}

func TestRemoveDeletesStateFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	if err := Write(configPath, State{PID: 1}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := Remove(configPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	_, ok, err := Read(configPath)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if ok {
		t.Fatal("Read() ok = true after Remove(), want false")
	}
}

func TestRemoveIsSafeWhenFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	if err := Remove(configPath); err != nil {
		t.Fatalf("Remove() error = %v, want nil for already-missing file", err)
	}
}

func TestWriteOverwritesPreviousState(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "dev.yaml")

	first := State{PID: 1, Services: map[string]ServiceStatus{"api": {State: "starting"}}}
	second := State{PID: 1, Services: map[string]ServiceStatus{"api": {State: "running"}}}

	if err := Write(configPath, first); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if err := Write(configPath, second); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}

	got, _, err := Read(configPath)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got.Services["api"].State != "running" {
		t.Fatalf(`Services["api"].State = %q, want "running" (should reflect latest write)`, got.Services["api"].State)
	}
}