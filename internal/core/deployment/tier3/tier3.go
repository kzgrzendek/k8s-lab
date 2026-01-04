// Package tier3 handles the deployment of NOVA Tier 3 (Application Layer).
package tier3

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/helm"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// DeployTier3 deploys tier 3: Application layer (llm-d, Open WebUI, HELIX).
func DeployTier3(ctx context.Context, cfg *config.Config) error {
	ui.Header("Tier 3: Application Layer")

	// Add Helm repos
	if err := addTier3HelmRepos(ctx); err != nil {
		return fmt.Errorf("failed to add Tier 3 Helm repositories: %w", err)
	}

	steps := []string{
		"llm-d Model Service",
		"llm-d Inference Pool",
		"llm-d Gateway & Routing",
		"Open WebUI",
		"HELIX JupyterHub",
	}
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Helper wrapper for step execution
	runStep := func(name string, fn func() error) error {
		progress.StartStep(currentStep)
		if err := fn(); err != nil {
			progress.FailStep(currentStep, err)
			return err
		}
		progress.CompleteStep(currentStep)
		currentStep++
		return nil
	}

	// 1. Deploy llm-d Model Service
	if err := runStep("llm-d Model Service", func() error {
		return deployLLMD(ctx, cfg)
	}); err != nil {
		return err
	}

	// 2. Deploy llm-d Inference Pool
	if err := runStep("llm-d Inference Pool", func() error {
		return deployLLMDInferencePool(ctx)
	}); err != nil {
		return err
	}

	// 3. Deploy llm-d Gateway & Routing
	if err := runStep("llm-d Gateway & Routing", func() error {
		return deployLLMDGatewayAndRouting(ctx, cfg)
	}); err != nil {
		return err
	}

	// 4. Deploy Open WebUI
	if err := runStep("Open WebUI", func() error {
		return deployOpenWebUI(ctx, cfg)
	}); err != nil {
		return err
	}

	// 5. Deploy HELIX
	if err := runStep("HELIX JupyterHub", func() error {
		return deployHelix(ctx, cfg)
	}); err != nil {
		return err
	}

	progress.Complete()
	ui.Header("Tier 3 Deployment Complete")
	ui.Success("Application services are running")

	return nil
}

// addTier3HelmRepos adds Helm repositories required for Tier 3 deployment.
func addTier3HelmRepos(ctx context.Context) error {
	repos := map[string]string{
		"aphp-helix":         constants.HelmRepoAPHPHelix,
		"llm-d-modelservice": constants.HelmRepoLLMD,
		"open-webui":         constants.HelmRepoOpenWebUI,
	}

	helmClient := helm.NewClient("")

	ui.Info("Adding %d Helm repositories...", len(repos))
	for name, url := range repos {
		ui.Debug("  - Adding %s repository", name)
		if err := helmClient.AddRepository(ctx, name, url); err != nil {
			return fmt.Errorf("failed to add %s repository (%s): %w", name, url, err)
		}
	}

	// Update repos
	ui.Info("Updating repository indexes...")
	if err := helmClient.UpdateRepositories(ctx); err != nil {
		return fmt.Errorf("failed to update Helm repository indexes: %w", err)
	}

	return nil
}

// deployLLMD deploys the llm-d model service with vLLM.
func deployLLMD(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceLLMD, map[string]string{
		"trust-manager/inject-ca-secret": "enabled",
		"service-type":                   "llm",
	}); err != nil {
		return err
	}

	// Create HF token secret if provided
	if cfg.LLM.HfToken != "" {
		ui.Info("Creating Hugging Face token secret...")
		if err := k8s.CreateSecret(ctx, constants.NamespaceLLMD, "hf-token", map[string]string{
			"token": cfg.LLM.HfToken,
		}); err != nil {
			return fmt.Errorf("failed to create HF token secret: %w", err)
		}
	}

	// Choose values file based on GPU/CPU mode
	valuesPath := shared.GetLLMDValuesPath(cfg)
	mode := "GPU"
	if !cfg.IsGPUMode() {
		mode = "CPU"
	}
	ui.Info("Deploying llm-d in %s mode", mode)

	// Deploy llm-d via Helm
	// Note: Namespace is already created and labeled by EnsureNamespace above
	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "llmd",
		ChartRef:        "llm-d-modelservice/llm-d-modelservice",
		Namespace:       constants.NamespaceLLMD,
		ValuesPath:      valuesPath,
		Wait:            true,
		TimeoutSeconds:  3600,
		CreateNamespace: false, // Namespace already created by EnsureNamespace
		InfoMessage:     "Installing llm-d model service (this may take several minutes)...",
	})
}

// deployLLMDInferencePool deploys the Gateway API Inference Extension pool.
func deployLLMDInferencePool(ctx context.Context) error {
	ui.Info("Deploying llm-d inference pool...")

	// Deploy inference pool using OCI chart (v1.2.1)
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "llmd-qwen3-pool",
		ChartRef:        "oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool:v1.2.1",
		Namespace:       constants.NamespaceLLMD,
		ValuesPath:      "resources/core/deployment/tier3/llmd/inferencepools/helm/ip-llmd.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	}); err != nil {
		return err
	}

	// Apply inference objectives
	ui.Info("Applying inference objectives...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier3/llmd/inferenceobjectives/io-llmd.yaml"); err != nil {
		return fmt.Errorf("failed to apply inference objectives: %w", err)
	}

	return nil
}

// deployLLMDGatewayAndRouting deploys the internal gateway and AI routing.
func deployLLMDGatewayAndRouting(ctx context.Context, cfg *config.Config) error {
	data := map[string]interface{}{
		"Domain": cfg.DNS.Domain,
	}

	// Apply certificate for internal gateway
	ui.Info("Creating internal gateway certificate...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/certificates/gateway-internal-llmd.yaml", data); err != nil {
		return fmt.Errorf("failed to apply llmd gateway certificate: %w", err)
	}

	// Apply internal gateway
	ui.Info("Creating internal llm-d gateway...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/gateways/k8s-internal-llmd.yaml", data); err != nil {
		return fmt.Errorf("failed to apply llmd gateway: %w", err)
	}

	// Apply AI Gateway Routes (CRITICAL: Do not modify these!)
	ui.Info("Applying AI Gateway routes...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier3/llmd/aigateawayroutes/aigr-llmd.yaml"); err != nil {
		return fmt.Errorf("failed to apply AI gateway routes: %w", err)
	}

	return nil
}

// deployOpenWebUI deploys Open WebUI with OIDC integration.
func deployOpenWebUI(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceOpenWebUI, map[string]string{
		"service-type":                   "lab",
		"trust-manager/inject-ca-secret": "enabled",
	}); err != nil {
		return err
	}

	// Apply secrets
	ui.Info("Creating Open WebUI secrets...")
	secrets := []string{
		"resources/core/deployment/tier3/openwebui/secrets/api-key.yaml",
		"resources/core/deployment/tier3/openwebui/secrets/oidc.yaml",
		"resources/core/deployment/tier3/openwebui/secrets/openwebui-secret-key.yaml",
	}

	for _, secret := range secrets {
		if err := k8s.ApplyYAML(ctx, secret); err != nil {
			return fmt.Errorf("failed to apply secret %s: %w", secret, err)
		}
	}

	// Deploy Open WebUI via Helm with templated values
	data := map[string]interface{}{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "open-webui",
		ChartRef:        "open-webui/open-webui",
		Namespace:       constants.NamespaceOpenWebUI,
		ValuesPath:      "resources/core/deployment/tier3/openwebui/helm/openwebui.yaml",
		TemplateData:    data,
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
		InfoMessage:     "Installing Open WebUI...",
	}); err != nil {
		return err
	}

	// Apply HTTPRoute
	ui.Info("Applying Open WebUI HTTPRoute...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/openwebui/httproutes/openwebui.yaml", data); err != nil {
		return fmt.Errorf("failed to apply Open WebUI HTTPRoute: %w", err)
	}

	return nil
}

// deployHelix deploys HELIX (JupyterHub) with OIDC integration.
func deployHelix(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceHelix, map[string]string{
		"trust-manager/inject-ca-secret": "enabled",
		"service-type":                   "lab",
	}); err != nil {
		return err
	}

	// Apply OIDC secret
	ui.Info("Creating HELIX OIDC secret...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier3/helix/secrets/oidc.yaml"); err != nil {
		return fmt.Errorf("failed to apply HELIX OIDC secret: %w", err)
	}

	// Deploy HELIX via Helm with templated values
	data := map[string]interface{}{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "helix",
		ChartRef:        "aphp-helix/helix",
		Namespace:       constants.NamespaceHelix,
		ValuesPath:      "resources/core/deployment/tier3/helix/helm/helix.yaml",
		TemplateData:    data,
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
		InfoMessage:     "Installing HELIX...",
	}); err != nil {
		return err
	}

	return nil
}
