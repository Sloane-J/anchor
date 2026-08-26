package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := writeConfig(t, `version: 1
services:
  api:
    command: npm
    args: ["run", "dev"]
    working_dir: ./api
    depends_on: [database]
  database:
    command: docker
    args: ["compose", "up"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.SourcePath != path {
		t.Fatalf("SourcePath = %q, want %q", cfg.SourcePath, path)
	}
	if cfg.Version != CurrentVersion {
		t.Fatalf("Version = %d, want %d", cfg.Version, CurrentVersion)
	}
	if got := cfg.Services["api"].Args; len(got) != 2 || got[0] != "run" || got[1] != "dev" {
		t.Fatalf("api args = %#v, want [run dev]", got)
	}
}

func TestLoadRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "empty", contents: "", want: "is empty"},
		{name: "unknown field", contents: "version: 1\nunknown: true\nservices: {}\n", want: "field unknown not found"},
		{name: "duplicate service", contents: "version: 1\nservices:\n  api: {command: npm}\n  api: {command: pnpm}\n", want: "mapping key"},
		{name: "multiple documents", contents: "version: 1\nservices: {}\n---\nversion: 1\nservices: {}\n", want: "more than one YAML document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeConfig(t, tt.contents))
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dev.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write configuration: %v", err)
	}
	return path
}
