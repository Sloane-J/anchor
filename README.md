# Dev Orchestrator

Windows-first Go CLI for starting a local development stack from `dev.yaml`.

The first release provides one reliable foreground workflow: validate configuration, start services in dependency order, stream labelled logs, and cleanly stop all started services on Ctrl+C.

## Project documents

- [Architecture](ARCHITECTURE.md)
- [Project context](docs/PROJECT_CONTEXT.md)
- [Implementation plan](docs/IMPLEMENTATION_PLAN.md)
- [Initial planning decision](docs/decisions/2026-08-26-initial-planning.md)

## Current stage

Planning is complete. The next implementation work is Phase 0 and Phase 1: initialise the Go module, then define and test the `dev.yaml` configuration contract before adding process execution.

## Build and run

Prerequisite: Go 1.26 or later.

```powershell
go test ./...
go vet ./...
go run ./cmd/dev --help
```

The executable currently supplies the command shell only. `dev start` is implemented in Phase 3 after the configuration, dependency, and process contracts are tested.

An example configuration is available at [`examples/dev.yaml`](examples/dev.yaml). Copy it into the repository you want to run and adapt its services; loading and validation will be added in Phase 1.
