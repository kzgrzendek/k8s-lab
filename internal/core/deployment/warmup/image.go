// Package warmup provides warmup operations for NOVA deployment.
// This includes model downloading and image pre-pulling to optimize startup time.
package warmup

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// ImageWarmupResult contains information about the image warmup operation.
type ImageWarmupResult struct {
	Image   string
	Success bool
}

// StartImageWarmupAsync starts the image warmup process in the background.
// This function returns immediately and provides a callback to wait for completion.
//
// The warmup simply pulls the image directly on the elected llm-d node via SSH.
// This is more efficient than the registry-based approach since we only target
// one node at a time.
//
// If the warmup fails, it cancels the provided context to stop the parent deployment process.
//
// Returns a function that blocks until the warmup completes.
func StartImageWarmupAsync(ctx context.Context, cancelFunc context.CancelFunc, cfg *config.Config, image string) func() (*ImageWarmupResult, error) {
	// Create channel to signal completion
	done := make(chan *ImageWarmupResult, 1)

	go func() {
		ui.Info("Starting image warmup in background: %s", image)

		if err := pullImageDirectOnNode(ctx, cfg, image); err != nil {
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

// pullImageDirectOnNode pulls the image directly on the elected llm-d node.
// This is simpler and faster than the registry-based approach since we only
// need to download the image once (directly from the source registry to the node).
func pullImageDirectOnNode(ctx context.Context, cfg *config.Config, image string) error {
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

	// Pull directly on the elected node
	node := targetNodes[0]
	ui.Info("Pulling image directly on node %s (this may take 10-30 minutes for large images)...", node)

	if err := pullOnNode(ctx, node, image); err != nil {
		return fmt.Errorf("failed to pull image on node %s: %w", node, err)
	}

	ui.Success("Image ready on node %s", node)
	return nil
}

// pullOnNode pulls an image on a specific minikube node via SSH.
func pullOnNode(ctx context.Context, nodeName, image string) error {
	pullCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	pullCmd := fmt.Sprintf("docker pull %s", image)
	cmd := exec.CommandContext(pullCtx, "minikube", "-p", "nova", "ssh", "-n", nodeName, "--", pullCmd)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pull failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
