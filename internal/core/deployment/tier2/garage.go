// Package tier2 handles the deployment of NOVA Tier 2 (Platform Services).
package tier2

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// DeployGarage deploys Garage S3-compatible storage for LaSuite applications.
// Garage is a lightweight, self-hosted S3-compatible storage solution.
// This is conditional on the LaSuite profile being active.
func DeployGarage(ctx context.Context, cfg *config.Config) error {
	// Check if Garage is needed based on infrastructure requirements
	req := cfg.GetInfraRequirements()
	if !req.Garage {
		ui.Info("Garage S3 not required (LaSuite profile not active)")
		return nil
	}

	ui.Info("Deploying Garage S3-compatible storage...")

	// Add Garage Helm repo
	repos := map[string]string{
		"garage": constants.HelmRepoGarage,
	}
	if err := shared.AddHelmRepositories(ctx, repos); err != nil {
		return fmt.Errorf("failed to add Garage Helm repository: %w", err)
	}

	// Generate or retrieve existing S3 credentials
	accessKey, err := shared.GetOrGenerateSecret(ctx, constants.NamespaceGarage, "garage-credentials", "access-key", 20)
	if err != nil {
		return fmt.Errorf("failed to get or generate S3 access key: %w", err)
	}
	secretKey, err := shared.GetOrGenerateSecret(ctx, constants.NamespaceGarage, "garage-credentials", "secret-key", 40)
	if err != nil {
		return fmt.Errorf("failed to get or generate S3 secret key: %w", err)
	}

	// Ensure namespace exists
	if err := shared.EnsureNamespace(ctx, constants.NamespaceGarage, map[string]string{
		"service-type": "nova",
	}); err != nil {
		return fmt.Errorf("failed to ensure garage namespace: %w", err)
	}

	// Create S3 credentials secret
	ui.Info("Creating Garage S3 credentials...")
	if err := k8s.CreateSecret(ctx, constants.NamespaceGarage, "garage-credentials", map[string]string{
		"access-key": accessKey,
		"secret-key": secretKey,
	}); err != nil {
		return fmt.Errorf("failed to create garage credentials secret: %w", err)
	}

	// Deploy Garage via Helm
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "garage",
		ChartRef:       cfg.Versions.Tier2.Garage.ChartRef(),
		Version:        cfg.Versions.Tier2.Garage.GetVersion(),
		Namespace:      constants.NamespaceGarage,
		ValuesPath:     "resources/core/deployment/tier2/garage/values.yaml",
		Wait:           true,
		TimeoutSeconds: 300,
	}); err != nil {
		return err
	}

	ui.Success("Garage S3 deployed successfully")
	return nil
}

// CopyGarageCredentialsToNamespace copies Garage S3 credentials to a target namespace.
// This is used by applications that need to access Garage S3 storage.
func CopyGarageCredentialsToNamespace(ctx context.Context, targetNamespace string) error {
	// Get credentials from garage namespace
	data, err := k8s.GetSecretData(ctx, constants.NamespaceGarage, "garage-credentials")
	if err != nil {
		return fmt.Errorf("failed to get garage credentials: %w", err)
	}

	// Create secret in target namespace
	if err := k8s.CreateSecret(ctx, targetNamespace, "s3-credentials", map[string]string{
		"access-key": data["access-key"],
		"secret-key": data["secret-key"],
	}); err != nil {
		return fmt.Errorf("failed to copy s3 credentials to %s: %w", targetNamespace, err)
	}

	return nil
}
