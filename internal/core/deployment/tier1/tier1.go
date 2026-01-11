package tier1

import (
	"context"
	"fmt"
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

// DeployTier1 deploys tier 1: Infrastructure layer (Security, GPU, Certificates, Gateways).
func DeployTier1(ctx context.Context, cfg *config.Config) error {
	// Define deployment steps
	steps := []string{
		"Prerequisites Check",
		"Helm Repositories",
		"Falco Security",
		"NVIDIA GPU Operator",
		"Cert Manager",
		"Trust Manager",
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
			"nvidia":        constants.HelmRepoNvidia,
			"falcosecurity": constants.HelmRepoFalco,
			"jetstack":      constants.HelmRepoJetstack,
			"dandydev":      constants.HelmRepoDandyDev,
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

	// Step 6: NVIDIA GPU Operator
	if err := runner.RunStep("NVIDIA GPU Operator", func() error {
		return deployGPUOperator(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 7: Cert Manager
	if err := runner.RunStep("Cert Manager", func() error {
		return deployCertManager(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 8: Trust Manager
	if err := runner.RunStep("Trust Manager", func() error {
		return deployTrustManager(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 9: Envoy AI Gateway
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

func deployGPUOperator(ctx context.Context, cfg *config.Config) error {
	// Skip if not in GPU mode (no GPU configured, disabled, or CPU mode forced)
	if !cfg.IsGPUMode() {
		ui.Info("GPU mode disabled - skipping NVIDIA GPU Operator")
		return nil
	}

	ui.Info("GPU mode: %s", cfg.Minikube.GPUs)

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
