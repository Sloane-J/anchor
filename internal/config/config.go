// Package config defines and loads the dev.yaml configuration format.
package config

// CurrentVersion is the only supported dev.yaml version.
const CurrentVersion = 1

// Config is the root document of a dev.yaml file.
type Config struct {
	Version  int                `yaml:"version"`
	Services map[string]Service `yaml:"services"`

	// SourcePath is assigned by Load and is not read from YAML.
	SourcePath string `yaml:"-"`
}

// Service defines one local development process.
type Service struct {
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
	WorkingDir string   `yaml:"working_dir"`
	DependsOn  []string `yaml:"depends_on"`
}
