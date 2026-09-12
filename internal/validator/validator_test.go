package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"anchor/internal/config"
)

func TestValidateAcceptsValidConfiguration(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		Version:    config.CurrentVersion,
		SourcePath: filepath.Join(dir, "dev.yaml"),
		Services: map[string]config.Service{
			"api": {
				Command:   "npm",
				Args:      []string{"run", "dev"},
				DependsOn: []string{"database"},
			},
			"database": {Command: "docker", Args: []string{"compose", "up"}},
		},
	}

	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	cfg := config.Config{
		Version:    2,
		SourcePath: filepath.Join(t.TempDir(), "dev.yaml"),
		Services: map[string]config.Service{
			"bad name": {
				WorkingDir: "missing-directory",
				DependsOn:  []string{"unknown", "unknown", "bad name"},
			},
		},
	}

	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	for _, want := range []string{
		"version must be 1, got 2",
		"service \"bad name\" has an invalid name",
		"service \"bad name\": command is required",
		"working_dir \"missing-directory\" does not exist",
		"depends on unknown service \"unknown\"",
		"dependency \"unknown\" is listed more than once",
		"cannot depend on itself",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error = %q, want substring %q", err, want)
		}
	}
}

func TestValidateRejectsFileAsWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	workingFile := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(workingFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("create working-dir fixture: %v", err)
	}
	cfg := config.Config{
		Version:    config.CurrentVersion,
		SourcePath: filepath.Join(dir, "dev.yaml"),
		Services: map[string]config.Service{
			"api": {Command: "npm", WorkingDir: "not-a-directory"},
		},
	}

	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("Validate() error = %v, want non-directory error", err)
	}
}
