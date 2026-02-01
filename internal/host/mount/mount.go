// Package mount provides management for minikube mount operations.
// This replaces NFS for mounting host directories into minikube nodes,
// providing a simpler and more reliable solution for model storage.
package mount

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
)

// mountProcess stores the minikube mount process for lifecycle management.
// This is a package-level variable following the pattern used in registry.go.
var mountProcess *os.Process

// Start starts a minikube mount in the background.
// The mount makes the host models directory available to all minikube nodes
// at the configured mount point (/mnt/nova/models).
//
// This follows the same pattern as registry.Start() and nfs.Start().
func Start(ctx context.Context, cfg *config.Config) error {
	// Idempotence: check if already running
	running, err := IsRunning(ctx)
	if err != nil {
		ui.Debug("Failed to check mount status: %v", err)
	}
	if running {
		ui.Debug("Minikube mount already running")
		return nil
	}

	// Prepare the source directory (host side)
	modelsPath := cfg.GetModelsPath()
	if err := os.MkdirAll(modelsPath, 0755); err != nil {
		return fmt.Errorf("failed to create models directory: %w", err)
	}

	ui.Info("Starting minikube mount...")

	// Build the mount spec: <host-path>:<node-path>
	mountSpec := fmt.Sprintf("%s:%s", modelsPath, constants.MountPointModels)

	// Launch minikube mount in background
	// minikube -p nova mount <source>:<dest>
	cmd := exec.CommandContext(ctx, "minikube", "-p", "nova", "mount", mountSpec)

	// Redirect stdout/stderr to discard (mount runs silently in background)
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the process without waiting for it to complete
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start minikube mount: %w", err)
	}

	// Store the process for later lifecycle management
	mountProcess = cmd.Process

	ui.Success("Minikube mount started: %s -> %s", modelsPath, constants.MountPointModels)
	ui.Debug("Mount process PID: %d", mountProcess.Pid)

	return nil
}

// Stop stops the minikube mount process started by this nova instance.
// This is called by nova stop to cleanly terminate the mount.
func Stop(ctx context.Context) error {
	if mountProcess == nil {
		ui.Debug("No mount process to stop")
		return nil
	}

	ui.Info("Stopping minikube mount...")

	// Send SIGTERM for graceful shutdown
	if err := mountProcess.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if err.Error() != "os: process already finished" {
			return fmt.Errorf("failed to stop minikube mount: %w", err)
		}
	}

	// Wait for the process to exit (with timeout via context)
	// We don't block indefinitely - if it doesn't stop, that's ok
	done := make(chan error, 1)
	go func() {
		_, err := mountProcess.Wait()
		done <- err
	}()

	select {
	case <-done:
		// Process exited
	case <-ctx.Done():
		// Context cancelled, force kill
		mountProcess.Kill()
	}

	mountProcess = nil
	ui.Success("Minikube mount stopped")

	return nil
}

// Delete removes ALL minikube mount processes for the nova profile.
// This handles orphaned mount processes from previous nova instances.
// Called by nova delete to ensure complete cleanup.
func Delete(ctx context.Context) error {
	ui.Info("Stopping all minikube mount processes...")

	// First stop our own process if we have one
	if mountProcess != nil {
		_ = Stop(ctx)
	}

	// Kill ALL mount processes for nova profile (handles orphans)
	cmd := exec.CommandContext(ctx, "pkill", "-f", "minikube -p nova mount")
	_ = cmd.Run() // Ignore error - pkill returns 1 if no processes found

	// Verify all are gone
	running, _ := IsRunning(ctx)
	if running {
		// Force kill if still running
		cmd = exec.CommandContext(ctx, "pkill", "-9", "-f", "minikube -p nova mount")
		_ = cmd.Run()
	}

	ui.Success("All minikube mount processes stopped")
	return nil
}

// IsRunning checks if any minikube mount process is running for the nova profile.
// This detects mount processes from any nova instance, not just the current one.
func IsRunning(ctx context.Context) (bool, error) {
	// First check our in-memory reference
	if mountProcess != nil {
		err := mountProcess.Signal(syscall.Signal(0))
		if err == nil {
			return true, nil
		}
		// Process is dead, clean up our reference
		mountProcess = nil
	}

	// Check for existing mount processes from other nova instances
	cmd := exec.CommandContext(ctx, "pgrep", "-f", "minikube -p nova mount")
	if err := cmd.Run(); err == nil {
		return true, nil // Process found
	}

	return false, nil
}

// GetModelsPath returns the host path where models are stored.
// This is a convenience function that wraps the config method.
func GetModelsPath(cfg *config.Config) string {
	return cfg.GetModelsPath()
}

// GetMountPoint returns the path where models are mounted inside minikube nodes.
func GetMountPoint() string {
	return constants.MountPointModels
}

// GetModelMountPath returns the full path to a specific model inside the mount.
func GetModelMountPath(modelSlug string) string {
	return filepath.Join(constants.MountPointModels, modelSlug)
}
