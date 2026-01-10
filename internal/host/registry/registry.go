// Package registry provides management for the local Docker registry container.
// This registry serves as an intermediary for multi-node minikube clusters,
// allowing efficient image distribution without RAM-intensive transfers.
package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
	"github.com/kzgrzendek/nova/internal/tools/docker"
)

// Start starts the local Docker registry container.
// The registry is configured to:
// - Run on port 5000
// - Use persistent storage at ~/.nova/registry-data
// - Use mkcert-generated TLS certificates for registry.local
// - Restart automatically unless stopped
func Start(ctx context.Context, cfg *config.Config) error {
	// Check if registry is already running
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Prepare certificate directory
	certDir, err := getRegistryCertPath(cfg)
	if err != nil {
		return fmt.Errorf("failed to get registry cert path: %w", err)
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create registry cert directory: %w", err)
	}
	certPath := filepath.Join(certDir, "registry.crt")
	keyPath := filepath.Join(certDir, "registry.key")

	// Prepare persistent data directory
	dataDir, err := getRegistryDataPath(cfg)
	if err != nil {
		return fmt.Errorf("failed to get registry data path: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create registry data directory: %w", err)
	}

	// Pre-create registry directory structure that the registry expects
	// This avoids permission issues when running as non-root user
	registrySubdirs := []string{
		filepath.Join(dataDir, "docker", "registry", "v2", "blobs"),
		filepath.Join(dataDir, "docker", "registry", "v2", "repositories"),
	}
	for _, dir := range registrySubdirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create registry subdirectory %s: %w", dir, err)
		}
	}

	// Generate certificate if it doesn't exist using mkcert
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		ui.Debug("Generating TLS certificate for %s using mkcert...", constants.RegistryDomain)
		if err := pki.GenerateCertificate(
			certPath,
			keyPath,
			constants.RegistryDomain,
			"localhost",
			"127.0.0.1",
		); err != nil {
			return fmt.Errorf("failed to generate registry certificate: %w", err)
		}
		ui.Debug("Registry TLS certificate generated")
	} else {
		ui.Debug("Using existing registry TLS certificate")
	}

	running, err := dockerClient.IsRunning(ctx, constants.ContainerRegistry)
	if err != nil {
		return fmt.Errorf("failed to check registry status: %w", err)
	}

	if running {
		ui.Debug("Registry already running")
		return nil
	}

	// Check if container exists but is stopped
	exists, err := dockerClient.Exists(ctx, constants.ContainerRegistry)
	if err != nil {
		return fmt.Errorf("failed to check if registry exists: %w", err)
	}

	if exists {
		ui.Debug("Registry container exists but is stopped, removing it...")
		if err := dockerClient.Remove(ctx, constants.ContainerRegistry, true); err != nil {
			return fmt.Errorf("failed to remove existing registry container: %w", err)
		}
	}

	ui.Info("Starting local registry container with TLS...")

	// Get current user UID and GID to avoid creating root-owned files
	uid := os.Getuid()
	gid := os.Getgid()
	userSpec := fmt.Sprintf("%d:%d", uid, gid)

	// Create and start registry container with persistent storage
	// Storage is persisted on host, survives container deletion and minikube cluster rebuilds
	// We pre-created the directory structure above, so non-root user can write to it
	containerCfg := docker.ContainerConfig{
		Name:  constants.ContainerRegistry,
		Image: constants.ImageRegistry,
		User:  userSpec, // Run as current user to avoid root-owned files
		Ports: map[string]string{
			// Internal port only, no host mapping needed
			fmt.Sprintf("%d", constants.RegistryPort): fmt.Sprintf("%d", constants.RegistryPort),
		},
		Volumes: map[string]string{
			certDir: "/certs",            // Mount TLS certificates
			dataDir: "/var/lib/registry", // Mount persistent storage
		},
		Env: []string{
			// Enable TLS
			"REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt",
			"REGISTRY_HTTP_TLS_KEY=/certs/registry.key",
		},
		Network:       "nova",
		RestartPolicy: "unless-stopped",
	}

	if err := dockerClient.CreateAndStart(ctx, containerCfg); err != nil {
		return fmt.Errorf("failed to start registry container: %w", err)
	}

	ui.Success("Local registry started on port %d (TLS enabled)", constants.RegistryPort)
	ui.Debug("Registry certificate: %s", certPath)
	ui.Debug("Registry storage (persistent): %s", dataDir)

	return nil
}

// Stop stops the local Docker registry container.
func Stop(ctx context.Context) error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	ui.Info("Stopping local registry...")

	if err := dockerClient.Stop(ctx, constants.ContainerRegistry); err != nil {
		return fmt.Errorf("failed to stop registry: %w", err)
	}

	ui.Success("Local registry stopped")
	return nil
}

// Delete removes the local Docker registry container.
func Delete(ctx context.Context) error {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Stop and remove the container
	if err := dockerClient.Remove(ctx, constants.ContainerRegistry, true); err != nil {
		return fmt.Errorf("failed to remove registry: %w", err)
	}

	return nil
}

// IsRunning checks if the registry container is running.
func IsRunning(ctx context.Context) (bool, error) {
	dockerClient, err := docker.NewClient()
	if err != nil {
		return false, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	return dockerClient.IsRunning(ctx, constants.ContainerRegistry)
}

// GetHost returns the registry host address accessible from minikube nodes.
func GetHost() string {
	return constants.RegistryHost
}

// getRegistryCertPath returns the path where registry TLS certificates are stored.
func getRegistryCertPath(cfg *config.Config) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".nova", "registry-certs"), nil
}

// getRegistryDataPath returns the path where registry data is persisted.
// This directory survives container deletion and minikube cluster rebuilds.
func getRegistryDataPath(cfg *config.Config) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".nova", "registry-data"), nil
}
