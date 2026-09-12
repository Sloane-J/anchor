// Command dev starts and supervises local development services.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dev-orchestrator/internal/config"
	"dev-orchestrator/internal/dependency"
	"dev-orchestrator/internal/logger"
	"dev-orchestrator/internal/process"
	"dev-orchestrator/internal/signals"
	"dev-orchestrator/internal/supervisor"
	"dev-orchestrator/internal/validator"
)

// version is the current release version. Bump it for each release.
const version = "0.1.0"

const usage = `Dev Orchestrator starts local development services from dev.yaml.

Usage:
  dev start [--file path] [--debug]  Start configured services in dependency order
  dev --version                      Show version
  dev --help                         Show this help

Exit codes:
  0    clean shutdown
  1    configuration or user error
  2    runtime or startup failure
  130  interrupted (Ctrl+C) after shutdown handling
`

// Exit codes per ARCHITECTURE.md's UX contract.
const (
	exitOK             = 0
	exitConfigError    = 1
	exitRuntimeFailure = 2
	exitInterrupted    = 130
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, _ = fmt.Fprint(stdout, usage)
		return exitOK
	}

	if args[0] == "--version" || args[0] == "-v" {
		_, _ = fmt.Fprintf(stdout, "dev-orchestrator %s\n", version)
		return exitOK
	}

	switch args[0] {
	case "start":
		return runStart(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n%s", args[0], usage)
		return exitConfigError
	}
}

func runStart(args []string, stdout, stderr io.Writer) int {
	path := "dev.yaml"
	debug := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 < len(args) {
				path = args[i+1]
				i++
			}
		case "--debug":
			debug = true
		}
	}

	reportErr := func(err error) {
		if debug {
			_, _ = fmt.Fprintf(stderr, "error: %v\n\ndebug: full error chain:\n", err)
			for u := err; u != nil; u = errors.Unwrap(u) {
				_, _ = fmt.Fprintf(stderr, "  - %v\n", u)
			}
			return
		}
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		reportErr(err)
		return exitConfigError
	}

	if err := validator.Validate(cfg); err != nil {
		reportErr(err)
		return exitConfigError
	}

	plan, err := dependency.Build(cfg)
	if err != nil {
		reportErr(err)
		return exitConfigError
	}

	mux := logger.New(stdout, nil)

	specs := make(map[string]supervisor.ServiceSpec, len(cfg.Services))
	for name, svc := range cfg.Services {
		specs[name] = supervisor.ServiceSpec{
			Name:    name,
			Command: svc.Command,
			Args:    svc.Args,
			Dir:     resolveDir(cfg, svc),
			Stdout:  mux.Writer(name, logger.Stdout),
			Stderr:  mux.Writer(name, logger.Stderr),
		}
	}

	sup := supervisor.New(process.NewRunner(), specs)

	ctx, stop := signals.WithCancelOnInterrupt(context.Background())
	defer stop()

	_, _ = fmt.Fprintf(stdout, "starting %d service(s) from %s\n", len(plan.Startup), cfg.SourcePath)

	events := make(chan supervisor.StatusEvent, 32)
	done := make(chan error, 1)
	go func() {
		done <- sup.Run(ctx, plan, events)
	}()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			reportStatus(stdout, ev)
		case err := <-done:
			return finish(stdout, stderr, ctx, err, debug, reportErr)
		}
	}
}

func finish(stdout, stderr io.Writer, ctx context.Context, err error, debug bool, reportErr func(error)) int {
	if err == nil {
		_, _ = fmt.Fprintln(stdout, "all services stopped")
		return exitOK
	}

	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(stdout, "shutdown complete")
		return exitInterrupted
	}

	reportErr(err)
	return exitRuntimeFailure
}

func reportStatus(w io.Writer, ev supervisor.StatusEvent) {
	switch ev.State {
	case supervisor.Starting:
		_, _ = fmt.Fprintf(w, "-- %s: starting\n", ev.Name)
	case supervisor.Running:
		_, _ = fmt.Fprintf(w, "-- %s: running\n", ev.Name)
	case supervisor.Stopped:
		_, _ = fmt.Fprintf(w, "-- %s: stopped\n", ev.Name)
	case supervisor.Failed:
		_, _ = fmt.Fprintf(w, "-- %s: failed: %v\n", ev.Name, ev.Err)
	}
}

func resolveDir(cfg config.Config, svc config.Service) string {
	baseDir := "."
	if cfg.SourcePath != "" {
		baseDir = filepath.Dir(cfg.SourcePath)
	}
	if svc.WorkingDir == "" {
		return baseDir
	}
	if filepath.IsAbs(svc.WorkingDir) {
		return svc.WorkingDir
	}
	return filepath.Join(baseDir, svc.WorkingDir)
}