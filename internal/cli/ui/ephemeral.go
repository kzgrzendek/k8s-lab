package ui

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
)

// EphemeralWriter is a writer that displays output temporarily and clears it when done.
// Similar to Docker build output - shows progress but doesn't clutter the terminal.
type EphemeralWriter struct {
	mu          sync.Mutex
	lines       []string
	maxLines    int
	clearOnDone bool
}

// NewEphemeralWriter creates a new ephemeral writer.
// maxLines controls how many lines to keep visible (0 = unlimited).
func NewEphemeralWriter(maxLines int) *EphemeralWriter {
	return &EphemeralWriter{
		lines:       make([]string, 0),
		maxLines:    maxLines,
		clearOnDone: true,
	}
}

// Write implements io.Writer interface.
func (e *EphemeralWriter) Write(p []byte) (n int, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	scanner := bufio.NewScanner(strings.NewReader(string(p)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Add line to buffer
		e.lines = append(e.lines, line)

		// Limit buffer size if maxLines is set
		if e.maxLines > 0 && len(e.lines) > e.maxLines {
			e.lines = e.lines[len(e.lines)-e.maxLines:]
		}

		// Clear previous lines and redraw
		e.redraw()
	}

	return len(p), nil
}

// redraw clears the terminal and redraws visible lines.
func (e *EphemeralWriter) redraw() {
	// Move cursor up and clear lines for all visible content
	if len(e.lines) > 1 {
		fmt.Printf("\033[%dA", len(e.lines)-1) // Move up
	}

	// Clear from cursor to end of screen
	fmt.Print("\033[J")

	// Print all lines in dim/gray color
	for _, line := range e.lines {
		fmt.Printf("%s\n", dimColor(line))
	}
}

// Clear removes all ephemeral output from the terminal.
func (e *EphemeralWriter) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.lines) == 0 {
		return
	}

	// Move cursor up to the first line
	if len(e.lines) > 0 {
		fmt.Printf("\033[%dA", len(e.lines))
	}

	// Clear from cursor to end of screen
	fmt.Print("\033[J")

	// Clear the buffer
	e.lines = make([]string, 0)
}

// Done marks the ephemeral output as complete and clears it.
func (e *EphemeralWriter) Done() {
	if e.clearOnDone {
		e.Clear()
	}
}

// KeepOnDone prevents the output from being cleared when Done() is called.
// Useful for keeping error output visible.
func (e *EphemeralWriter) KeepOnDone() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clearOnDone = false
}

// PipeWriter creates an io.Writer that can be used with command output.
// Call Done() when the command completes to clear the output.
func PipeWriter() *EphemeralWriter {
	return NewEphemeralWriter(20) // Show last 20 lines max
}
