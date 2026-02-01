// Package tier2 handles the deployment of NOVA Tier 2 (Platform Services).
package tier2

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// Note: CNPG Operator is deployed in tier1 for infrastructure consistency.
// This file contains CNPG cluster deployment functions for tier2/tier3.

// deployKeycloakCNPGCluster deploys a CNPG-managed PostgreSQL cluster for Keycloak.
func deployKeycloakCNPGCluster(ctx context.Context) error {
	// Generate or retrieve existing database password
	pwd, err := shared.GetOrGenerateSecret(ctx, keycloakNamespace, "keycloak-db-secret", "password", constants.DefaultPasswordLength)
	if err != nil {
		return fmt.Errorf("failed to get or generate database password: %w", err)
	}

	// Create database secret for CNPG cluster bootstrap
	ui.Info("Creating database credentials for CNPG cluster...")
	if err := k8s.CreateSecret(ctx, keycloakNamespace, "keycloak-db-secret", map[string]string{
		"username": "keycloak",
		"password": pwd,
	}); err != nil {
		return fmt.Errorf("failed to create keycloak db secret: %w", err)
	}

	// Apply CNPG Cluster CR
	ui.Info("Deploying CNPG PostgreSQL Cluster...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier2/cnpg/clusters/keycloak-db.yaml"); err != nil {
		return fmt.Errorf("failed to apply keycloak-db CNPG cluster: %w", err)
	}

	// Wait for CNPG cluster to be ready
	ui.Info("Waiting for CNPG PostgreSQL cluster to be ready...")
	if err := k8s.WaitForCondition(ctx, keycloakNamespace, "clusters.postgresql.cnpg.io/keycloak-db", "Ready", 300); err != nil {
		return fmt.Errorf("CNPG PostgreSQL cluster not ready: %w", err)
	}

	return nil
}

// DeployLaSuiteCNPGCluster deploys a CNPG-managed PostgreSQL cluster for LaSuite apps.
// This cluster contains databases for OpenGateLLM and Conversations.
func DeployLaSuiteCNPGCluster(ctx context.Context, namespace string) error {
	// Generate or retrieve existing database password
	pwd, err := shared.GetOrGenerateSecret(ctx, namespace, "lasuite-db-secret", "password", constants.DefaultPasswordLength)
	if err != nil {
		return fmt.Errorf("failed to get or generate LaSuite database password: %w", err)
	}

	// Create database secret for CNPG cluster bootstrap
	ui.Info("Creating LaSuite database credentials for CNPG cluster...")
	if err := k8s.CreateSecret(ctx, namespace, "lasuite-db-secret", map[string]string{
		"username": "lasuite",
		"password": pwd,
	}); err != nil {
		return fmt.Errorf("failed to create lasuite db secret: %w", err)
	}

	// Apply CNPG Cluster CR with namespace templating
	ui.Info("Deploying LaSuite CNPG PostgreSQL Cluster...")
	data := map[string]any{
		"Namespace": namespace,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/cnpg/clusters/lasuite-db.yaml", data); err != nil {
		return fmt.Errorf("failed to apply lasuite-db CNPG cluster: %w", err)
	}

	// Wait for CNPG cluster to be ready
	ui.Info("Waiting for LaSuite CNPG PostgreSQL cluster to be ready...")
	if err := k8s.WaitForCondition(ctx, namespace, "clusters.postgresql.cnpg.io/lasuite-db", "Ready", 300); err != nil {
		return fmt.Errorf("LaSuite CNPG PostgreSQL cluster not ready: %w", err)
	}

	return nil
}
