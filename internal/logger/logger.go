// Package logger formats and multiplexes labelled process output.
//
// Each service's stdout and stderr is wrapped in a Writer that prefixes
// every line with a timestamp, the service name, and the stream (OUT or
// ERR), per ARCHITECTURE.md's UX contract. Writers are safe for concurrent
// use by multiple services writing at once. Each service is assigned a
// stable colour for the duration of a run, so its lines are visually
// distinct from other services'.
package logger

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// Clock returns the current time. It exists so tests can inject a fixed
// time instead of depending on the real clock.
type Clock func() time.Time

// Multiplexer writes labelled lines from any number of services to a
// single underlying writer, serializing concurrent writes so lines from
// different services are never interleaved mid-line.
type Multiplexer struct {
	out     io.Writer
	clock   Clock
	colorer *ServiceColorer

	mu sync.Mutex
}

// New returns a Multiplexer that writes labelled lines to out. If clock is
// nil, time.Now is used.
func New(out io.Writer, clock Clock) *Multiplexer {
	if clock == nil {
		clock = time.Now
	}
	return &Multiplexer{out: out, clock: clock, colorer: NewServiceColorer()}
}

// Stream identifies which output stream a line came from.
type Stream string

const (
	Stdout Stream = "OUT"
	Stderr Stream = "ERR"
)

// Writer returns a LineWriter that labels every line written to it with
// the given service name and stream, then forwards it to the
// Multiplexer's underlying writer.
func (m *Multiplexer) Writer(service string, stream Stream) *LineWriter {
	return &LineWriter{
		mux:     m,
		service: service,
		stream:  stream,
	}
}

func (m *Multiplexer) writeLine(service string, stream Stream, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timestamp := m.clock().Format("15:04:05.000")
	color := m.colorer.Color(service)
	label := Colorize(color, fmt.Sprintf("%-12s", service))
	streamLabel := string(stream)
	if stream == Stderr {
		streamLabel = Colorize(ansiRed, streamLabel)
	} else {
		streamLabel = Colorize(ansiGray, streamLabel)
	}
	fmt.Fprintf(m.out, "%s %s %s %s\n", timestamp, label, streamLabel, line)
}

// LineWriter accumulates bytes written to it and emits one labelled line
// per newline. A process's stdout/stderr may write partial lines across
// multiple Write calls; LineWriter buffers until it sees '\n'.
type LineWriter struct {
	mux     *Multiplexer
	service string
	stream  Stream

	mu  sync.Mutex
	buf []byte
}

// Write implements io.Writer.
func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)

	for {
		idx := indexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := trimCR(string(w.buf[:idx]))
		w.mux.writeLine(w.service, w.stream, line)
		w.buf = w.buf[idx+1:]
	}

	return len(p), nil
}

// Close flushes any buffered partial line (one without a trailing
// newline) that was never terminated before the process exited.
func (w *LineWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.buf) > 0 {
		w.mux.writeLine(w.service, w.stream, trimCR(string(w.buf)))
		w.buf = nil
	}
	return nil
}

func indexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

var _ io.Writer = (*LineWriter)(nil)