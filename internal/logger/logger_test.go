package logger

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func fixedClock(t time.Time) Clock {
	return func() time.Time { return t }
}

func TestWriterLabelsCompleteLines(t *testing.T) {
	var buf bytes.Buffer
	clock := fixedClock(time.Date(2026, 1, 1, 15, 4, 5, 0, time.UTC))
	mux := New(&buf, clock)

	w := mux.Writer("api", Stdout)
	if _, err := w.Write([]byte("server listening\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "api") {
		t.Errorf("output %q does not contain service name", got)
	}
	if !strings.Contains(got, "OUT") {
		t.Errorf("output %q does not contain stream label", got)
	}
	if !strings.Contains(got, "server listening") {
		t.Errorf("output %q does not contain the line content", got)
	}
	if !strings.Contains(got, "15:04:05") {
		t.Errorf("output %q does not contain the timestamp", got)
	}
}

func TestWriterBuffersPartialLines(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))
	w := mux.Writer("api", Stdout)

	// Write a line in two pieces with no newline in the first piece.
	if _, err := w.Write([]byte("hello ")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("output before newline = %q, want empty (should still be buffered)", buf.String())
	}

	if _, err := w.Write([]byte("world\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("output = %q, want it to contain %q", buf.String(), "hello world")
	}
}

func TestWriterEmitsOneLinePerNewline(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))
	w := mux.Writer("api", Stdout)

	if _, err := w.Write([]byte("line one\nline two\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2 (output: %q)", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "line one") {
		t.Errorf("first line = %q, want it to contain %q", lines[0], "line one")
	}
	if !strings.Contains(lines[1], "line two") {
		t.Errorf("second line = %q, want it to contain %q", lines[1], "line two")
	}
}

func TestWriterTrimsCarriageReturn(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))
	w := mux.Writer("api", Stdout)

	// Windows-style line ending.
	if _, err := w.Write([]byte("hello\r\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if strings.Contains(buf.String(), "\r") {
		t.Fatalf("output %q still contains a carriage return", buf.String())
	}
}

func TestCloseFlushesTrailingPartialLine(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))
	w := mux.Writer("api", Stderr)

	// No trailing newline: the process exited mid-line.
	if _, err := w.Write([]byte("incomplete")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("output before Close = %q, want empty", buf.String())
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !strings.Contains(buf.String(), "incomplete") {
		t.Fatalf("output after Close = %q, want it to contain %q", buf.String(), "incomplete")
	}
	if !strings.Contains(buf.String(), "ERR") {
		t.Fatalf("output after Close = %q, want ERR stream label", buf.String())
	}
}

func TestCloseIsSafeWithNoBufferedData(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))
	w := mux.Writer("api", Stdout)

	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("output = %q, want empty (nothing was buffered)", buf.String())
	}
}

func TestConcurrentWritersDoNotInterleaveMidLine(t *testing.T) {
	var buf bytes.Buffer
	mux := New(&buf, fixedClock(time.Now()))

	wApi := mux.Writer("api", Stdout)
	wWeb := mux.Writer("web", Stdout)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			wApi.Write([]byte("api line\n"))
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 50; i++ {
			wWeb.Write([]byte("web line\n"))
		}
		done <- struct{}{}
	}()
	<-done
	<-done

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 100 {
		t.Fatalf("got %d lines, want 100 (a corrupted/merged line would produce fewer)", len(lines))
	}
	for _, line := range lines {
		hasApi := strings.Contains(line, "api line")
		hasWeb := strings.Contains(line, "web line")
		if hasApi == hasWeb {
			t.Fatalf("line %q is corrupted (should contain exactly one of api/web content)", line)
		}
	}
}