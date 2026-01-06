package warmup

import (
	"context"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/host/imagewarmup"
)

// StartImageWarmupAsync starts the image warmup service in the background.
// This MUST be called AFTER minikube starts and the local registry is running.
//
// The function returns immediately and warms up images asynchronously.
// Returns a wait function that can be called to wait for completion.
//
// The image to warm up is dynamically determined from the llm-d Helm values file,
// ensuring consistency between warmup and actual deployment.
//
// Returns nil if GPU mode is disabled (image warmup only runs in GPU mode).
// Delegates to the imagewarmup host service for actual implementation.
func StartImageWarmupAsync(ctx context.Context, cfg *config.Config) func() (*ImageWarmupResult, error) {
	if !cfg.IsGPUMode() {
		ui.Debug("GPU mode disabled - skipping image warmup")
		return nil
	}

	// Determine which image to warm up
	image, err := shared.GetLLMDImage(cfg)
	if err != nil {
		ui.Debug("Failed to determine image for warmup: %v", err)
		return func() (*ImageWarmupResult, error) {
			return &ImageWarmupResult{Image: "", Success: false}, nil
		}
	}

	ui.Info("Starting image warmup for: %s", image)

	// Start the warmup service (returns wait function)
	return imagewarmup.StartAsync(ctx, cfg, image)
}
