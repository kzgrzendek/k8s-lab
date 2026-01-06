// Package tier3 handles the deployment of NOVA Tier 3 (Application Layer).
package tier3

import (
	"context"
	"fmt"
	"time"

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
	repos := map[string]string{
		"aphp-helix":         constants.HelmRepoAPHPHelix,
		"llm-d-modelservice": constants.HelmRepoLLMD,
		"open-webui":         constants.HelmRepoOpenWebUI,
	}
	if err := shared.AddHelmRepositories(ctx, repos); err != nil {
		return fmt.Errorf("failed to add Tier 3 Helm repositories: %w", err)
	}

	steps := []string{
		"llm-d Model Service",
		"llm-d Inference Pool",
		"llm-d Gateway & Routing",
		"Open WebUI",
		"HELIX JupyterHub",
	}
	runner := shared.NewStepRunner(steps)

	// 1. Deploy llm-d Model Service
	if err := runner.RunStep("llm-d Model Service", func() error {
		return deployLLMD(ctx, cfg)
	}); err != nil {
		return err
	}

	// 2. Deploy llm-d Inference Pool
	if err := runner.RunStep("llm-d Inference Pool", func() error {
		return deployLLMDInferencePool(ctx, cfg)
	}); err != nil {
		return err
	}

	// 3. Deploy llm-d Gateway & Routing
	if err := runner.RunStep("llm-d Gateway & Routing", func() error {
		return deployLLMDGatewayAndRouting(ctx, cfg)
	}); err != nil {
		return err
	}

	// 4. Deploy Open WebUI
	if err := runner.RunStep("Open WebUI", func() error {
		return deployOpenWebUI(ctx, cfg)
	}); err != nil {
		return err
	}

	// 5. Deploy HELIX
	if err := runner.RunStep("HELIX JupyterHub", func() error {
		return deployHelix(ctx, cfg)
	}); err != nil {
		return err
	}

	runner.Complete()
	ui.Header("Tier 3 Deployment Complete")
	ui.Success("Application services are running")

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

	// Note: HF token secret is created in tier1 during model warmup setup
	// Both the model warmup Job and llmd will use the same "huggingface-token" secret

	// Choose default values file based on GPU/CPU mode
	defaultValuesPath := shared.GetLLMDValuesPath(cfg)
	mode := "GPU"
	if !cfg.IsGPUMode() {
		mode = "CPU"
	}
	ui.Info("Deploying llm-d in %s mode", mode)

	// Load custom values if specified (will be merged on top of default values)
	var customValues map[string]any
	if cfg.Versions.Tier3.LLMD.CustomValuesPath != "" {
		var err error
		customValues, err = helm.LoadValues(cfg.Versions.Tier3.LLMD.CustomValuesPath)
		if err != nil {
			return fmt.Errorf("failed to load custom values from %s: %w", cfg.Versions.Tier3.LLMD.CustomValuesPath, err)
		}
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.LLMD.CustomValuesPath)
	}

	// Wait for model warmup Job to complete (if it exists)
	modelSlug := cfg.GetModelSlug()
	warmupJobName := fmt.Sprintf("model-warmup-%s", modelSlug)
	pvcName := fmt.Sprintf("llm-model-%s", modelSlug)

	if jobExists, _ := k8s.ResourceExists(ctx, "job", warmupJobName, constants.NamespaceLLMD); jobExists {
		ui.Info("Waiting for model warmup to complete (this may take 10-20 minutes)...")
		if err := waitForModelWarmup(ctx, warmupJobName); err != nil {
			ui.Warn("Model warmup Job incomplete: %v", err)
			ui.Warn("llm-d will download the model on first startup (slower)")
			pvcName = "" // Don't mount PVC if warmup failed
		} else {
			ui.Success("Model pre-warmed and ready on PVC: %s", pvcName)
		}
	} else {
		ui.Debug("No model warmup Job found, llm-d will download on startup")
		pvcName = "" // No PVC to mount
	}

	// Prepare template data for model configuration
	templateData := map[string]string{
		"ModelName":    cfg.GetModelName(),
		"ModelSlug":    modelSlug,
		"ModelURI":     cfg.GetModelURI(),
		"ModelPVCName": pvcName, // Empty if no warmup, otherwise PVC name
	}

	// Deploy llm-d via Helm
	// Note: Namespace is already created and labeled by EnsureNamespace above
	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "llmd",
		ChartRef:        cfg.Versions.Tier3.LLMD.ChartRef(),
		Version:         cfg.Versions.Tier3.LLMD.GetVersion(),
		Namespace:       constants.NamespaceLLMD,
		ValuesPath:      defaultValuesPath,
		Values:          customValues, // Custom values will be merged on top
		TemplateData:    templateData, // Template data for model configuration
		Wait:            true,
		TimeoutSeconds:  3600,
		CreateNamespace: false, // Namespace already created by EnsureNamespace
		InfoMessage:     fmt.Sprintf("Installing llm-d with model %s (this may take several minutes)...", cfg.GetModelName()),
	})
}

// deployLLMDInferencePool deploys the Gateway API Inference Extension pool.
func deployLLMDInferencePool(ctx context.Context, cfg *config.Config) error {
	ui.Info("Deploying llm-d inference pool...")

	// Always use default values path
	defaultValuesPath := "resources/core/deployment/tier3/llmd/inferencepools/helm/ip-llmd.yaml"

	// Load custom values if specified (will be merged on top of default values)
	var customValues map[string]any
	if cfg.Versions.Tier3.InferencePool.CustomValuesPath != "" {
		var err error
		customValues, err = helm.LoadValues(cfg.Versions.Tier3.InferencePool.CustomValuesPath)
		if err != nil {
			return fmt.Errorf("failed to load custom values from %s: %w", cfg.Versions.Tier3.InferencePool.CustomValuesPath, err)
		}
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.InferencePool.CustomValuesPath)
	}

	// Deploy inference pool using OCI chart with dynamic model configuration
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     fmt.Sprintf("llmd-%s-pool", cfg.GetModelSlug()),
		ChartRef:        cfg.Versions.Tier3.InferencePool.ChartRef(),
		Version:         cfg.Versions.Tier3.InferencePool.GetVersion(),
		Namespace:       constants.NamespaceLLMD,
		ValuesPath:      defaultValuesPath,
		Values:          customValues, // Custom values will be merged on top
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	}); err != nil {
		return err
	}

	// Apply inference objectives with model configuration
	ui.Info("Applying inference objectives...")
	modelData := map[string]string{
		"ModelSlug": cfg.GetModelSlug(),
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/inferenceobjectives/io-llmd.yaml", modelData); err != nil {
		return fmt.Errorf("failed to apply inference objectives: %w", err)
	}

	return nil
}

// deployLLMDGatewayAndRouting deploys the internal gateway and AI routing.
func deployLLMDGatewayAndRouting(ctx context.Context, cfg *config.Config) error {
	data := map[string]any{
		"Domain": cfg.DNS.Domain,
	}

	// Apply certificate for internal gateway
	ui.Info("Creating internal gateway certificate...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/certificates/gateway-nova-internal.yaml", data); err != nil {
		return fmt.Errorf("failed to apply llmd gateway certificate: %w", err)
	}

	// Apply internal gateway
	ui.Info("Creating internal llm-d gateway...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/gateways/gateway-nova-internal.yaml", data); err != nil {
		return fmt.Errorf("failed to apply llmd gateway: %w", err)
	}

	// Prepare model-specific data for templating
	modelData := map[string]string{
		"ModelSlug": cfg.GetModelSlug(),
		"ModelName": cfg.GetModelName(),
	}

	// Apply discovery service for model listing (with model-specific selector)
	ui.Info("Creating llm-d discovery service...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/services/llmd-discovery.yaml", modelData); err != nil {
		return fmt.Errorf("failed to apply llmd discovery service: %w", err)
	}

	// Apply HTTPRoute for discovery endpoints (/v1/models, etc.)
	ui.Info("Creating llm-d discovery route...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier3/llmd/httproutes/llmd-discovery.yaml"); err != nil {
		return fmt.Errorf("failed to apply llmd discovery route: %w", err)
	}

	// Apply AI Gateway Routes for inference requests (with model-specific configuration)
	ui.Info("Applying AI Gateway routes...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier3/llmd/aigateawayroutes/aigr-llmd.yaml", modelData); err != nil {
		return fmt.Errorf("failed to apply AI gateway routes: %w", err)
	}

	return nil
}

// deployOpenWebUI deploys Open WebUI with OIDC integration.
func deployOpenWebUI(ctx context.Context, cfg *config.Config) error {
	// Ensure namespace exists and is labeled
	if err := shared.EnsureNamespace(ctx, constants.NamespaceOpenWebUI, map[string]string{
		"service-type":                   "nova",
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
	data := map[string]any{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	// Always use default values path
	defaultValuesPath := "resources/core/deployment/tier3/openwebui/helm/openwebui.yaml"

	// Load custom values if specified (will be merged on top of default values)
	customValues, err := shared.LoadAndTemplateCustomValues(cfg.Versions.Tier3.OpenWebUI.CustomValuesPath, data)
	if err != nil {
		return fmt.Errorf("failed to load custom values: %w", err)
	}
	if customValues != nil {
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.OpenWebUI.CustomValuesPath)
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "open-webui",
		ChartRef:        cfg.Versions.Tier3.OpenWebUI.ChartRef(),
		Version:         cfg.Versions.Tier3.OpenWebUI.GetVersion(),
		Namespace:       constants.NamespaceOpenWebUI,
		ValuesPath:      defaultValuesPath,
		Values:          customValues, // Custom values will be merged on top
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
		"service-type":                   "nova",
	}); err != nil {
		return err
	}

	// Apply OIDC secret
	ui.Info("Creating HELIX OIDC secret...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier3/helix/secrets/oidc.yaml"); err != nil {
		return fmt.Errorf("failed to apply HELIX OIDC secret: %w", err)
	}

	// Deploy HELIX via Helm with templated values
	data := map[string]any{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	// Always use default values path
	defaultValuesPath := "resources/core/deployment/tier3/helix/helm/helix.yaml"

	// Load custom values if specified (will be merged on top of default values)
	customValues, err := shared.LoadAndTemplateCustomValues(cfg.Versions.Tier3.Helix.CustomValuesPath, data)
	if err != nil {
		return fmt.Errorf("failed to load custom values: %w", err)
	}
	if customValues != nil {
		ui.Info("Using custom values from: %s", cfg.Versions.Tier3.Helix.CustomValuesPath)
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "helix",
		ChartRef:        cfg.Versions.Tier3.Helix.ChartRef(),
		Version:         cfg.Versions.Tier3.Helix.GetVersion(),
		Namespace:       constants.NamespaceHelix,
		ValuesPath:      defaultValuesPath,
		Values:          customValues, // Custom values will be merged on top
		TemplateData:    data,
		Wait:            false, // Don't wait - PVC uses WaitForFirstConsumer and stays pending
		TimeoutSeconds:  600,
		CreateNamespace: true,
		InfoMessage:     "Installing HELIX...",
	}); err != nil {
		return err
	}

	// Wait for essential HELIX pods (hub and proxy) to be ready
	// Note: The jupyterlab PVC will remain pending until a user spawns a notebook
	ui.Info("Waiting for HELIX hub and proxy pods to be ready...")

	// Wait for hub pod
	hubPodName, err := k8s.GetFirstPodName(ctx, constants.NamespaceHelix, "component=hub")
	if err != nil {
		ui.Warn("Could not find HELIX hub pod: %v", err)
	} else {
		if err := k8s.WaitForPodReady(ctx, constants.NamespaceHelix, hubPodName, 300); err != nil {
			ui.Warn("HELIX hub pod not ready: %v", err)
		} else {
			ui.Info("HELIX hub pod is ready")
		}
	}

	// Wait for proxy pod
	proxyPodName, err := k8s.GetFirstPodName(ctx, constants.NamespaceHelix, "component=proxy")
	if err != nil {
		ui.Warn("Could not find HELIX proxy pod: %v", err)
	} else {
		if err := k8s.WaitForPodReady(ctx, constants.NamespaceHelix, proxyPodName, 300); err != nil {
			ui.Warn("HELIX proxy pod not ready: %v", err)
		} else {
			ui.Info("HELIX proxy pod is ready")
		}
	}

	return nil
}

// waitForModelWarmup waits for the model warmup Job to complete.
// This is called before deploying llmd to ensure the model is pre-downloaded.
func waitForModelWarmup(ctx context.Context, jobName string) error {
	namespace := constants.NamespaceLLMD
	const timeout = 30 * time.Minute

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(time.Duration(constants.DefaultCheckInterval) * time.Second)
	defer ticker.Stop()

	lastLog := time.Now()
	const logInterval = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for warmup Job %s to complete", jobName)
		case <-ticker.C:
			// Check Job status
			completed, err := k8s.IsJobComplete(ctx, jobName, namespace)
			if err != nil {
				return fmt.Errorf("error checking Job status: %w", err)
			}
			if completed {
				return nil
			}

			// Log progress every 30 seconds
			if time.Since(lastLog) >= logInterval {
				ui.Info("Model warmup in progress...")
				lastLog = time.Now()
			}
		}
	}
}
