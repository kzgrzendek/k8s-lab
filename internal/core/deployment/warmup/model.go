package warmup

import (
	"context"
	"fmt"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// StartModelWarmupAsync starts warming up the LLM model in a background Kubernetes Job.
// This downloads the model to a PVC which can then be mounted by llmd for instant startup.
//
// Strategy:
//  1. Create namespace and PVC for the model (one PVC per model slug)
//  2. Start a Kubernetes Job that downloads the model using huggingface-cli
//  3. Job runs in background, downloads to PVC at /models/model
//  4. llmd (in tier3) will wait for this Job and mount the PVC
//
// Returns a wait function that tier3 can call to wait for completion.
// Returns nil if no model is configured (warmup skipped).
func StartModelWarmupAsync(ctx context.Context, cfg *config.Config) func() error {
	if cfg.LLM.Model == "" {
		ui.Debug("No LLM model configured, skipping warmup")
		return nil
	}

	modelSlug := cfg.GetModelSlug()
	jobName := fmt.Sprintf("model-warmup-%s", modelSlug)
	namespace := constants.NamespaceLLMD

	// Start warmup in goroutine (non-blocking)
	go func() {
		pvcName := fmt.Sprintf("llm-model-%s", modelSlug)

		ui.Info("Starting model warmup in background: %s", cfg.LLM.Model)

		// Create namespace with proper labels
		if err := shared.EnsureNamespace(ctx, namespace, map[string]string{
			"name":                           "llmd",
			"service-type":                   "llm",
			"trust-manager/inject-ca-secret": "enabled",
			"pod-security.kubernetes.io/enforce": "baseline",
		}); err != nil {
			ui.Debug("Failed to create namespace for warmup: %v", err)
			return
		}

		// Check if PVC already exists and has model
		if pvcExists, _ := k8s.ResourceExists(ctx, "pvc", pvcName, namespace); pvcExists {
			ui.Debug("PVC %s already exists, model may be cached", pvcName)
			// PVC exists, Job is idempotent so we'll create it anyway
		}

		// Create HF token secret if provided
		if cfg.LLM.HfToken != "" {
			if err := k8s.CreateSecret(ctx, namespace, "huggingface-token", map[string]string{
				"token": cfg.LLM.HfToken,
			}); err != nil {
				ui.Debug("Failed to create HF token secret: %v", err)
				// Not fatal, continue without token (public models will work)
			}
		}

		// Prepare template data
		data := map[string]interface{}{
			"Model":       cfg.LLM.Model,
			"ModelSlug":   modelSlug,
			"HfToken":     cfg.LLM.HfToken,
			"StorageSize": "20Gi",
		}

		// Create PVC
		pvcPath := "resources/core/deployment/warmup/model-warmup/pvc.yaml"
		if err := k8s.ApplyYAMLWithTemplate(ctx, pvcPath, data); err != nil {
			ui.Debug("Failed to create model PVC: %v", err)
			return
		}

		// Create warmup Job
		jobPath := "resources/core/deployment/warmup/model-warmup/job.yaml"
		if err := k8s.ApplyYAMLWithTemplate(ctx, jobPath, data); err != nil {
			ui.Debug("Failed to create warmup Job: %v", err)
			return
		}

		ui.Success("Model warmup Job created: model-warmup-%s", modelSlug)
	}()

	// Return wait function for tier3 to use
	return func() error {
		return waitForModelWarmup(ctx, jobName, namespace)
	}
}

// waitForModelWarmup waits for a model warmup Kubernetes Job to complete successfully.
func waitForModelWarmup(ctx context.Context, jobName, namespace string) error {
	const timeout = 30 * time.Minute

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Duration(constants.DefaultCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Job %s to complete", jobName)
		case <-ticker.C:
			// Check Job status
			completed, err := k8s.IsJobComplete(ctx, jobName, namespace)
			if err != nil {
				ui.Debug("Error checking Job status: %v", err)
				continue
			}
			if completed {
				return nil
			}
			// Log progress every interval
			ui.Debug("Waiting for warmup Job %s to complete...", jobName)
		}
	}
}
