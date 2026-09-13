// Package logger: ANSI colour helpers for terminal output.
package logger

import "fmt"

// ansi codes. These are widely supported on Windows Terminal, VS Code's
// terminal, and modern PowerShell/cmd.exe. If the output isn't a real
// terminal (e.g. redirected to a file), colour codes are harmless but
// pointless; v0.1 does not attempt terminal detection and always emits
// them, matching most modern CLI tools' default behaviour.
const (
	ansiReset   = "\x1b[0m"
	ansiCyan    = "\x1b[36m"
	ansiMagenta = "\x1b[35m"
	ansiYellow  = "\x1b[33m"
	ansiGreen   = "\x1b[32m"
	ansiBlue    = "\x1b[34m"
	ansiRed     = "\x1b[31m"
	ansiGray    = "\x1b[90m"
)

// servicePalette cycles distinct colours across services by order of first
// appearance, so each service keeps one consistent colour for the whole
// run (the same approach docker compose and foreman use).
var servicePalette = []string{ansiCyan, ansiMagenta, ansiYellow, ansiGreen, ansiBlue, ansiRed}

// ServiceColorer assigns a stable colour to each service name, in the
// order names are first seen.
type ServiceColorer struct {
	assigned map[string]string
	next     int
}

// NewServiceColorer returns an empty ServiceColorer.
func NewServiceColorer() *ServiceColorer {
	return &ServiceColorer{assigned: make(map[string]string)}
}

// Color returns the ANSI colour code for name, assigning the next unused
// palette colour the first time name is seen.
func (c *ServiceColorer) Color(name string) string {
	if color, ok := c.assigned[name]; ok {
		return color
	}
	color := servicePalette[c.next%len(servicePalette)]
	c.assigned[name] = color
	c.next++
	return color
}

// Colorize wraps text in the given ANSI colour, resetting afterward.
func Colorize(color, text string) string {
	return fmt.Sprintf("%s%s%s", color, text, ansiReset)
}