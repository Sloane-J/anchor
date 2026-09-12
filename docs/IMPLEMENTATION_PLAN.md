# Implementation plan

This is the execution order. A phase is complete only when its acceptance criteria and tests pass; later phases must not be used to work around unfinished earlier work.

## Engineering conventions

- Use the standard library first. Add dependencies only when they remove substantial, well-defined work.
- Record the Go version in `go.mod`; introduce CI after the baseline build exists.
- Keep domain logic deterministic and inject process runners, clocks, and terminal writers through small interfaces for tests.
- Run `gofmt`, `go vet ./...`, and `go test ./...` for every completed change.
- Prefer table-driven unit tests. Integration tests create temporary resources and clean them with `t.Cleanup`.
- Errors name what failed, why, and how to fix it. Wrap operational errors with `%w`; keep validation errors concise.
- Document public commands in `README.md`; keep internal contracts beside their owner.
- Keep commits focused: one logical change, tests included, no unrelated formatting churn.

## Version roadmap

| Version | Outcome | Included | Explicitly excluded |
| --- | --- | --- | --- |
| `v0.1.0` | dependable foreground runner | validation, dependency ordering, start, live logs, Ctrl+C shutdown | restart, health, TUI, plugins |
| `v0.2.0` | service awareness | `status`, filtered `logs`, runtime state display | daemon mode |
| `v0.3.0` | resilience | opt-in restart policy and health checks | watch mode |
| `v0.4.0` | developer ergonomics | watch mode, richer terminal display | plugin execution |
| `v1.0.0` | stable local-dev contract | documented compatibility and cross-platform assessment | production orchestration |

Dates are intentionally omitted. Scope and quality gates are more useful than optimistic calendar estimates.

## v0.1 implementation phases

### Phase 0 ? repository foundation

Create the Go module, executable entry point, `.gitignore`, example configuration, baseline test command, and concise build/run README.

**Acceptance:** `go test ./...` and `go vet ./...` pass; `go run ./cmd/anchor --help` returns help without importing runtime/process packages.

### Phase 1 ? configuration and validation

Define the versioned `dev.yaml` schema, loader, defaults, and validator. Validate duplicate names, empty commands, unsupported version, invalid working directories, unknown dependencies, and malformed YAML.

**Acceptance:** invalid input starts no process; each error identifies field/service and remedy; unit tests cover valid and invalid files.

### Phase 2 ? dependency graph and plan

Build a DAG from validated services. Detect cycles with a readable path, derive deterministic startup order, and derive reverse shutdown order.

**Acceptance:** independent services retain configuration order; cycles and self-dependencies fail before scheduling; table-driven graph tests cover branching graphs.

### Phase 3 ? foreground supervisor and process adapter

Introduce a process-runner interface and Windows implementation. Start services by plan, track transitions, and stop started services in reverse order if later startup fails or cancellation arrives.

**Acceptance:** doubles verify start/stop ordering; an integration test launches a harmless child and confirms clean termination; no process is targeted by guessed PID or image name.

### Phase 4 ? logs, signals, and CLI polish

Multiplex stdout/stderr without blocking children, add terminal-safe formatting, translate Ctrl+C/Ctrl+Break to cancellation, and finish messages and exit codes.

**Acceptance:** concurrent output remains labelled; Ctrl+C triggers orderly shutdown; ordinary user errors have no stack trace; a PowerShell smoke test passes.

### Phase 5 ? release readiness

Add documentation, examples, coverage review, Windows manual test checklist, version metadata, and a reproducible build command.

**Acceptance:** a new user can build, configure, start, and stop the example stack from the README; automated checks pass; limitations are documented.
