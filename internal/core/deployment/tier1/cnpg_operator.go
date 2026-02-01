// Package tier1 handles the deployment of NOVA Tier 1 (Cluster Infrastructure).
package tier1

import (
	"context"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
)

// deployCNPGOperator deploys CloudNative PG operator CRDs and controller.
// The operator is deployed in tier1 to be available for all database needs
// (Keycloak in tier2, LaSuite apps in tier3).
func deployCNPGOperator(ctx context.Context, cfg *config.Config) error {
	return shared.DeployOperator(ctx, shared.OperatorDeploymentOptions{
		Name:      "CloudNative PG Operator",
		Namespace: constants.NamespaceCNPG,
		NamespaceLabels: map[string]string{
			"service-type": "nova",
		},
		ManifestURLs: []string{
			cfg.GetCNPGOperatorManifestURL(),
		},
		DeploymentName: "cnpg-controller-manager",
		TimeoutSeconds: 300,
	})
}
