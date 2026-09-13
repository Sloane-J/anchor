# Anchor

Windows-first Go CLI for starting a local development stack from `dev.yaml`.

Validates configuration, starts services in dependency order, streams
labelled logs, and cleanly stops every started service on Ctrl+C.

## Project documents

- [Architecture](ARCHITECTURE.md)
- [Project context](docs/PROJECT_CONTEXT.md)
- [Implementation plan](docs/IMPLEMENTATION_PLAN.md)
- [Initial planning decision](docs/decisions/2026-08-26-initial-planning.md)

## Current stage

`anchor start` is implemented and working: configuration loading and
validation, dependency ordering, process start/stop via Windows Job
Objects, labelled concurrent logging, and Ctrl+C shutdown are all in
place and tested. Phase 5 (documentation, examples, release polish) is
in progress.

## Install

Prerequisite: Go 1.26 or later, on Windows.

**Option 1 — `go install` (recommended if you have Go):**

```powershell
go install github.com/Sloane-J/anchor/cmd/anchor@latest
```

This builds and places `anchor.exe` in your Go bin directory
(`%USERPROFILE%\go\bin` by default), which is usually already on your
PATH. Confirm with:

```powershell
anchor --version
```

**Option 2 — build from source manually:**

```powershell
git clone https://github.com/Sloane-J/anchor.git
cd anchor
go build -o anchor.exe .\cmd\anchor
```

Then either run `.\anchor.exe` directly, or copy `anchor.exe` into a
folder already on your PATH so you can run `anchor` from anywhere.

## Build and test (for contributors)

```powershell
go test ./...
go vet ./...
go run ./cmd/anchor --help
```

## Quick start

1. Copy [`examples/dev.yaml`](examples/dev.yaml) into your project root
   as `dev.yaml` and adjust the services and working directories to match
   your repository.
2. Run:

```powershell
   go run ./cmd/anchor start
```

   Or, from a `dev.yaml` in a different location:

```powershell
   go run ./cmd/anchor start --file path\to\dev.yaml
```

3. Services start in dependency order. Output is prefixed with a
   timestamp, service name, and stream (`OUT`/`ERR`).
4. Press **Ctrl+C** to stop every started service, in reverse startup
   order.
5. From another terminal, run `anchor status` (or `anchor status --file
   path\to\dev.yaml`) to see which services are currently running,
   without needing to watch the `anchor start` terminal.

## `dev.yaml` format

```yaml
version: 1

services:
  database:
    command: docker
    args: ["compose", "up"]

  api:
    command: npm
    args: ["run", "dev"]
    working_dir: ./apps/api
    depends_on: [database]

  web:
    command: pnpm
    args: ["dev"]
    working_dir: ./apps/web
    depends_on: [api]
```

- `command` / `args` — the executable and its arguments, run directly
  (never through a shell).
- `working_dir` — relative to the `dev.yaml` file, unless absolute.
- `depends_on` — services that must be started before this one starts.

See [`examples/dev.yaml`](examples/dev.yaml) for a fuller example
covering npm, yarn, pnpm, bun, Go, Docker Compose, and Python.

Invalid configuration (unknown fields, missing commands, unresolvable
working directories, unknown or cyclic dependencies) is rejected before
any process is started, with an error identifying the affected service.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Clean shutdown |
| `1` | Configuration or user error |
| `2` | Runtime or startup failure |
| `130` | Interrupted (Ctrl+C) after shutdown handling |

## Current limitations (v0.1)

- Windows only. Linux/WSL support is planned but not yet implemented.
- No `anchor stop` or `anchor logs` commands — `anchor start` runs
- `anchor status` relies on a state file that is removed on clean shutdown; if a session crashes instead of shutting down normally, `anchor status` will flag the state as stale rather than trusting it blindly.
  in the foreground until it exits or is interrupted.
- No automatic restart or health checks.
- Commands are run directly, not through a shell — features that rely on
  shell syntax (pipes, `&&`, environment expansion in the command itself)
  are not supported unless the command you specify is itself a shell
  (e.g. `command: cmd`, `args: ["/c", "..."]`).