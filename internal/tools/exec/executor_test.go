package exec

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCommandExecutor_Run(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		cmd       string
		args      []string
		wantError bool
	}{
		{
			name:      "successful command",
			cmd:       "echo",
			args:      []string{"test"},
			wantError: false,
		},
		{
			name:      "failing command",
			cmd:       "false",
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(ctx, tt.cmd, tt.args...).Run()
			if (err != nil) != tt.wantError {
				t.Errorf("Run() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCommandExecutor_Output(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		cmd        string
		args       []string
		wantOutput string
		wantError  bool
	}{
		{
			name:       "capture output",
			cmd:        "echo",
			args:       []string{"hello world"},
			wantOutput: "hello world",
			wantError:  false,
		},
		{
			name:       "command not found",
			cmd:        "nonexistent_command_12345",
			args:       []string{},
			wantOutput: "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := New(ctx, tt.cmd, tt.args...).Output()
			if (err != nil) != tt.wantError {
				t.Errorf("Output() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && output != tt.wantOutput {
				t.Errorf("Output() = %q, want %q", output, tt.wantOutput)
			}
		})
	}
}

func TestCommandExecutor_OutputStdout(t *testing.T) {
	ctx := context.Background()

	// Test that stdout is captured and stderr is discarded
	output, err := New(ctx, "sh", "-c", "echo stdout; echo stderr >&2").OutputStdout()
	if err != nil {
		t.Fatalf("OutputStdout() error = %v", err)
	}
	if output != "stdout" {
		t.Errorf("OutputStdout() = %q, want %q", output, "stdout")
	}
}

func TestCommandExecutor_WithStdout(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer

	err := New(ctx, "echo", "test").WithStdout(&buf).Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "test" {
		t.Errorf("captured output = %q, want %q", output, "test")
	}
}

func TestCommandExecutor_WithEnv(t *testing.T) {
	ctx := context.Background()

	output, err := New(ctx, "sh", "-c", "echo $TEST_VAR").
		WithEnv([]string{"TEST_VAR=hello"}).
		OutputStdout()

	if err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if output != "hello" {
		t.Errorf("Output() = %q, want %q", output, "hello")
	}
}

func TestConvenienceFunctions(t *testing.T) {
	ctx := context.Background()

	t.Run("Run", func(t *testing.T) {
		err := Run(ctx, "echo", "test")
		if err != nil {
			t.Errorf("Run() error = %v", err)
		}
	})

	t.Run("Output", func(t *testing.T) {
		output, err := Output(ctx, "echo", "test")
		if err != nil {
			t.Errorf("Output() error = %v", err)
		}
		if output != "test" {
			t.Errorf("Output() = %q, want %q", output, "test")
		}
	})

	t.Run("OutputStdout", func(t *testing.T) {
		output, err := OutputStdout(ctx, "echo", "test")
		if err != nil {
			t.Errorf("OutputStdout() error = %v", err)
		}
		if output != "test" {
			t.Errorf("OutputStdout() = %q, want %q", output, "test")
		}
	})

	t.Run("Check", func(t *testing.T) {
		if !Check(ctx, "echo", "test") {
			t.Error("Check() = false, want true")
		}
		if Check(ctx, "false") {
			t.Error("Check() = true, want false")
		}
	})
}
