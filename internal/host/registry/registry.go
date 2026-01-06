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
// - Connect to the minikube network
// - Persist data to ~/.nova/registry
// - Restart automatically unless stopped
func Start(ctx context.Context, cfg *config.Config) error {
	// Check if registry is already running
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Prepare certificate directory (registry data will be stored in container, not on host)
	certDir, err := getRegistryCertPath(cfg)
	if err != nil {
		return fmt.Errorf("failed to get registry cert path: %w", err)
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("failed to create registry cert directory: %w", err)
	}
	certPath := filepath.Join(certDir, "registry.crt")
	keyPath := filepath.Join(certDir, "registry.key")

	// Generate certificate if it doesn't exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		ui.Debug("Generating TLS certificate for registry...")
		if err := pki.GenerateCertificate(
			certPath,
			keyPath,
			"nova-registry",
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

	// Create and start registry container on minikube network with TLS
	// Note: Registry data is stored in container (ephemeral), only certs are persisted on host
	containerCfg := docker.ContainerConfig{
		Name:  constants.ContainerRegistry,
		Image: constants.ImageRegistry,
		// Don't set User - let registry run as default user with correct permissions
		Ports: map[string]string{
			// Internal port only, no host mapping needed
			fmt.Sprintf("%d", constants.RegistryPort): fmt.Sprintf("%d", constants.RegistryPort),
		},
		Volumes: map[string]string{
			certDir: "/certs", // Only mount certs, not storage
		},
		Env: []string{
			// Enable TLS
			"REGISTRY_HTTP_TLS_CERTIFICATE=/certs/registry.crt",
			"REGISTRY_HTTP_TLS_KEY=/certs/registry.key",
		},
		Network:            "minikube",
		AdditionalNetworks: []string{}, // No additional networks needed
		RestartPolicy:      "unless-stopped",
	}

	if err := dockerClient.CreateAndStart(ctx, containerCfg); err != nil {
		return fmt.Errorf("failed to start registry container: %w", err)
	}

	ui.Success("Local registry started on port %d (TLS enabled)", constants.RegistryPort)
	ui.Debug("Registry certificate: %s", certPath)
	ui.Debug("Registry storage: ephemeral (in container)")

	return nil
}

// Stop stops the local Docker registry container.
func Stop(ctx context.Context, cfg *config.Config) error {
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
// Registry data is stored ephemerally in the container, only certs are persisted.
func getRegistryCertPath(cfg *config.Config) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".nova", "registry-certs"), nil
}
