# Windows manual test checklist

Automated tests cover unit and integration behaviour with fakes and
harmless real processes (`ping`, `cmd /c exit N`). This checklist covers
real-world scenarios that are impractical to automate but matter before
a release.

## Setup

- [ ] Build a fresh binary: `go build -o anchor.exe ./cmd/anchor`
- [ ] Have at least one real Node.js project available with an npm or
      pnpm dev script, to test against a real long-running dev server.

## Basic flow

- [ ] `anchor.exe --help` prints usage and exits `0`.
- [ ] `anchor.exe start` with no `dev.yaml` in the current directory prints
      a clear, actionable error and exits `1`.
- [ ] `anchor.exe start --file path\to\dev.yaml` loads a config from a
      non-default location.

## Configuration validation

- [ ] A `dev.yaml` with an unknown top-level field is rejected with a
      clear error, before anything starts.
- [ ] A `dev.yaml` with a service depending on an unknown service name
      is rejected with a clear error naming both services.
- [ ] A `dev.yaml` with a dependency cycle (A → B → A) is rejected with
      a readable cycle path.
- [ ] A `dev.yaml` with an invalid `working_dir` (doesn't exist) is
      rejected before any process starts.

## Real process behaviour

- [ ] Start a config with 2–3 real services (e.g. one npm dev server,
      one `docker compose up`). Confirm all start in the declared
      dependency order (check the log timestamps).
- [ ] Confirm labelled output from each service appears correctly and is
      not garbled or interleaved mid-line, even under real concurrent
      output (e.g. a noisy webpack dev server).
- [ ] Press **Ctrl+C** while all services are running. Confirm:
  - [ ] All services report `stopped` in the terminal output.
  - [ ] Exit code is `130`.
  - [ ] Open Task Manager (or `tasklist`) and confirm no child processes
        remain — especially for tools like `npm`/`pnpm` that spawn a
        `node.exe` child. This is the Job Object kill-tree guarantee;
        an orphaned `node.exe` here is a release blocker.
- [ ] Press **Ctrl+Break** instead of Ctrl+C (if your terminal supports
      it) and confirm the same clean shutdown behaviour.
- [ ] Kill one service's process directly from Task Manager while
      `anchor start` is running (simulating a crash). Confirm:
  - [ ] `anchor start` detects the exit and reports it as failed.
  - [ ] Every other running service is stopped in reverse order.
  - [ ] `anchor.exe` itself exits with code `2`.

## Terminal compatibility

- [ ] Run `anchor start` in PowerShell.
- [ ] Run `anchor start` in Command Prompt (`cmd.exe`).
- [ ] Run `anchor start` in an IDE-integrated terminal (VS Code or similar).
- [ ] Run `anchor start` in Git Bash. Note: Git Bash may reinterpret certain
      argument forms (e.g. leading single-slash flags like `/c`) — if a
      command behaves unexpectedly only in Git Bash, retest the same
      `dev.yaml` from PowerShell before treating it as a bug in Dev
      Orchestrator itself.

## Long-running stability

- [ ] Leave a multi-service stack running for at least 15 minutes.
      Confirm output continues to stream correctly and no service is
      unexpectedly marked failed or stopped.