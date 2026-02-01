// Package tier3 handles the deployment of NOVA Tier 3 (Applications).
package tier3

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
)

// DeployQdrant deploys Qdrant vector database for LaSuite applications.
// Qdrant is used by OpenGateLLM for vector similarity search.
// This is conditional on the LaSuite profile being active.
func DeployQdrant(ctx context.Context, cfg *config.Config) error {
	// Check if Qdrant is needed based on infrastructure requirements
	req := cfg.GetInfraRequirements()
	if !req.Qdrant {
		ui.Info("Qdrant not required (LaSuite profile not active)")
		return nil
	}

	ui.Info("Deploying Qdrant vector database...")

	// Add Qdrant Helm repo
	repos := map[string]string{
		"qdrant": constants.HelmRepoQdrant,
	}
	if err := shared.AddHelmRepositories(ctx, repos); err != nil {
		return fmt.Errorf("failed to add Qdrant Helm repository: %w", err)
	}

	// Ensure namespace exists
	if err := shared.EnsureNamespace(ctx, constants.NamespaceQdrant, map[string]string{
		"service-type": "nova",
	}); err != nil {
		return fmt.Errorf("failed to ensure qdrant namespace: %w", err)
	}

	// Deploy Qdrant via Helm
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "qdrant",
		ChartRef:        cfg.Versions.Tier3.Qdrant.ChartRef(),
		Version:         cfg.Versions.Tier3.Qdrant.GetVersion(),
		Namespace:       constants.NamespaceQdrant,
		ValuesPath:      "resources/core/deployment/tier3/qdrant/helm/qdrant.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	}); err != nil {
		return err
	}

	ui.Success("Qdrant deployed successfully")
	return nil
}
