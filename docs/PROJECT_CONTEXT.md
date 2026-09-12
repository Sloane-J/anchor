# Anchor: project context

## Problem

Local applications often require dependent commands: a database, API, web client, worker, or mock service. Developers otherwise open multiple terminals, remember ordering, and manually stop everything. Anchor provides one repeatable local command to run that stack.

## Audience

Developers working on Windows, especially repositories with several local services. The CLI must remain usable in PowerShell, Command Prompt, and IDE terminals. Cross-platform support is a future compatibility goal, not a v0.1 promise.

## v0.1 success statement

Given a valid `dev.yaml`, a developer can run `anchor start`, see services start in dependency order with labelled logs, and press Ctrl+C to stop all child processes cleanly. Invalid configuration produces a clear error and starts nothing.

## Non-goals for v0.1

- production supervision or deployment;
- Docker or Kubernetes orchestration;
- automatic restart, health checks, file watching, plugins, TUI, metrics, or a VS Code extension;
- elevated privileges or modification of machine-level settings.

## Design constraints

- Go CLI, Windows-first.
- Low resource usage: suitable for 8 GB RAM and an HDD.
- Direct child-process execution; no implicit shell.
- Repository-local, human-editable configuration.
- Domain packages testable without real child processes.

## Proposed minimal configuration

```yaml
version: 1

services:
  redis:
    command: redis-server

  api:
    command: go
    args: ["run", "."]
    depends_on: [redis]

  web:
    command: npm
    args: ["run", "dev"]
