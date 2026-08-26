package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// Load reads one strictly-decoded dev.yaml document from path.
func Load(path string) (Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, fmt.Errorf("resolve configuration path %q: %w", path, err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration %q: %w", path, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			return Config{}, fmt.Errorf("configuration %q is empty", path)
		}
		return Config{}, fmt.Errorf("invalid YAML in %q: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("configuration %q contains more than one YAML document", path)
		}
		return Config{}, fmt.Errorf("invalid YAML in %q: %w", path, err)
	}

	cfg.SourcePath = absPath
	return cfg, nil
}
