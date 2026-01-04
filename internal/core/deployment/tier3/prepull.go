package tier3

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
)

// PrepullHeavyImagesAsync starts pulling heavy tier 3 images in the background.
// This should be called early (during tier 0/1) so images are ready when tier 3 deploys.
//
// The function returns immediately and pulls images asynchronously on GPU nodes.
// It logs progress but doesn't block the main deployment flow.
//
// The image to pre-pull is dynamically determined from the llm-d Helm values file,
// ensuring consistency between pre-pull and actual deployment.
func PrepullHeavyImagesAsync(ctx context.Context, cfg *config.Config) {
	if !cfg.IsGPUMode() {
		ui.Debug("GPU mode disabled - skipping tier 3 image pre-pull")
		return
	}

	go func() {
		// Determine which image to pre-pull by reading from the appropriate values file
		image, err := shared.GetLLMDImage(cfg)
		if err != nil {
			ui.Warn("Failed to determine llm-d image for pre-pull: %v", err)
			return
		}

		ui.Info("⏬ Pre-pulling tier 3 llm-d image in background (this will take 20-40 min)...")
		ui.Debug("  Image: %s", image)

		if err := prepullImageToGPUNodes(ctx, image); err != nil {
			ui.Warn("Background image pre-pull failed (will retry during tier 3): %v", err)
			return
		}

		ui.Success("✓ Tier 3 llm-d image pre-pulled successfully")
	}()
}

// prepullImageToGPUNodes pulls an image on all GPU-labeled nodes using minikube ssh + docker pull.
// This is faster than waiting for kubelet to pull during pod creation.
func prepullImageToGPUNodes(ctx context.Context, image string) error {
	// Strategy: Use minikube ssh to docker pull directly on GPU nodes
	// This is faster than creating a DaemonSet because:
	// 1. No pod scheduling overhead
	// 2. Direct docker pull (no kubelet intermediary)
	// 3. Can show progress to user

	// Get GPU node name (in minikube, it's typically minikube-m02)
	// For simplicity in this lab, we hardcode the GPU node
	// In production, you'd query for nodes with nvidia.com/gpu.present=true
	gpuNode := "minikube-m02"

	ui.Debug("  Pulling on node: %s", gpuNode)

	// Use minikube ssh to pull image
	// Note: This runs in a goroutine, so we use a reasonable timeout
	// The actual pull might take 20-40 minutes for CUDA images
	pullCtx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	// Docker pull command
	cmd := fmt.Sprintf("minikube ssh -n %s -- docker pull %s", gpuNode, image)

	// For the background pull, we'll use a simple approach:
	// Just execute the command and let it run
	// We don't need sophisticated progress tracking since it's background
	pullCmd := exec.CommandContext(pullCtx, "sh", "-c", cmd)

	// Run and capture output (for logging if it fails)
	output, err := pullCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker pull failed on %s: %w (output: %s)", gpuNode, err, string(output))
	}

	ui.Debug("  Image cached on %s", gpuNode)
	return nil
}
