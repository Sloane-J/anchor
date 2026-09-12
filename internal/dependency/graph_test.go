package dependency

import (
	"reflect"
	"strings"
	"testing"

	"anchor/internal/config"
)

func svc(deps ...string) config.Service {
	return config.Service{Command: "noop", DependsOn: deps}
}

func TestBuildOrdersIndependentServicesByName(t *testing.T) {
	cfg := config.Config{
		Services: map[string]config.Service{
			"web": svc(),
			"api": svc(),
			"db":  svc(),
		},
	}

	plan, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	want := []string{"api", "db", "web"}
	if !reflect.DeepEqual(plan.Startup, want) {
		t.Fatalf("Startup = %v, want %v", plan.Startup, want)
	}
}

func TestBuildRespectsDependencies(t *testing.T) {
	cfg := config.Config{
		Services: map[string]config.Service{
			"web": svc("api"),
			"api": svc("db"),
			"db":  svc(),
		},
	}

	plan, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	index := make(map[string]int, len(plan.Startup))
	for i, name := range plan.Startup {
		index[name] = i
	}

	if index["db"] > index["api"] {
		t.Fatalf("db must start before api: order = %v", plan.Startup)
	}
	if index["api"] > index["web"] {
		t.Fatalf("api must start before web: order = %v", plan.Startup)
	}
}

func TestBuildBranchingGraph(t *testing.T) {
	// web depends on api and worker; both depend on db.
	cfg := config.Config{
		Services: map[string]config.Service{
			"web":    svc("api", "worker"),
			"api":    svc("db"),
			"worker": svc("db"),
			"db":     svc(),
		},
	}

	plan, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	index := make(map[string]int, len(plan.Startup))
	for i, name := range plan.Startup {
		index[name] = i
	}

	if index["db"] > index["api"] || index["db"] > index["worker"] {
		t.Fatalf("db must start before api and worker: order = %v", plan.Startup)
	}
	if index["api"] > index["web"] || index["worker"] > index["web"] {
		t.Fatalf("api and worker must start before web: order = %v", plan.Startup)
	}
	if len(plan.Startup) != 4 {
		t.Fatalf("Startup length = %d, want 4", len(plan.Startup))
	}
}

func TestBuildDetectsCycle(t *testing.T) {
	cfg := config.Config{
		Services: map[string]config.Service{
			"a": svc("b"),
			"b": svc("c"),
			"c": svc("a"),
		},
	}

	_, err := Build(cfg)
	if err == nil {
		t.Fatal("Build() error = nil, want cycle error")
	}
	var cycleErr CycleError
	if !errorsAs(err, &cycleErr) {
		t.Fatalf("Build() error = %v, want CycleError", err)
	}
	if !strings.Contains(cycleErr.Error(), "->") {
		t.Fatalf("CycleError.Error() = %q, want readable path", cycleErr.Error())
	}
}

func TestBuildDetectsSelfDependency(t *testing.T) {
	cfg := config.Config{
		Services: map[string]config.Service{
			"a": svc("a"),
		},
	}

	_, err := Build(cfg)
	if err == nil {
		t.Fatal("Build() error = nil, want cycle error")
	}
	var cycleErr CycleError
	if !errorsAs(err, &cycleErr) {
		t.Fatalf("Build() error = %v, want CycleError", err)
	}
}

func TestPlanShutdownIsReverseOfStartup(t *testing.T) {
	plan := Plan{Startup: []string{"db", "api", "web"}}
	want := []string{"web", "api", "db"}

	if got := plan.Shutdown(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Shutdown() = %v, want %v", got, want)
	}
}

// errorsAs avoids importing "errors" just for this narrow use in tests.
func errorsAs(err error, target *CycleError) bool {
	ce, ok := err.(CycleError)
	if !ok {
		return false
	}
	*target = ce
	return true
}