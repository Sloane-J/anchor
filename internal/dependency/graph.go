// Package dependency builds a startup order from validated services.
package dependency

import (
	"fmt"
	"sort"
	"strings"

	"anchor/internal/config"
)

// CycleError reports a dependency cycle as a readable path.
type CycleError struct {
	Path []string
}

func (e CycleError) Error() string {
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(e.Path, " -> "))
}

// Plan is the ordered result of resolving a service dependency graph.
type Plan struct {
	// Startup lists service names in the order they should be started.
	// A service always appears after everything it depends on.
	Startup []string
}

// Shutdown returns service names in reverse startup order, the order in
// which running services should be stopped.
func (p Plan) Shutdown() []string {
	reversed := make([]string, len(p.Startup))
	for i, name := range p.Startup {
		reversed[len(p.Startup)-1-i] = name
	}
	return reversed
}

// Build resolves a deterministic startup order from cfg.Services.
// It detects cycles (including self-dependencies) and reports them as a
// readable path. Configuration order is preserved among services that have
// no ordering relationship to each other.
func Build(cfg config.Config) (Plan, error) {
	names := make([]string, 0, len(cfg.Services))
	for name := range cfg.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(names))
	order := make([]string, 0, len(names))

	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		switch state[name] {
		case visited:
			return nil
		case visiting:
			return CycleError{Path: append(append([]string{}, path...), name)}
		}

		state[name] = visiting
		path = append(path, name)

		deps := append([]string{}, cfg.Services[name].DependsOn...)
		sort.Strings(deps)
		for _, dep := range deps {
			if err := visit(dep, path); err != nil {
				return err
			}
		}

		state[name] = visited
		order = append(order, name)
		return nil
	}

	for _, name := range names {
		if err := visit(name, nil); err != nil {
			return Plan{}, err
		}
	}

	return Plan{Startup: order}, nil
}