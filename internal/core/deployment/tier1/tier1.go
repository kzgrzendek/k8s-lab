package tier1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/core/errors"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
	"github.com/kzgrzendek/nova/internal/tools/crypto"
	"github.com/kzgrzendek/nova/internal/tools/exec"
	"github.com/kzgrzendek/nova/internal/tools/helm"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// DeployTier1 deploys tier 1: Infrastructure layer (Security, Certificates, GPU, Gateways).
func DeployTier1(ctx context.Context, cfg *config.Config) error {
	// Define deployment steps
	steps := []string{
		"Prerequisites Check",
		"Helm Repositories",
		"Falco Security",
		"Cert Manager",
		"Trust Manager",
		"GPU Support",
		"Envoy AI Gateway",
		"Envoy Gateway",
		"Nova Namespace & RBAC",
	}

	// Create step runner with progress tracking
	runner := shared.NewStepRunner(steps)

	// Step 1: Check prerequisites
	if err := runner.RunStep("Prerequisites Check", func() error {
		return checkPrerequisites(ctx)
	}); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	// Step 2: Add Helm repositories
	if err := runner.RunStep("Helm Repositories", func() error {
		repos := map[string]string{
			"falcosecurity": constants.HelmRepoFalco,
			"jetstack":      constants.HelmRepoJetstack,
			"dandydev":      constants.HelmRepoDandyDev,
			"nvidia":        constants.HelmRepoNvidia,
			"intel":         constants.HelmRepoIntel,
			"nfd":           constants.HelmRepoNFD,
		}
		return shared.AddHelmRepositories(ctx, repos)
	}); err != nil {
		return fmt.Errorf("failed to add Helm repositories: %w", err)
	}

	// Step 3: Falco Security
	if err := runner.RunStep("Falco Security", func() error {
		return deployFalco(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 4: Cert Manager (must be before GPU Support - Intel Device Plugin requires it)
	if err := runner.RunStep("Cert Manager", func() error {
		return deployCertManager(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 5: Trust Manager
	if err := runner.RunStep("Trust Manager", func() error {
		return deployTrustManager(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 6: GPU Support (NVIDIA GPU Operator or Intel Device Plugin)
	if err := runner.RunStep("GPU Support", func() error {
		return deployGPUSupport(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 7: Envoy AI Gateway
	if err := runner.RunStep("Envoy AI Gateway", func() error {
		return deployEnvoyAIGateway(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 10: Envoy Gateway
	if err := runner.RunStep("Envoy Gateway", func() error {
		return deployEnvoyGateway(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 11: Nova Namespace & RBAC
	if err := runner.RunStep("Nova Namespace & RBAC", func() error {
		return setupNovaNamespace(ctx)
	}); err != nil {
		return err
	}

	// Mark all steps complete
	runner.Complete()

	ui.Header("Tier 1 Deployment Complete")
	ui.Success("All infrastructure components deployed successfully")

	return nil
}

func checkPrerequisites(ctx context.Context) error {
	// Check kubectl
	if !exec.Check(ctx, "kubectl", "version", "--client") {
		return errors.NewNotAvailableWithMessage("kubectl", "please ensure kubectl is installed and in PATH")
	}

	// Check helm
	if !exec.Check(ctx, "helm", "version") {
		return errors.NewNotAvailableWithMessage("helm", "please ensure Helm is installed and in PATH")
	}

	return nil
}

// deployFalco deploys Falco security auditor.
func deployFalco(ctx context.Context, cfg *config.Config) error {
	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "falco",
		ChartRef:        cfg.Versions.Tier1.Falco.ChartRef(),
		Version:         cfg.Versions.Tier1.Falco.GetVersion(),
		Namespace:       "falco",
		ValuesPath:      "resources/core/deployment/tier1/falco/values.yaml",
		Wait:            true,
		TimeoutSeconds:  600,
		InfoMessage:     "Installing Falco Security (may take a few minutes)...",
		CreateNamespace: true,
	})
}

func deployCertManager(ctx context.Context, cfg *config.Config) error {
	// Label namespace for CA injection
	if err := k8s.CreateNamespace(ctx, "cert-manager"); err != nil {
		return fmt.Errorf("failed to create cert-manager namespace: %w", err)
	}

	if err := k8s.LabelNamespace(ctx, "cert-manager", "trust-manager/inject-ca-secret", "enabled"); err != nil {
		return fmt.Errorf("failed to label cert-manager namespace: %w", err)
	}

	// Deploy Cert Manager
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "cert-manager",
		ChartRef:       cfg.Versions.Tier1.CertManager.ChartRef(),
		Version:        cfg.Versions.Tier1.CertManager.GetVersion(),
		Namespace:      "cert-manager",
		ValuesPath:     "resources/core/deployment/tier1/cert-manager/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Cert Manager (may take a few minutes)...",
	}); err != nil {
		return err
	}

	// Wait for cert-manager-webhook to be ready
	ui.Info("Waiting for cert-manager webhook...")
	if err := k8s.WaitForDeploymentReady(ctx, "cert-manager", "cert-manager-webhook", 300); err != nil {
		return fmt.Errorf("cert-manager webhook failed to become ready: %w", err)
	}

	// Apply mkcert CA secret
	ui.Info("Creating mkcert CA secret...")
	caSecretPath := pki.GetDefaultSecretPath()
	if err := k8s.ApplyYAML(ctx, caSecretPath); err != nil {
		return fmt.Errorf("failed to apply CA secret: %w", err)
	}

	// Apply ClusterIssuers (mkcert CA)
	ui.Info("Creating ClusterIssuers...")
	domainData := map[string]any{
		"Domain": cfg.DNS.Domain,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/cert-manager/clusterissuers/nova-issuer.yaml", domainData); err != nil {
		return fmt.Errorf("failed to create cluster issuers: %w", err)
	}

	return nil
}

// deployTrustManager installs trust-manager and handles CA distribution.
func deployTrustManager(ctx context.Context, cfg *config.Config) error {
	// Deploy Trust Manager
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "trust-manager",
		ChartRef:       cfg.Versions.Tier1.TrustManager.ChartRef(),
		Version:        cfg.Versions.Tier1.TrustManager.GetVersion(),
		Namespace:      "cert-manager",
		ValuesPath:     "resources/core/deployment/tier1/trust-manager/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Trust Manager...",
	}); err != nil {
		return err
	}

	// Wait for trust-manager to be ready
	ui.Info("Waiting for trust-manager pods...")
	if err := k8s.WaitForDeploymentReady(ctx, "cert-manager", "trust-manager", 300); err != nil {
		return fmt.Errorf("trust-manager failed to become ready: %w", err)
	}

	// Wait for trust-manager webhook endpoints to be ready
	ui.Info("Waiting for trust-manager webhook endpoints...")
	if err := k8s.WaitForEndpoints(ctx, "cert-manager", "trust-manager", 300); err != nil {
		return fmt.Errorf("trust-manager webhook endpoints not ready: %w", err)
	}

	// Wait for Cilium to fully propagate network routing for the webhook
	// The webhook endpoint is ready but iptables rules may not be fully in place
	ui.Info("Waiting for webhook network routing to stabilize...")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
	}

	// Label kube-system namespace BEFORE creating trust bundle
	// This ensures trust-manager knows where to inject the CA secret
	ui.Info("Labeling kube-system namespace for CA injection and routing...")
	if err := k8s.LabelNamespace(ctx, "kube-system", "trust-manager/inject-ca-secret", "enabled"); err != nil {
		return fmt.Errorf("failed to label kube-system namespace for CA: %w", err)
	}
	if err := k8s.LabelNamespace(ctx, "kube-system", "service-type", "nova"); err != nil {
		return fmt.Errorf("failed to label kube-system namespace for routing: %w", err)
	}

	// Apply trust bundles with retry (Cilium network routing may not be fully ready)
	ui.Info("Creating trust bundles...")
	if err := shared.ApplyTemplateWithRetry(ctx, "resources/core/deployment/tier1/trust-manager/bundles/nova-ca-bundle.yaml", nil, 5, 3*time.Second); err != nil {
		return fmt.Errorf("failed to create trust bundles: %w", err)
	}

	// Wait for secret injection to propagate
	ui.Info("Waiting for CA secret injection...")
	if err := k8s.WaitForSecret(ctx, "kube-system", "nova-ca-secret", 120); err != nil {
		return fmt.Errorf("CA secret not injected into kube-system: %w", err)
	}

	// Upgrade Cilium to mount CA
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "cilium",
		ChartRef:       cfg.Versions.Tier1.Cilium.ChartRef(),
		Version:        cfg.Versions.Tier1.Cilium.GetVersion(),
		Namespace:      "kube-system",
		ValuesPath:     "resources/core/deployment/tier1/trust-manager/cilium-envoy-mount-ca.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		ReuseValues:    true,
	}); err != nil {
		return fmt.Errorf("failed to upgrade Cilium with CA mount: %w", err)
	}

	return nil
}

func deployEnvoyAIGateway(ctx context.Context, cfg *config.Config) error {
	// Create namespace
	if err := k8s.CreateNamespace(ctx, "envoy-ai-gateway-system"); err != nil {
		return fmt.Errorf("failed to create envoy-ai-gateway-system namespace: %w", err)
	}

	// Label namespace for CA injection
	if err := k8s.LabelNamespace(ctx, "envoy-ai-gateway-system", "trust-manager/inject-ca-secret", "enabled"); err != nil {
		return fmt.Errorf("failed to label envoy-ai-gateway-system namespace: %w", err)
	}

	// Install Inference Extension CRDs first
	ui.Info("Installing Gateway API Inference Extension CRDs...")
	if err := k8s.ApplyURL(ctx, cfg.GetGatewayAPIInferenceExtensionManifestURL()); err != nil {
		return fmt.Errorf("failed to install inference extension CRDs: %w", err)
	}

	// Install AI Gateway CRDs using OCI
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "aieg-crd",
		ChartRef:       cfg.Versions.Tier1.EnvoyAiGatewayCRDs.ChartRef(),
		Version:        cfg.Versions.Tier1.EnvoyAiGatewayCRDs.GetVersion(),
		Namespace:      "envoy-ai-gateway-system",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Envoy AI Gateway CRDs...",
	}); err != nil {
		return err
	}

	// Install AI Gateway using OCI
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "aieg",
		ChartRef:       cfg.Versions.Tier1.EnvoyAiGateway.ChartRef(),
		Version:        cfg.Versions.Tier1.EnvoyAiGateway.GetVersion(),
		Namespace:      "envoy-ai-gateway-system",
		ValuesPath:     "resources/core/deployment/tier1/envoy-ai-gateway/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Envoy AI Gateway...",
	}); err != nil {
		return err
	}

	return nil
}

func deployEnvoyGateway(ctx context.Context, cfg *config.Config) error {
	// Create namespace
	if err := k8s.CreateNamespace(ctx, "envoy-gateway-system"); err != nil {
		return fmt.Errorf("failed to create envoy-gateway-system namespace: %w", err)
	}

	// Label namespace for CA injection
	if err := k8s.LabelNamespace(ctx, "envoy-gateway-system", "trust-manager/inject-ca-secret", "enabled"); err != nil {
		return fmt.Errorf("failed to label envoy-gateway-system namespace: %w", err)
	}

	// Create Redis secret with a generated password
	ui.Info("Creating Redis authentication secret...")
	redisPassword, err := crypto.GenerateRandomPassword(32)
	if err != nil {
		return fmt.Errorf("failed to generate Redis password: %w", err)
	}
	if err := k8s.CreateSecret(ctx, "envoy-gateway-system", "redis", map[string]string{
		"redis-password": redisPassword,
	}); err != nil {
		return fmt.Errorf("failed to create Redis secret: %w", err)
	}

	// Install Redis for rate limiting
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "redis-ha",
		ChartRef:       cfg.Versions.Tier1.Redis.ChartRef(),
		Version:        cfg.Versions.Tier1.Redis.GetVersion(),
		Namespace:      "envoy-gateway-system",
		ValuesPath:     "resources/core/deployment/tier1/redis/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Redis backend...",
	}); err != nil {
		return err
	}

	// Install Envoy Gateway using OCI
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "envoy-gateway",
		ChartRef:       cfg.Versions.Tier1.EnvoyGateway.ChartRef(),
		Version:        cfg.Versions.Tier1.EnvoyGateway.GetVersion(),
		Namespace:      "envoy-gateway-system",
		ValuesPath:     "resources/core/deployment/tier1/envoy-gateway/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Envoy Gateway...",
	}); err != nil {
		return err
	}

	// Apply Envoy Gateway configuration manifests
	domainData := map[string]any{
		"Domain":     cfg.DNS.Domain,
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	ui.Info("Applying Envoy Gateway certificates...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/envoy-gateway/certificates/nova-gateway-cert.yaml", domainData); err != nil {
		return fmt.Errorf("failed to apply envoy gateway certificates: %w", err)
	}

	ui.Info("Applying Envoy proxies configuration...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/envoy-gateway/envoyproxies/nova-envoy-proxy.yaml", nil); err != nil {
		return fmt.Errorf("failed to apply envoy proxies: %w", err)
	}

	ui.Info("Applying Gateway classes...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/envoy-gateway/gatewayclasses/nova.yaml", nil); err != nil {
		return fmt.Errorf("failed to apply gateway classes: %w", err)
	}

	ui.Info("Applying Gateways...")
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/envoy-gateway/gateways/nova-https.yaml", domainData); err != nil {
		return fmt.Errorf("failed to apply https gateway: %w", err)
	}

	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/envoy-gateway/gateways/nova-passthrough.yaml", domainData); err != nil {
		return fmt.Errorf("failed to apply passthrough gateway: %w", err)
	}

	return nil
}

// setupNovaNamespace creates the nova namespace, RBAC for developer and user roles, and kubectl contexts.
func setupNovaNamespace(ctx context.Context) error {
	const novaNamespace = "nova"
	const developerContextName = "developer"
	const developerServiceAccount = "developer"
	const userServiceAccount = "user"

	ui.Info("Creating nova namespace and RBAC...")

	// Apply namespace
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/namespace.yaml"); err != nil {
		return fmt.Errorf("failed to create nova namespace: %w", err)
	}

	// Apply developer service account, role, and binding
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/developer-serviceaccount.yaml"); err != nil {
		return fmt.Errorf("failed to create developer service account: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/developer-role.yaml"); err != nil {
		return fmt.Errorf("failed to create developer role: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/developer-rolebinding.yaml"); err != nil {
		return fmt.Errorf("failed to create developer role binding: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/developer-secret.yaml"); err != nil {
		return fmt.Errorf("failed to create developer token secret: %w", err)
	}

	// Apply user service account, role, and binding
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/user-serviceaccount.yaml"); err != nil {
		return fmt.Errorf("failed to create user service account: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/user-role.yaml"); err != nil {
		return fmt.Errorf("failed to create user role: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/user-rolebinding.yaml"); err != nil {
		return fmt.Errorf("failed to create user role binding: %w", err)
	}

	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nova-rbac/user-secret.yaml"); err != nil {
		return fmt.Errorf("failed to create user token secret: %w", err)
	}

	// Wait for the developer token to be populated
	ui.Info("Waiting for service account tokens...")
	if err := k8s.WaitForSecret(ctx, novaNamespace, developerServiceAccount+"-token", 30); err != nil {
		return fmt.Errorf("failed waiting for developer token: %w", err)
	}

	if err := k8s.WaitForSecret(ctx, novaNamespace, userServiceAccount+"-token", 30); err != nil {
		return fmt.Errorf("failed waiting for user token: %w", err)
	}

	// Create kubectl contexts
	ui.Info("Creating kubectl contexts...")

	// Create developer context (if it doesn't exist)
	if !k8s.ContextExists(ctx, developerContextName) {
		if err := k8s.CreateKubectlContext(ctx, developerContextName, novaNamespace, developerServiceAccount); err != nil {
			return fmt.Errorf("failed to create developer kubectl context: %w", err)
		}
		ui.Info("Created kubectl context '%s'", developerContextName)
	} else {
		ui.Info("Developer context already exists - skipping")
	}

	ui.Success("Nova namespace and RBAC configured")
	return nil
}

// deployGPUSupport deploys the appropriate GPU support based on GPU mode.
func deployGPUSupport(ctx context.Context, cfg *config.Config) error {
	gpuMode := cfg.GetGPUMode()

	switch gpuMode {
	case config.GPUModeNVIDIA:
		return deployNVIDIAGPUOperator(ctx, cfg)
	case config.GPUModeIntel:
		return deployIntelDevicePlugin(ctx, cfg)
	default:
		ui.Info("GPU mode: auto (should be resolved during setup)")
		return nil
	}
}

// deployNVIDIAGPUOperator deploys the NVIDIA GPU Operator.
func deployNVIDIAGPUOperator(ctx context.Context, cfg *config.Config) error {
	ui.Info("GPU mode: NVIDIA")

	// Skip if already installed to avoid hanging on pre-upgrade hooks (common with this operator)
	helmClient := helm.NewClient("nvidia-gpu-operator")
	if exists, _ := helmClient.ReleaseExists(ctx, "gpu-operator", "nvidia-gpu-operator"); exists {
		ui.Info("NVIDIA GPU Operator already installed - skipping")
		return nil
	}

	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "gpu-operator",
		ChartRef:        cfg.Versions.Tier1.GPUOperator.ChartRef(),
		Version:         cfg.Versions.Tier1.GPUOperator.GetVersion(),
		Namespace:       "nvidia-gpu-operator",
		ValuesPath:      "resources/core/deployment/tier1/nvidia-gpu-operator/values.yaml",
		Wait:            true,
		TimeoutSeconds:  1200,
		InfoMessage:     "Installing NVIDIA GPU Operator (may take several minutes)...",
		SuccessMessage:  "NVIDIA GPU Operator deployed (using host drivers)",
		CreateNamespace: true,
	})
}

// deployNodeFeatureDiscovery deploys Node Feature Discovery (NFD) for automatic hardware detection.
// NFD labels nodes with hardware features like GPU vendor, model, and capabilities.
// This is required for Intel Device Plugins to discover Intel GPUs.
// Reference: https://kubernetes-sigs.github.io/node-feature-discovery/
func deployNodeFeatureDiscovery(ctx context.Context, cfg *config.Config) error {
	const nfdNamespace = "node-feature-discovery"

	// Pre-create namespace with labels
	if err := shared.EnsureNamespace(ctx, nfdNamespace, map[string]string{
		"service-type": "nova",
	}); err != nil {
		return fmt.Errorf("failed to create NFD namespace: %w", err)
	}

	helmClient := helm.NewClient(nfdNamespace)

	// Skip if already installed
	if exists, _ := helmClient.ReleaseExists(ctx, "nfd", nfdNamespace); exists {
		ui.Info("Node Feature Discovery already installed - skipping")
		return nil
	}

	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "nfd",
		ChartRef:        cfg.Versions.Tier1.NodeFeatureDiscovery.ChartRef(),
		Version:         cfg.Versions.Tier1.NodeFeatureDiscovery.GetVersion(),
		Namespace:       nfdNamespace,
		ValuesPath:      "resources/core/deployment/tier1/node-feature-discovery/values.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		InfoMessage:     "Installing Node Feature Discovery...",
		SuccessMessage:  "Node Feature Discovery deployed",
		CreateNamespace: true,
	}); err != nil {
		return fmt.Errorf("failed to install Node Feature Discovery: %w", err)
	}

	// Wait for NFD master to be ready before proceeding
	ui.Info("Waiting for NFD master to be ready...")
	if err := k8s.WaitForDeploymentReady(ctx, nfdNamespace, "nfd-node-feature-discovery-master", 120); err != nil {
		return fmt.Errorf("NFD master not ready: %w", err)
	}

	// Brief pause to allow NFD worker DaemonSet to start labeling nodes
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
	}

	ui.Info("NFD is running - nodes will be labeled with hardware features")
	return nil
}

// deployIntelDevicePlugin deploys the Intel GPU Device Plugin for Intel integrated/discrete GPUs.
// This requires Node Feature Discovery (NFD) and Intel Device Plugins Operator to be installed first.
// Reference: https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/INSTALL.md
func deployIntelDevicePlugin(ctx context.Context, cfg *config.Config) error {
	ui.Info("GPU mode: Intel")

	// Step 0: Deploy Node Feature Discovery first (required for Intel GPU detection)
	if err := deployNodeFeatureDiscovery(ctx, cfg); err != nil {
		return fmt.Errorf("failed to deploy NFD: %w", err)
	}

	const intelNamespace = "inteldeviceplugins-system"

	// Pre-create namespace with labels for Cilium network routing
	// This is required for API server to reach the webhook service
	if err := shared.EnsureNamespace(ctx, intelNamespace, map[string]string{
		"service-type": "nova",
	}); err != nil {
		return fmt.Errorf("failed to create Intel namespace: %w", err)
	}

	helmClient := helm.NewClient(intelNamespace)

	// Step 1: Deploy the Intel Device Plugins Operator (contains CRDs)
	// Operator is restricted to GPU node via nodeSelector (same as NVIDIA GPU Operator)
	if exists, _ := helmClient.ReleaseExists(ctx, "intel-device-plugins-operator", intelNamespace); !exists {
		if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
			ReleaseName:     "intel-device-plugins-operator",
			ChartRef:        cfg.Versions.Tier1.IntelDevicePluginsOperator.ChartRef(),
			Version:         cfg.Versions.Tier1.IntelDevicePluginsOperator.GetVersion(),
			Namespace:       intelNamespace,
			ValuesPath:      "resources/core/deployment/tier1/intel-device-plugins-operator/values.yaml",
			Wait:            true,
			TimeoutSeconds:  300,
			InfoMessage:     "Installing Intel Device Plugins Operator (CRDs)...",
			SuccessMessage:  "Intel Device Plugins Operator deployed",
			CreateNamespace: true,
		}); err != nil {
			return fmt.Errorf("failed to install Intel Device Plugins Operator: %w", err)
		}
	} else {
		ui.Info("Intel Device Plugins Operator already installed - skipping")
	}

	// Always wait for the webhook to be ready before deploying the GPU plugin
	// (even if operator was already installed from a previous failed attempt)
	ui.Info("Waiting for Intel Device Plugins webhook...")
	if err := k8s.WaitForDeploymentReady(ctx, intelNamespace, "inteldeviceplugins-controller-manager", 120); err != nil {
		return fmt.Errorf("Intel Device Plugins controller not ready: %w", err)
	}

	// Wait for webhook endpoints to be ready
	ui.Info("Waiting for webhook endpoints...")
	if err := k8s.WaitForEndpoints(ctx, intelNamespace, "inteldeviceplugins-webhook-service", 120); err != nil {
		return fmt.Errorf("Intel Device Plugins webhook endpoints not ready: %w", err)
	}

	// Brief pause for network routing stabilization
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
	}

	// Step 2: Deploy the GPU Device Plugin CR
	if exists, _ := helmClient.ReleaseExists(ctx, "intel-gpu-plugin", intelNamespace); exists {
		ui.Info("Intel GPU Device Plugin already installed - skipping")
		return nil
	}

	// Retry the GPU plugin install - Cilium eBPF datapath may not be fully
	// programmed for the webhook service yet, causing "operation not permitted"
	// errors when API server tries to call the mutating webhook
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			ui.Info("Retrying GPU plugin install (attempt %d/3)...", attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}

		lastErr = shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
			ReleaseName:     "intel-gpu-plugin",
			ChartRef:        cfg.Versions.Tier1.IntelGPUPlugin.ChartRef(),
			Version:         cfg.Versions.Tier1.IntelGPUPlugin.GetVersion(),
			Namespace:       intelNamespace,
			ValuesPath:      "resources/core/deployment/tier1/intel-gpu-plugin/values.yaml",
			Wait:            true,
			TimeoutSeconds:  600,
			InfoMessage:     "Installing Intel GPU Device Plugin...",
			SuccessMessage:  "Intel GPU Device Plugin deployed",
			CreateNamespace: true,
		})
		if lastErr == nil {
			return nil
		}

		// Only retry on webhook connectivity errors
		if !isWebhookConnectivityError(lastErr) {
			return lastErr
		}
		ui.Warn("Webhook not reachable yet: %v", lastErr)
	}
	return lastErr
}

// isWebhookConnectivityError checks if the error is a webhook connectivity issue
// that might resolve with a retry (Cilium eBPF datapath not ready yet)
func isWebhookConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// "operation not permitted" = eBPF/iptables not ready
	// "connection refused" = service endpoints not ready
	// "i/o timeout" = network path not established
	return strings.Contains(errStr, "operation not permitted") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "i/o timeout")
}
