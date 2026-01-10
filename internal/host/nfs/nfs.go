// Package nfs provides management for the local NFS server container.
// This NFS server serves as persistent storage for Kubernetes PVCs,
// allowing model data and workspaces to survive minikube cluster rebuilds.
package nfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/tools/docker"
)

// Start starts the local NFS server container.
// The NFS server is configured to:
// - Run on port 2049 (NFSv4)
// - Export ~/.nova/share/nfs directory (contains models/ subdirectory)
// - Connect to the nova network
// - Restart automatically unless stopped
func Start(ctx context.Context, cfg *config.Config) error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Prepare NFS export directory with permissive permissions
	// In a local lab environment, we use 0777 to allow writes from any user/pod
	// This is acceptable for local development but should be restricted in production
	exportDir, err := getNFSExportPath(cfg)
	if err != nil {
		return fmt.Errorf("failed to get NFS export path: %w", err)
	}

	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return fmt.Errorf("failed to create NFS export directory: %w", err)
	}

	// Set permissive permissions to allow Kubernetes pods to write
	// This is safe for local development where all access is trusted
	if err := os.Chmod(exportDir, 0777); err != nil {
		return fmt.Errorf("failed to set NFS export directory permissions: %w", err)
	}

	running, err := dockerClient.IsRunning(ctx, constants.ContainerNFS)
	if err != nil {
		return fmt.Errorf("failed to check NFS server status: %w", err)
	}

	if running {
		ui.Debug("NFS server already running")
		return nil
	}

	// Check if container exists but is stopped
	exists, err := dockerClient.Exists(ctx, constants.ContainerNFS)
	if err != nil {
		return fmt.Errorf("failed to check if NFS server exists: %w", err)
	}

	if exists {
		ui.Debug("NFS server container exists but is stopped, removing it...")
		if err := dockerClient.Remove(ctx, constants.ContainerNFS, true); err != nil {
			return fmt.Errorf("failed to remove existing NFS server container: %w", err)
		}
	}

	ui.Info("Starting local NFS server...")

	// Create and start NFS server container
	// The container exports the host directory and makes it accessible via NFSv4
	// Export options: rw,no_root_squash,no_subtree_check,insecure
	// - no_root_squash: Allows root in containers to write as root (required for K8s)
	// - insecure: Allows connections from ports > 1024 (required for unprivileged clients)
	containerCfg := docker.ContainerConfig{
		Name:  constants.ContainerNFS,
		Image: constants.ImageNFS,
		Env: []string{
			fmt.Sprintf("SHARED_DIRECTORY=%s", "/nfs-export"), // Internal mount point
			"SYNC=true", // Sync writes to disk for data safety
		},
		Volumes: map[string]string{
			exportDir: "/nfs-export", // Mount host directory into container
		},
		// NFS requires privileged mode for mount operations
		Privileged:    true,
		Network:       "nova",
		RestartPolicy: "unless-stopped",
	}

	if err := dockerClient.CreateAndStart(ctx, containerCfg); err != nil {
		return fmt.Errorf("failed to start NFS server container: %w", err)
	}

	ui.Success("Local NFS server started on port %d", constants.NFSPort)
	ui.Debug("NFS export directory: %s", exportDir)

	return nil
}

// Stop stops the local NFS server container.
func Stop(ctx context.Context) error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	ui.Info("Stopping local NFS server...")

	if err := dockerClient.Stop(ctx, constants.ContainerNFS); err != nil {
		return fmt.Errorf("failed to stop NFS server: %w", err)
	}

	ui.Success("Local NFS server stopped")
	return nil
}

// Delete removes the local NFS server container.
func Delete(ctx context.Context) error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Stop and remove the container
	if err := dockerClient.Remove(ctx, constants.ContainerNFS, true); err != nil {
		return fmt.Errorf("failed to remove NFS server: %w", err)
	}

	return nil
}

// IsRunning checks if the NFS server container is running.
func IsRunning(ctx context.Context) (bool, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return false, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	return dockerClient.IsRunning(ctx, constants.ContainerNFS)
}

// GetExportPath returns the host path being exported by NFS.
func GetExportPath(cfg *config.Config) (string, error) {
	return getNFSExportPath(cfg)
}

// getNFSExportPath returns the path where NFS exports are stored.
// This directory is mounted into the NFS container and made available to Kubernetes.
// Structure:
//   - ~/.nova/share/nfs/         (NFS root export directory)
//   - ~/.nova/share/nfs/models/  (Models subdirectory)
func getNFSExportPath(cfg *config.Config) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".nova", "share", "nfs"), nil
}

// GetModelsPath returns the path where models are stored within the NFS export.
// Models are stored at ~/.nova/share/nfs/models/
func GetModelsPath(cfg *config.Config) (string, error) {
	nfsRoot, err := getNFSExportPath(cfg)
	if err != nil {
		return "", err
	}

	return filepath.Join(nfsRoot, "models"), nil
}
