// Package validator performs semantic validation of a loaded dev.yaml file.
package validator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"anchor/internal/config"
)

var serviceName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// Error collects configuration problems so developers can fix them in one pass.
type Error struct {
	Problems []string
}

func (e Error) Error() string {
	return "invalid dev.yaml:\n- " + strings.Join(e.Problems, "\n- ")
}

// Validate reports every structural issue that can be found without starting a process.
func Validate(cfg config.Config) error {
	var problems []string
	if cfg.Version != config.CurrentVersion {
		problems = append(problems, fmt.Sprintf("version must be %d, got %d", config.CurrentVersion, cfg.Version))
	}
	if len(cfg.Services) == 0 {
		problems = append(problems, "services must define at least one service")
	}

	baseDir := "."
	if cfg.SourcePath != "" {
		baseDir = filepath.Dir(cfg.SourcePath)
	}

	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		service := cfg.Services[name]
		if !serviceName.MatchString(name) {
			problems = append(problems, fmt.Sprintf("service %q has an invalid name; use letters, numbers, hyphens, or underscores and begin with a letter", name))
		}
		if strings.TrimSpace(service.Command) == "" {
			problems = append(problems, fmt.Sprintf("service %q: command is required", name))
		}

		workingDir := baseDir
		if service.WorkingDir != "" {
			workingDir = service.WorkingDir
			if !filepath.IsAbs(workingDir) {
				workingDir = filepath.Join(baseDir, workingDir)
			}
		}
		info, err := os.Stat(workingDir)
		if err != nil {
			problems = append(problems, fmt.Sprintf("service %q: working_dir %q does not exist", name, service.WorkingDir))
		} else if !info.IsDir() {
			problems = append(problems, fmt.Sprintf("service %q: working_dir %q is not a directory", name, service.WorkingDir))
		}

		seenDependencies := make(map[string]struct{}, len(service.DependsOn))
		for _, dependency := range service.DependsOn {
			if _, seen := seenDependencies[dependency]; seen {
				problems = append(problems, fmt.Sprintf("service %q: dependency %q is listed more than once", name, dependency))
				continue
			}
			seenDependencies[dependency] = struct{}{}
			if dependency == name {
				problems = append(problems, fmt.Sprintf("service %q cannot depend on itself", name))
				continue
			}
			if _, ok := cfg.Services[dependency]; !ok {
				problems = append(problems, fmt.Sprintf("service %q depends on unknown service %q", name, dependency))
			}
		}
	}

	if len(problems) > 0 {
		return Error{Problems: problems}
	}
	return nil
}
