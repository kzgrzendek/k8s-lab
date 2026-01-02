// Package exec provides utilities for executing external commands with consistent error handling.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// CommandExecutor provides a structured way to execute external commands.
type CommandExecutor struct {
	ctx    context.Context
	name   string
	args   []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	env    []string
}

// New creates a new CommandExecutor.
func New(ctx context.Context, name string, args ...string) *CommandExecutor {
	return &CommandExecutor{
		ctx:  ctx,
		name: name,
		args: args,
	}
}

// WithStdin sets the stdin for the command.
func (c *CommandExecutor) WithStdin(r io.Reader) *CommandExecutor {
	c.stdin = r
	return c
}

// WithStdout sets the stdout for the command.
func (c *CommandExecutor) WithStdout(w io.Writer) *CommandExecutor {
	c.stdout = w
	return c
}

// WithStderr sets the stderr for the command.
func (c *CommandExecutor) WithStderr(w io.Writer) *CommandExecutor {
	c.stderr = w
	return c
}

// WithEnv sets environment variables for the command.
func (c *CommandExecutor) WithEnv(env []string) *CommandExecutor {
	c.env = env
	return c
}

// Run executes the command and returns an error if it fails.
// Output is discarded unless WithStdout/WithStderr is used.
func (c *CommandExecutor) Run() error {
	cmd := c.buildCmd()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}
	return nil
}

// Output executes the command and returns its combined stdout and stderr as a string.
// This is useful when you need to capture the command output.
func (c *CommandExecutor) Output() (string, error) {
	cmd := c.buildCmd()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(buf.String()), fmt.Errorf("command failed: %s %s: %w\nOutput: %s",
			c.name, strings.Join(c.args, " "), err, buf.String())
	}
	return strings.TrimSpace(buf.String()), nil
}

// OutputStdout executes the command and returns only stdout as a string.
// stderr is discarded unless WithStderr is used.
func (c *CommandExecutor) OutputStdout() (string, error) {
	cmd := c.buildCmd()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if c.stderr == nil {
		cmd.Stderr = io.Discard
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %s %s: %w", c.name, strings.Join(c.args, " "), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// MustRun executes the command and panics if it fails.
// This should only be used in initialization code where failure is unrecoverable.
func (c *CommandExecutor) MustRun() {
	if err := c.Run(); err != nil {
		panic(err)
	}
}

// MustOutput executes the command and returns its output, panicking if it fails.
// This should only be used in initialization code where failure is unrecoverable.
func (c *CommandExecutor) MustOutput() string {
	output, err := c.Output()
	if err != nil {
		panic(err)
	}
	return output
}

// buildCmd builds the exec.Cmd with all configured options.
func (c *CommandExecutor) buildCmd() *exec.Cmd {
	cmd := exec.CommandContext(c.ctx, c.name, c.args...)

	if c.stdin != nil {
		cmd.Stdin = c.stdin
	}
	if c.stdout != nil {
		cmd.Stdout = c.stdout
	}
	if c.stderr != nil {
		cmd.Stderr = c.stderr
	}
	if c.env != nil {
		cmd.Env = append(os.Environ(), c.env...)
	}

	return cmd
}

// --- Convenience Functions ---

// Run executes a command and returns an error if it fails.
// This is a convenience function for simple command execution.
func Run(ctx context.Context, name string, args ...string) error {
	return New(ctx, name, args...).Run()
}

// Output executes a command and returns its combined output as a string.
// This is a convenience function for capturing command output.
func Output(ctx context.Context, name string, args ...string) (string, error) {
	return New(ctx, name, args...).Output()
}

// OutputStdout executes a command and returns only stdout as a string.
// This is a convenience function for capturing command stdout only.
func OutputStdout(ctx context.Context, name string, args ...string) (string, error) {
	return New(ctx, name, args...).OutputStdout()
}

// Check executes a command and returns whether it succeeded (true) or failed (false).
// This is useful for checking if a tool is available or a condition is met.
func Check(ctx context.Context, name string, args ...string) bool {
	return New(ctx, name, args...).Run() == nil
}

// Interactive executes a command with stdin/stdout/stderr connected to the current process.
// This is useful for interactive commands that need user input.
func Interactive(ctx context.Context, name string, args ...string) error {
	return New(ctx, name, args...).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Run()
}

// RunWithEphemeralOutput executes a command with ephemeral output that disappears after completion.
// The output is shown in real-time (dimmed/grayed) but cleared when done - similar to Docker build.
// Only the final status remains visible.
func (c *CommandExecutor) RunWithEphemeralOutput(writer io.Writer) error {
	cmd := c.buildCmd()

	// Pipe both stdout and stderr to the ephemeral writer
	cmd.Stdout = writer
	cmd.Stderr = writer

	return cmd.Run()
}
