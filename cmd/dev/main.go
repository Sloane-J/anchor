// Command dev starts and supervises local development services.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `Dev Orchestrator starts local development services from dev.yaml.

Usage:
  dev start [--file path]  Start configured services (coming in v0.1)
  dev --help               Show this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	}

	_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n%s", args[0], usage)
	return 1
}
