package shared

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// EnsureNamespace ensures a namespace exists and has the specified labels.
// It is idempotent: checks existence first, then creates or updates.
func EnsureNamespace(ctx context.Context, name string, labels map[string]string) error {
	// 1. Create namespace if it doesn't exist
	if err := k8s.CreateNamespace(ctx, name); err != nil {
		return fmt.Errorf("failed to ensure namespace %s: %w", name, err)
	}

	// 2. Apply labels if provided
	if len(labels) > 0 {
		for key, value := range labels {
			if err := k8s.LabelNamespace(ctx, name, key, value); err != nil {
				return fmt.Errorf("failed to label namespace %s with %s=%s: %w", name, key, value, err)
			}
		}
	}

	return nil
}

// OperatorDeploymentOptions contains configuration for deploying a Kubernetes operator.
type OperatorDeploymentOptions struct {
	Name            string            // Operator name (e.g., "Keycloak Operator")
	Namespace       string            // Target namespace
	NamespaceLabels map[string]string // Labels to apply to namespace
	ManifestURLs    []string          // Ordered list of manifest URLs to apply
	DeploymentName  string            // Name of the operator deployment to wait for
	TimeoutSeconds  int               // Timeout for waiting for operator readiness (default: 300)
}

// DeployOperator deploys a Kubernetes operator using manifest URLs.
// This is a generic function that can be used for any operator deployment.
func DeployOperator(ctx context.Context, opts OperatorDeploymentOptions) error {
	// Set default timeout if not specified
	if opts.TimeoutSeconds == 0 {
		opts.TimeoutSeconds = 300
	}

	// Ensure namespace exists with labels
	if err := EnsureNamespace(ctx, opts.Namespace, opts.NamespaceLabels); err != nil {
		return err
	}

	// Apply all manifests in order
	for _, url := range opts.ManifestURLs {
		ui.Info("Applying %s...", url)
		if err := k8s.ApplyURLWithNamespace(ctx, url, opts.Namespace); err != nil {
			return fmt.Errorf("failed to apply %s: %w", url, err)
		}
	}

	// Wait for operator deployment to be ready
	ui.Info("Waiting for %s to be ready...", opts.Name)
	if err := k8s.WaitForDeploymentReady(ctx, opts.Namespace, opts.DeploymentName, opts.TimeoutSeconds); err != nil {
		return fmt.Errorf("%s not ready: %w", opts.Name, err)
	}

	return nil
}
