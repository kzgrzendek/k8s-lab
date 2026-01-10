// Package warmup provides orchestration for background warmup operations.
//
// The warmup phase runs in parallel with tier 0-2 deployment to optimize startup time:
//   - Model download: Downloads LLM models from Hugging Face to NFS storage
//   - Image warmup: Pre-pulls heavy container images to minikube nodes (GPU mode only)
//
// Warmup operations use context cancellation for fail-fast behavior: if any warmup
// operation fails, the entire deployment is cancelled immediately.
package warmup

import (
	"context"
	"fmt"
	"sync"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
)

// Orchestrator manages background warmup operations for tier 3 deployments.
// It coordinates model downloads and image warmup in parallel with cluster deployment.
type Orchestrator struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config

	waitForModelDownload func() (*ModelDownloadResult, error)
	waitForImageWarmup   func() (*ImageWarmupResult, error)

	mu      sync.Mutex
	started bool
}

// New creates a new warmup orchestrator.
// The provided context will be used as the parent for all warmup operations.
func New(ctx context.Context, cfg *config.Config) *Orchestrator {
	warmupCtx, cancel := context.WithCancel(ctx)
	return &Orchestrator{
		ctx:    warmupCtx,
		cancel: cancel,
		cfg:    cfg,
	}
}

// Start initiates background warmup operations (model download + image warmup).
// This function returns immediately - use Wait() to block until completion.
//
// Warmup operations run in background goroutines and will automatically
// cancel the deployment context if they fail.
func (o *Orchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.started {
		return fmt.Errorf("warmup already started")
	}

	ui.Header("Warmup: Background Operations")

	// Start model download in background
	ui.Info("Starting model download in background...")
	o.waitForModelDownload = StartModelDownloadAsync(o.ctx, o.cancel, o.cfg)
	if o.waitForModelDownload != nil {
		ui.Success("Model download started in background")
	} else {
		ui.Info("Model download skipped (no model configured)")
	}

	// Start image warmup in background
	var warmupImage string
	imageTag := o.cfg.GetLLMDImageTag()
	if o.cfg.IsGPUMode() {
		warmupImage = fmt.Sprintf("ghcr.io/llm-d/llm-d-cuda:%s", imageTag)
		ui.Info("Starting image warmup in background (GPU mode)...")
	} else {
		warmupImage = fmt.Sprintf("ghcr.io/llm-d/llm-d-cpu:%s", imageTag)
		ui.Info("Starting image warmup in background (CPU mode)...")
	}

	o.waitForImageWarmup = StartImageWarmupAsync(o.ctx, o.cancel, o.cfg, warmupImage)
	if o.waitForImageWarmup != nil {
		ui.Success("Image warmup started in background")
	}

	ui.Success("Warmup operations running in background")
	ui.Info("")

	o.started = true
	return nil
}

// Wait blocks until all warmup operations complete.
// Returns an error if any warmup operation failed.
//
// This should be called after tier 0-2 deployments complete, but before
// tier 3 deployment begins, to ensure models and images are ready.
func (o *Orchestrator) Wait() error {
	if !o.started {
		return nil // Nothing to wait for
	}

	ui.Header("Warmup: Waiting for Completion")

	// Wait for model download
	if o.waitForModelDownload != nil {
		ui.Step("Waiting for model download to complete...")
		modelResult, err := o.waitForModelDownload()
		if err != nil {
			ui.Error("Model download failed: %v", err)
			return fmt.Errorf("model download failed: %w", err)
		}
		if modelResult != nil && modelResult.Success {
			ui.Success("Model downloaded to: %s", modelResult.ModelPath)
		}
	}

	// Wait for image warmup
	if o.waitForImageWarmup != nil {
		ui.Step("Waiting for image warmup to complete...")
		imageResult, err := o.waitForImageWarmup()
		if err != nil {
			ui.Error("Image warmup failed: %v", err)
			return fmt.Errorf("image warmup failed: %w", err)
		}
		if imageResult != nil && imageResult.Success {
			ui.Success("Image warmed up: %s", imageResult.Image)
		}
	}

	ui.Success("All warmup operations completed successfully")
	return nil
}

// Cancel stops all warmup operations immediately.
// This is called automatically when a warmup operation fails, but can also
// be called manually to abort warmup (e.g., on user interrupt).
func (o *Orchestrator) Cancel() {
	o.cancel()
}

// Context returns the warmup context.
// This context is cancelled if any warmup operation fails, allowing
// the deployment to fail-fast instead of continuing with partial warmup.
func (o *Orchestrator) Context() context.Context {
	return o.ctx
}
