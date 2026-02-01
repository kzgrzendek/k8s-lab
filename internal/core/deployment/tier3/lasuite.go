// Package tier3 handles the deployment of NOVA Tier 3 (Application Layer).
// This file contains deployers for the LaSuite profile (French government AI stack).
package tier3

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier2"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// DeployLaSuiteInfrastructure deploys shared infrastructure for LaSuite apps.
// This includes Qdrant (vector DB), Garage S3, shared PostgreSQL, and shared Redis.
// Called before deploying individual LaSuite apps.
func DeployLaSuiteInfrastructure(ctx context.Context, cfg *config.Config) error {
	ui.Info("Deploying LaSuite shared infrastructure...")

	// 1. Deploy Qdrant vector database (for OpenGateLLM)
	if err := DeployQdrant(ctx, cfg); err != nil {
		return fmt.Errorf("failed to deploy Qdrant: %w", err)
	}

	// 2. Deploy Garage S3 storage (for Conversations file uploads)
	if err := tier2.DeployGarage(ctx, cfg); err != nil {
		return fmt.Errorf("failed to deploy Garage S3: %w", err)
	}

	// 3. Create LaSuite namespace for shared database and redis
	// Using opengatellm namespace as the primary location for shared infra
	laSuiteNamespace := constants.NamespaceOpenGateLLM
	if err := shared.EnsureNamespace(ctx, laSuiteNamespace, map[string]string{
		"service-type":                   "nova",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return fmt.Errorf("failed to ensure LaSuite namespace: %w", err)
	}

	// 4. Deploy shared PostgreSQL cluster (contains opengatellm and conversations DBs)
	if err := tier2.DeployLaSuiteCNPGCluster(ctx, laSuiteNamespace); err != nil {
		return fmt.Errorf("failed to deploy LaSuite PostgreSQL: %w", err)
	}

	// 5. Deploy shared Redis cluster
	if err := tier2.DeployLaSuiteRedisCluster(ctx, laSuiteNamespace); err != nil {
		return fmt.Errorf("failed to deploy LaSuite Redis: %w", err)
	}

	ui.Success("LaSuite shared infrastructure deployed")
	return nil
}

// deployOpenGateLLM deploys the OpenGateLLM stack from etalab-ia.
// OpenGateLLM is an API gateway for LLM services, providing authentication,
// rate limiting, and routing for multiple LLM backends.
func deployOpenGateLLM(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceOpenGateLLM, map[string]string{
		"service-type":                   "nova",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return err
	}

	// Create OIDC secret using shared constants (same values as configured in Keycloak realm)
	ui.Info("Creating OpenGateLLM OIDC secret...")
	if err := k8s.CreateSecret(ctx, constants.NamespaceOpenGateLLM, "oidc", map[string]string{
		"client-id":     constants.OIDCOpenGateLLM.ID,
		"client-secret": constants.OIDCOpenGateLLM.Secret,
	}); err != nil {
		return fmt.Errorf("failed to create OIDC secret: %w", err)
	}

	// Deploy OpenGateLLM via Helm with templated values
	data := map[string]any{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	// Default values path (will be created in Phase 6)
	defaultValuesPath := "resources/core/deployment/tier3/opengatellm/helm/opengatellm.yaml"

	// Load custom values if specified
	customValues, err := shared.LoadAndTemplateCustomValues(cfg.Versions.Tier3.OpenGateLLM.CustomValuesPath, data)
	if err != nil {
		return fmt.Errorf("failed to load custom values: %w", err)
	}
	if customValues != nil {
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.OpenGateLLM.CustomValuesPath)
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "opengatellm",
		ChartRef:        cfg.Versions.Tier3.OpenGateLLM.ChartRef(),
		Version:         cfg.Versions.Tier3.OpenGateLLM.GetVersion(),
		Namespace:       constants.NamespaceOpenGateLLM,
		ValuesPath:      defaultValuesPath,
		Values:          customValues,
		TemplateData:    data,
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
		InfoMessage:     "Installing OpenGateLLM...",
	}); err != nil {
		return err
	}

	// Apply HTTPRoute for OpenGateLLM
	ui.Info("Applying OpenGateLLM HTTPRoute...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/opengatellm/httproutes/opengatellm.yaml", data); err != nil {
		return fmt.Errorf("failed to apply OpenGateLLM HTTPRoute: %w", err)
	}

	return nil
}

// deployConversations deploys the Conversations app from suitenumerique.
// Conversations is a chat application that integrates with OpenGateLLM
// for LLM-powered conversations.
func deployConversations(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceConversations, map[string]string{
		"service-type":                   "nova",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return err
	}

	// Copy S3 credentials from Garage namespace for file uploads
	ui.Info("Copying S3 credentials to Conversations namespace...")
	if err := tier2.CopyGarageCredentialsToNamespace(ctx, constants.NamespaceConversations); err != nil {
		return fmt.Errorf("failed to copy S3 credentials: %w", err)
	}

	// Create OIDC secret using shared constants (same values as configured in Keycloak realm)
	ui.Info("Creating Conversations OIDC secret...")
	if err := k8s.CreateSecret(ctx, constants.NamespaceConversations, "oidc", map[string]string{
		"client-id":     constants.OIDCConversations.ID,
		"client-secret": constants.OIDCConversations.Secret,
	}); err != nil {
		return fmt.Errorf("failed to create OIDC secret: %w", err)
	}

	// Deploy Conversations via Helm with templated values
	data := map[string]any{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	// Default values path (will be created in Phase 6)
	defaultValuesPath := "resources/core/deployment/tier3/conversations/helm/conversations.yaml"

	// Load custom values if specified
	customValues, err := shared.LoadAndTemplateCustomValues(cfg.Versions.Tier3.Conversations.CustomValuesPath, data)
	if err != nil {
		return fmt.Errorf("failed to load custom values: %w", err)
	}
	if customValues != nil {
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.Conversations.CustomValuesPath)
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "conversations",
		ChartRef:        cfg.Versions.Tier3.Conversations.ChartRef(),
		Version:         cfg.Versions.Tier3.Conversations.GetVersion(),
		Namespace:       constants.NamespaceConversations,
		ValuesPath:      defaultValuesPath,
		Values:          customValues,
		TemplateData:    data,
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
		InfoMessage:     "Installing Conversations...",
	}); err != nil {
		return err
	}

	// Apply HTTPRoute for Conversations
	ui.Info("Applying Conversations HTTPRoute...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/conversations/httproutes/conversations.yaml", data); err != nil {
		return fmt.Errorf("failed to apply Conversations HTTPRoute: %w", err)
	}

	return nil
}
