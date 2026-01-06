// Package warmup provides centralized warmup orchestration for NOVA deployment.
// This module runs between tier0 (cluster basics) and tier1 (infrastructure),
// preparing the cluster for efficient application deployment by:
//   - Electing the optimal node for llm-d deployment
//   - Pre-downloading LLM models to persistent storage (background)
//   - Pre-pulling heavy container images to elected nodes (background, GPU mode only)
package warmup

import (
	"context"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/imagewarmup"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// ImageWarmupResult contains information about the warmed-up image.
type ImageWarmupResult = imagewarmup.Result

// WarmupResult contains results and wait functions from warmup operations.
// The wait functions should be called by tier3 before deploying llm-d to ensure
// warmup operations have completed.
type WarmupResult struct {
	// NodeElected is the name of the node elected for llm-d deployment.
	// Empty string if election failed.
	NodeElected string

	// ModelWarmupStarted indicates whether model warmup was initiated.
	ModelWarmupStarted bool

	// ImageWarmupStarted indicates whether image warmup was initiated (GPU mode only).
	ImageWarmupStarted bool

	// WaitForModelWarmup is a function that blocks until model warmup completes.
	// Returns error if warmup failed. Nil if model warmup wasn't started.
	WaitForModelWarmup func() error

	// WaitForImageWarmup is a function that blocks until image warmup completes.
	// Returns result with success status. Nil if image warmup wasn't started.
	WaitForImageWarmup func() (*ImageWarmupResult, error)
}

// DeployWarmup orchestrates the warmup phase between tier0 and tier1.
//
// This function:
//  1. Elects the optimal node for llm-d deployment based on cluster configuration
//  2. Starts model warmup in background (downloads LLM model to PVC)
//  3. Starts image warmup in background (pre-pulls heavy images, GPU mode only)
//
// All warmup operations are non-blocking. The returned WarmupResult contains
// wait functions that tier3 should call before deploying llm-d.
//
// Error handling follows graceful degradation: failures in individual operations
// are logged as warnings, allowing deployment to continue. llm-d will download
// missing resources at runtime if warmup fails.
func DeployWarmup(ctx context.Context, cfg *config.Config) (*WarmupResult, error) {
	ui.Header("Warmup Module")

	result := &WarmupResult{}

	// Step 1: Elect llmd node
	ui.Info("Electing node for llm-d deployment...")
	electedNode, err := minikube.ElectLLMDNode(ctx, cfg)
	if err != nil {
		ui.Warn("Failed to elect llm-d node: %v", err)
		ui.Warn("Warmups and llm-d may not be scheduled optimally")
		// Continue despite election failure
	} else {
		result.NodeElected = electedNode
		ui.Success("Node elected: %s", electedNode)
	}

	// Step 2: Start model warmup (async)
	ui.Info("Starting model warmup...")
	waitForModel := StartModelWarmupAsync(ctx, cfg)
	if waitForModel != nil {
		result.ModelWarmupStarted = true
		result.WaitForModelWarmup = waitForModel
		ui.Success("Model warmup started in background")
	} else {
		ui.Info("Model warmup skipped (no model configured)")
	}

	// Step 3: Start image warmup (async, GPU only)
	if cfg.IsGPUMode() {
		ui.Info("Starting image warmup (GPU mode)...")
		waitForImage := StartImageWarmupAsync(ctx, cfg)
		if waitForImage != nil {
			result.ImageWarmupStarted = true
			result.WaitForImageWarmup = waitForImage
			ui.Success("Image warmup started in background")
		} else {
			ui.Warn("Image warmup could not be started")
		}
	} else {
		ui.Info("Image warmup skipped (CPU mode)")
	}

	ui.Header("Warmup Module Complete")
	if result.ModelWarmupStarted || result.ImageWarmupStarted {
		ui.Info("Background tasks running (will complete during tier1/tier2)")
	}

	return result, nil
}
