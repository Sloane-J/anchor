// Command anchor starts and supervises local development services.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Sloane-J/anchor/internal/config"
	"github.com/Sloane-J/anchor/internal/dependency"
	"github.com/Sloane-J/anchor/internal/logger"
	"github.com/Sloane-J/anchor/internal/process"
	"github.com/Sloane-J/anchor/internal/signals"
	"github.com/Sloane-J/anchor/internal/statefile"
	"github.com/Sloane-J/anchor/internal/supervisor"
	"github.com/Sloane-J/anchor/internal/validator"
)

// version is the current release version. Bump it for each release.
const version = "0.1.1"

const usage = `Anchor starts local development services from dev.yaml.

Usage:
  anchor start [--file path] [--debug]  Start configured services in dependency order
  anchor status [--file path]           Show the status of a running (or last known) session
  anchor --version                      Show version
  anchor --help                         Show this help

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

// ANSI colours for CLI-level status messages (distinct from per-service
// log line colours in internal/logger).
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiGray   = "\x1b[90m"
	ansiCyan   = "\x1b[36m"
)

func colorize(color, text string) string {
	return color + text + ansiReset
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, _ = fmt.Fprint(stdout, usage)
		return exitOK
	}

	if args[0] == "--version" || args[0] == "-v" {
		_, _ = fmt.Fprintf(stdout, "anchor %s\n", version)
		return exitOK
	}

	switch args[0] {
	case "start":
		return runStart(args[1:], stdout, stderr)
	case "status":
		return runStatus(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n%s", args[0], usage)
		return exitConfigError
	}
}

func parseFileFlag(args []string) string {
	path := "dev.yaml"
	for i := 0; i < len(args); i++ {
		if args[i] == "--file" && i+1 < len(args) {
			path = args[i+1]
		}
	}
	return path
}

func runStart(args []string, stdout, stderr io.Writer) int {
	path := parseFileFlag(args)
	debug := false
	for _, a := range args {
		if a == "--debug" {
			debug = true
		}
	}

	reportErr := func(err error) {
		if debug {
			_, _ = fmt.Fprintf(stderr, "%s %v\n\ndebug: full error chain:\n", colorize(ansiRed, "error:"), err)
			for u := err; u != nil; u = errors.Unwrap(u) {
				_, _ = fmt.Fprintf(stderr, "  - %v\n", u)
			}
			return
		}
		_, _ = fmt.Fprintf(stderr, "%s %v\n", colorize(ansiRed, "error:"), err)
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

	_, _ = fmt.Fprintf(stdout, "%s %s: starting %d service(s) from %s\n",
		"🚀", colorize(ansiBold+ansiCyan, "Anchor"), len(plan.Startup), filepath.Base(cfg.SourcePath))

	startTime := time.Now()
	succeeded := 0

	// Initialize the state file so `anchor status` has something to read
	// from the moment services start being tracked, even before the
	// first StatusEvent arrives.
	sessionState := statefile.State{
		PID:        os.Getpid(),
		ConfigPath: cfg.SourcePath,
		StartedAt:  startTime,
		Services:   make(map[string]statefile.ServiceStatus, len(specs)),
	}
	for name := range specs {
		sessionState.Services[name] = statefile.ServiceStatus{State: "pending", Since: startTime}
	}
	_ = statefile.Write(cfg.SourcePath, sessionState)
	defer func() { _ = statefile.Remove(cfg.SourcePath) }()

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
			if ev.State == supervisor.Running {
				succeeded++
			}
			sessionState.Services[ev.Name] = statefile.ServiceStatus{
				State: ev.State.String(),
				Since: time.Now(),
			}
			_ = statefile.Write(cfg.SourcePath, sessionState)
			reportStatus(stdout, ev)
		case err := <-done:
			return finish(stdout, stderr, ctx, err, debug, reportErr, succeeded, len(plan.Startup), time.Since(startTime))
		}
	}
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	path := parseFileFlag(args)

	cfg, err := config.Load(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s %v\n", colorize(ansiRed, "error:"), err)
		return exitConfigError
	}

	state, ok, err := statefile.Read(cfg.SourcePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s %v\n", colorize(ansiRed, "error:"), err)
		return exitRuntimeFailure
	}
	if !ok {
		_, _ = fmt.Fprintf(stdout, "%s No session running for %s\n", colorize(ansiGray, "○"), filepath.Base(cfg.SourcePath))
		return exitOK
	}

	alive := process.IsAlive(state.PID)
	if !alive {
		_, _ = fmt.Fprintf(stdout, "%s Stale state found for %s (process %d is not running; last known state below may be inaccurate)\n\n",
			colorize(ansiYellow, "⚠️ "), filepath.Base(cfg.SourcePath), state.PID)
	} else {
		_, _ = fmt.Fprintf(stdout, "%s Session running for %s (pid %d, up %s)\n\n",
			colorize(ansiGreen, "●"), filepath.Base(cfg.SourcePath), state.PID, time.Since(state.StartedAt).Round(time.Second))
	}

	for name, svc := range state.Services {
		var icon, color string
		switch svc.State {
		case "running":
			icon, color = "✅", ansiGreen
		case "starting", "pending":
			icon, color = "⚙️ ", ansiYellow
		case "stopped":
			icon, color = "🛑", ansiGray
		case "failed":
			icon, color = "❌", ansiRed
		default:
			icon, color = "?", ansiGray
		}
		_, _ = fmt.Fprintf(stdout, "  %s %-12s %s\n", icon, name, colorize(color, svc.State))
	}

	return exitOK
}

func finish(stdout, stderr io.Writer, ctx context.Context, err error, debug bool, reportErr func(error), succeeded, total int, elapsed time.Duration) int {
	summary := func() {
		_, _ = fmt.Fprintf(stdout, "\n  %s  %d successful, %d total\n  %s     %s\n",
			colorize(ansiGray, "Services:"), succeeded, total,
			colorize(ansiGray, "Uptime:"), elapsed.Round(100*time.Millisecond))
	}

	if err == nil {
		summary()
		_, _ = fmt.Fprintf(stdout, "%s Clean shutdown.\n", colorize(ansiGreen, "✅"))
		return exitOK
	}

	if ctx.Err() != nil {
		summary()
		_, _ = fmt.Fprintf(stdout, "%s Clean shutdown. 👋\n", colorize(ansiGreen, "✅"))
		return exitInterrupted
	}

	reportErr(err)
	summary()
	return exitRuntimeFailure
}

func reportStatus(w io.Writer, ev supervisor.StatusEvent) {
	switch ev.State {
	case supervisor.Starting:
		_, _ = fmt.Fprintf(w, "  %s %-12s %s\n", colorize(ansiYellow, "⚙️ "), ev.Name, colorize(ansiYellow, "starting"))
	case supervisor.Running:
		_, _ = fmt.Fprintf(w, "  %s %-12s %s\n", colorize(ansiGreen, "✅"), ev.Name, colorize(ansiGreen, "running"))
	case supervisor.Stopped:
		_, _ = fmt.Fprintf(w, "  %s %-12s %s\n", colorize(ansiGray, "🛑"), ev.Name, colorize(ansiGray, "stopped"))
	case supervisor.Failed:
		_, _ = fmt.Fprintf(w, "  %s %-12s %s: %v\n", colorize(ansiRed, "❌"), ev.Name, colorize(ansiRed, "failed"), ev.Err)
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