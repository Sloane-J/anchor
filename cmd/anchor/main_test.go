package main

import (
	"bytes"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if got := run([]string{"--help"}, &stdout, &stderr); got != 0 {
		t.Fatalf("run(--help) exit code = %d, want 0", got)
	}
	if stdout.Len() == 0 {
		t.Fatal("run(--help) did not write usage")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if got := run([]string{"unknown"}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(unknown) exit code = %d, want 1", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("run(unknown) did not explain the error")
	}
}
