package shared

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/tools/helm"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// HelmDeploymentOptions defines options for deploying a Helm chart.
type HelmDeploymentOptions struct {
	// ReleaseName is the name of the Helm release
	ReleaseName string
	// ChartRef is the Helm chart reference (e.g., "jetstack/cert-manager" or "oci://...")
	ChartRef string
	// Version is the chart version (optional, for non-OCI charts where version isn't in URL)
	// For OCI charts, version is embedded in ChartRef (e.g., "oci://registry/chart:v1.0.0")
	Version string
	// Namespace is the Kubernetes namespace to deploy to
	Namespace string
	// ValuesPath is the path to the values.yaml file (optional)
	ValuesPath string
	// Values is a map of values to merge with the loaded values (optional)
	Values map[string]any
	// Wait indicates whether to wait for the deployment to complete
	Wait bool
	// TimeoutSeconds is the timeout for the deployment
	TimeoutSeconds int
	// InfoMessage is the message to display before deployment (optional)
	InfoMessage string
	// SuccessMessage is the message to display after successful deployment (optional)
	SuccessMessage string
	// CreateNamespace indicates whether to create the namespace before deployment
	CreateNamespace bool
	// ReuseValues indicates whether to reuse the existing release values
	ReuseValues bool
	// TemplateData is the data to use for rendering the values file as a template (optional)
	TemplateData any
}

// DeployHelmChart deploys a Helm chart with the specified options.
func DeployHelmChart(ctx context.Context, opts HelmDeploymentOptions) error {

	// Load values from file if specified
	var values map[string]any
	if opts.ValuesPath != "" {
		var loadedValues map[string]any
		var err error

		if opts.TemplateData != nil {
			// Render template
			rendered, err := RenderTemplate(opts.ValuesPath, opts.TemplateData)
			if err != nil {
				return fmt.Errorf("failed to render values template: %w", err)
			}

			// DEBUG: dump rendered template to file
			_ = os.WriteFile("/tmp/nova-rendered-values.yaml", []byte(rendered), 0644)

			// Parse YAML from rendered content
			if err := helm.UnmarshalValues([]byte(rendered), &loadedValues); err != nil {
				return fmt.Errorf("failed to parse templated values from %s: %w", opts.ValuesPath, err)
			}
		} else {
			loadedValues, err = helm.LoadValues(opts.ValuesPath)
			if err != nil {
				return fmt.Errorf("failed to load values from %s: %w", opts.ValuesPath, err)
			}
		}
		values = loadedValues
	}

	// Merge with additional values if provided
	if opts.Values != nil {
		if values == nil {
			values = opts.Values
		} else {
			values = helm.MergeValues(values, opts.Values)
		}
	}

	// Display info message if provided
	if opts.InfoMessage != "" {
		ui.Info("%s", opts.InfoMessage)
	}

	// Create Helm client
	helmClient := helm.NewClient(opts.Namespace)

	// Install or upgrade the chart
	if err := helmClient.InstallOrUpgradeReleaseWithOptions(
		ctx,
		opts.ReleaseName,
		opts.ChartRef,
		opts.Namespace,
		opts.Version,
		values,
		opts.Wait,
		opts.TimeoutSeconds,
		opts.ReuseValues,
		opts.CreateNamespace,
	); err != nil {
		return fmt.Errorf("failed to install %s: %w", opts.ReleaseName, err)
	}

	// Display success message if provided
	if opts.SuccessMessage != "" {
		ui.Success("%s", opts.SuccessMessage)
	}

	return nil
}

// ApplyTemplate processes a template file with the given data and applies it to the cluster.
// This is useful for generating Kubernetes manifests with dynamic values (e.g., secrets with passwords).
func ApplyTemplate(ctx context.Context, templatePath string, data any) error {
	// Render template
	rendered, err := RenderTemplate(templatePath, data)
	if err != nil {
		return err
	}

	// Apply the generated manifest
	if err := k8s.ApplyYAMLContent(ctx, rendered); err != nil {
		return fmt.Errorf("failed to apply manifest: %w", err)
	}

	return nil
}

// ApplyTemplateWithRetry processes a template file and applies it to the cluster with retry logic.
// This is useful when applying manifests that depend on webhooks that might not be fully ready
// (e.g., trust-manager webhook after Cilium CNI setup).
// maxRetries specifies the maximum number of retry attempts.
// initialDelay is the initial delay between retries (doubles each retry with exponential backoff).
func ApplyTemplateWithRetry(ctx context.Context, templatePath string, data any, maxRetries int, initialDelay time.Duration) error {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := ApplyTemplate(ctx, templatePath, data)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if this is a webhook connectivity error (Cilium not fully ready)
		errStr := err.Error()
		isWebhookError := strings.Contains(errStr, "failed calling webhook") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "operation not permitted") ||
			strings.Contains(errStr, "dial tcp")

		if !isWebhookError {
			// Non-retryable error
			return err
		}

		if attempt < maxRetries {
			ui.Warn("Webhook not ready, retrying in %v (attempt %d/%d)...", delay, attempt+1, maxRetries)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			// Exponential backoff with cap at 30 seconds
			delay = delay * 2
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
		}
	}

	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// LoadAndTemplateCustomValues loads a custom values file and renders it as a template.
// Returns nil if customValuesPath is empty. This consolidates the pattern of loading
// and templating custom values used across tier deployments.
func LoadAndTemplateCustomValues(customValuesPath string, templateData any) (map[string]any, error) {
	if customValuesPath == "" {
		return nil, nil
	}

	// Render template
	rendered, err := RenderTemplate(customValuesPath, templateData)
	if err != nil {
		return nil, err
	}

	var customValues map[string]any
	if err := helm.UnmarshalValues([]byte(rendered), &customValues); err != nil {
		return nil, fmt.Errorf("failed to parse custom values: %w", err)
	}

	return customValues, nil
}
