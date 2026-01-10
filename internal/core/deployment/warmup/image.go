// Package warmup provides warmup operations for NOVA deployment.
// This includes model downloading and image pre-pulling to optimize startup time.
package warmup

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

// ImageWarmupResult contains information about the image warmup operation.
type ImageWarmupResult struct {
	Image   string
	Success bool
}

// StartImageWarmupAsync starts the image warmup process in the background.
// This function returns immediately and provides a callback to wait for completion.
//
// The warmup service:
// 1. Ensures the local registry is running
// 2. Starts a temporary skopeo container on the nova network
// 3. Copies the image from source registry to local registry (~300MB RAM)
// 4. Distributes the image to all minikube nodes
// 5. Retags images to match original names for Kubernetes
//
// If the warmup fails, it cancels the provided context to stop the parent deployment process.
//
// Returns a function that blocks until the warmup completes.
func StartImageWarmupAsync(ctx context.Context, cancelFunc context.CancelFunc, cfg *config.Config, image string) func() (*ImageWarmupResult, error) {
	// Create channel to signal completion
	done := make(chan *ImageWarmupResult, 1)

	go func() {
		ui.Info("Starting image warmup in background: %s", image)

		if err := pullImageViaRegistry(ctx, cfg, image); err != nil {
			ui.Error("Image warmup failed: %v", err)
			ui.Error("Cancelling deployment - warmup is required for tier 3")
			// Cancel parent context to stop deployment immediately
			cancelFunc()
			done <- &ImageWarmupResult{Image: image, Success: false}
			return
		}

		ui.Success("Image warmup completed: %s", image)
		done <- &ImageWarmupResult{Image: image, Success: true}
	}()

	// Return wait function
	return func() (*ImageWarmupResult, error) {
		result := <-done
		return result, nil
	}
}

// pullImageViaRegistry implements the registry-based image distribution strategy.
//
// Strategy:
// 1. Ensure local registry is running
// 2. Use skopeo (Docker container) to copy image to registry (with retry)
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
	// Use retry logic to handle transient registry failures
	ui.Info("Copying image to local registry (this may take 10-30 minutes for large images)...")

	// Extract registry-relative path (e.g., ghcr.io/org/image:tag -> org/image:tag)
	registryImageName := extractRegistryImageName(image)

	if err := copyImageWithRetry(ctx, cfg, image, registryImageName); err != nil {
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
	cmd := exec.CommandContext(pullCtx, "minikube", "-p", "nova", "ssh", "-n", nodeName, "--", pullCmd)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pull failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// retagOnNode retags an image on a specific minikube node.
func retagOnNode(ctx context.Context, nodeName, sourceTag, targetTag string) error {
	retagCmd := fmt.Sprintf("docker tag %s %s", sourceTag, targetTag)
	cmd := exec.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", nodeName, "--", retagCmd)

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

// copyImageWithRetry copies an image to the local registry with retry logic.
// This handles transient failures like registry restarts during large image uploads.
func copyImageWithRetry(ctx context.Context, cfg *config.Config, sourceImage, registryImageName string) error {
	const maxRetries = 3
	var lastErr error

	skopeoClient := skopeo.NewClient()
	copyOpts := skopeo.CopyToRegistryOptions{
		SourceImage:          sourceImage,
		DestRegistry:         constants.RegistryHost,
		DestImage:            registryImageName,
		InsecureDestRegistry: false, // TLS is enabled
		SkipTLSVerify:        false, // Use proper TLS verification with mkcert CA
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Ensure registry is running before each attempt
		running, err := registry.IsRunning(ctx)
		if err != nil {
			ui.Warn("Failed to check registry status: %v", err)
		}
		if !running {
			ui.Info("Registry not running, starting it (attempt %d/%d)...", attempt, maxRetries)
			if err := registry.Start(ctx, cfg); err != nil {
				lastErr = fmt.Errorf("failed to start registry: %w", err)
				continue
			}
			// Wait for registry to be fully ready
			time.Sleep(2 * time.Second)
		}

		// Set timeout for this copy attempt
		copyCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)

		err = skopeoClient.CopyToRegistry(copyCtx, copyOpts)
		cancel()

		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if this is a retryable error (connection reset, EOF, etc.)
		errStr := err.Error()
		isRetryable := strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "EOF") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "MANIFEST_UNKNOWN") ||
			strings.Contains(errStr, "timeout")

		if !isRetryable {
			// Non-retryable error, fail immediately
			return err
		}

		if attempt < maxRetries {
			delay := time.Duration(attempt*30) * time.Second
			ui.Warn("Image copy failed (attempt %d/%d), retrying in %v: %v", attempt, maxRetries, delay, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
