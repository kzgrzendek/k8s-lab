// Package shared provides common utilities for deployment operations across all tiers.
package shared

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/tools/crypto"
	"github.com/kzgrzendek/nova/internal/tools/helm"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// StepRunner wraps progress tracking for deployment steps.
type StepRunner struct {
	progress    *ui.StepProgress
	currentStep int
}

// NewStepRunner creates a new step runner with progress tracking.
func NewStepRunner(steps []string) *StepRunner {
	return &StepRunner{
		progress:    ui.NewStepProgress(steps),
		currentStep: 0,
	}
}

// RunStep executes a deployment step with progress tracking.
func (r *StepRunner) RunStep(name string, fn func() error) error {
	r.progress.StartStep(r.currentStep)
	if err := fn(); err != nil {
		r.progress.FailStep(r.currentStep, err)
		return err
	}
	r.progress.CompleteStep(r.currentStep)
	r.currentStep++
	return nil
}

// Complete marks all steps as complete.
func (r *StepRunner) Complete() {
	r.progress.Complete()
}

// AddHelmRepositories adds multiple Helm repositories with retry logic.
func AddHelmRepositories(ctx context.Context, repos map[string]string) error {
	helmClient := helm.NewClient("")

	ui.Info("Adding %d Helm repositories...", len(repos))
	for name, url := range repos {
		ui.Debug("  - Adding %s repository", name)
		if err := helmClient.AddRepository(ctx, name, url); err != nil {
			return fmt.Errorf("failed to add %s repository (%s): %w", name, url, err)
		}
	}

	ui.Info("Updating repository indexes...")
	if err := helmClient.UpdateRepositories(ctx); err != nil {
		return fmt.Errorf("failed to update Helm repository indexes: %w", err)
	}

	return nil
}

// GetOrGenerateSecret retrieves an existing secret or generates a new one.
// If the secret exists and contains the specified key, it returns the existing value.
// Otherwise, it generates a new random password of the specified length.
func GetOrGenerateSecret(ctx context.Context, namespace, secretName, key string, length int) (string, error) {
	if k8s.SecretExists(ctx, namespace, secretName) {
		data, err := k8s.GetSecretData(ctx, namespace, secretName)
		if err == nil && data[key] != "" {
			return data[key], nil
		}
		if err != nil {
			ui.Warn("Failed to retrieve secret %s/%s: %v", namespace, secretName, err)
		}
	}

	return crypto.GenerateRandomPassword(length)
}
