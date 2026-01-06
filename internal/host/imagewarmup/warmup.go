// Package imagewarmup provides image warmup services for NOVA.
// Image warmup copies large container images to minikube nodes in the background
// using a local registry and skopeo (running as Docker container) to minimize RAM usage.
package imagewarmup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/host/registry"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
	"github.com/kzgrzendek/nova/internal/tools/skopeo"
)

// Result contains information about the image warmup operation.
type Result struct {
	Image   string
	Success bool
}

// StartAsync starts the image warmup process in the background.
// This function returns immediately and provides a callback to wait for completion.
//
// The warmup service:
// 1. Ensures the local registry is running
// 2. Starts a temporary skopeo container on the minikube network
// 3. Copies the image from source registry to local registry (~300MB RAM)
// 4. Distributes the image to all minikube nodes
// 5. Retags images to match original names for Kubernetes
//
// Returns a function that blocks until the warmup completes.
func StartAsync(ctx context.Context, cfg *config.Config, image string) func() (*Result, error) {
	// Create channel to signal completion
	done := make(chan *Result, 1)

	go func() {
		ui.Info("Starting image warmup in background: %s", image)

		if err := pullImageViaRegistry(ctx, cfg, image); err != nil {
			ui.Debug("Image warmup failed: %v", err)
			done <- &Result{Image: image, Success: false}
			return
		}

		ui.Success("Image warmup completed: %s", image)
		done <- &Result{Image: image, Success: true}
	}()

	// Return wait function
	return func() (*Result, error) {
		result := <-done
		return result, nil
	}
}

// pullImageViaRegistry implements the registry-based image distribution strategy.
//
// Strategy:
// 1. Ensure local registry is running
// 2. Use skopeo (Docker container) to copy image to registry
// 3. Pull image on each minikube node from registry
// 4. Retag to original name for Kubernetes compatibility
func pullImageViaRegistry(ctx context.Context, cfg *config.Config, image string) error {
	// Step 1: Ensure registry is running
	registryRunning, err := registry.IsRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check registry status: %w", err)
	}

	if !registryRunning {
		ui.Debug("Registry not running, starting it...")
		if err := registry.Start(ctx, cfg); err != nil {
			return fmt.Errorf("failed to start registry: %w", err)
		}
	}

	// Step 2: Copy image to local registry using skopeo container with TLS
	ui.Info("Copying image to local registry (this may take 10-30 minutes for large images)...")

	// Extract registry-relative path (e.g., ghcr.io/org/image:tag -> org/image:tag)
	registryImageName := extractRegistryImageName(image)

	skopeoClient := skopeo.NewClient()
	copyOpts := skopeo.CopyToRegistryOptions{
		SourceImage:          image,
		DestRegistry:         constants.RegistryHost,
		DestImage:            registryImageName,
		InsecureDestRegistry: false, // TLS is enabled
		SkipTLSVerify:        false, // Use proper TLS verification with mkcert CA
	}

	copyCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	if err := skopeoClient.CopyToRegistry(copyCtx, copyOpts); err != nil {
		return fmt.Errorf("failed to copy image to registry: %w", err)
	}

	ui.Success("Image copied to local registry")

	// Step 3: Distribute to elected llm-d node only
	ui.Info("Distributing image to elected llm-d node...")

	// Get the elected node (labeled with nova.local/llmd-node=true)
	targetNodes, err := minikube.GetNodesByLabel(ctx, cfg, "nova.local/llmd-node=true")
	if err != nil {
		return fmt.Errorf("failed to get llm-d node: %w", err)
	}

	if len(targetNodes) == 0 {
		ui.Warn("No node found with label nova.local/llmd-node=true")
		ui.Info("Image will be pulled when llm-d pod starts")
		return nil
	}

	registryImage := fmt.Sprintf("%s/%s", constants.RegistryHost, registryImageName)

	for _, node := range targetNodes {
		ui.Debug("Pulling on node %s...", node)

		// Pull from registry
		if err := pullOnNode(ctx, node, registryImage); err != nil {
			return fmt.Errorf("failed to pull on node %s: %w", node, err)
		}

		// Retag to original name
		if err := retagOnNode(ctx, node, registryImage, image); err != nil {
			return fmt.Errorf("failed to retag on node %s: %w", node, err)
		}

		ui.Success("Image ready on node %s", node)
	}

	ui.Success("Image warmed up on %d target node(s)", len(targetNodes))
	return nil
}

// pullOnNode pulls an image on a specific minikube node.
func pullOnNode(ctx context.Context, nodeName, image string) error {
	pullCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	pullCmd := fmt.Sprintf("docker pull %s", image)
	cmd := exec.CommandContext(pullCtx, "minikube", "ssh", "-n", nodeName, "--", pullCmd)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pull failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// retagOnNode retags an image on a specific minikube node.
func retagOnNode(ctx context.Context, nodeName, sourceTag, targetTag string) error {
	retagCmd := fmt.Sprintf("docker tag %s %s", sourceTag, targetTag)
	cmd := exec.CommandContext(ctx, "minikube", "ssh", "-n", nodeName, "--", retagCmd)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("retag failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// extractRegistryImageName extracts the image name and tag from a full reference.
// Example: ghcr.io/llm-d/llm-d-cuda:v1.2.3 -> llm-d/llm-d-cuda:v1.2.3
func extractRegistryImageName(fullImage string) string {
	parts := strings.Split(fullImage, "/")

	// If there's a registry host (e.g., ghcr.io), skip it
	if len(parts) >= 3 {
		return strings.Join(parts[1:], "/")
	}

	// Simple image name (e.g., ubuntu:22.04)
	return fullImage
}
