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
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// DeployTier1 deploys tier 1: Infrastructure layer (Cilium, Falco, GPU Operator, Cert-Manager, Envoy Gateway).
func DeployTier1(ctx context.Context, cfg *config.Config) error {
	// Define deployment steps
	steps := []string{
		"Prerequisites Check",
		"Helm Repositories",
		"Cilium CNI",
		"CoreDNS Configuration",
		"Local Path Storage",
		"Falco Security",
		"NVIDIA GPU Operator",
		"Cert Manager",
		"Trust Manager",
		"Envoy AI Gateway",
		"Envoy Gateway",
	}

	// Create progress tracker
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

	// Step 1: Check prerequisites
	if err := runStep("Prerequisites Check", func() error {
		return checkPrerequisites(ctx)
	}); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	// Step 2: Add Helm repositories
	if err := runStep("Helm Repositories", func() error {
		return addHelmRepos(ctx)
	}); err != nil {
		return fmt.Errorf("failed to add Helm repositories: %w", err)
	}

	// Step 3: Cilium CNI
	if err := runStep("Cilium CNI", func() error {
		return deployCilium(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 4: CoreDNS Configuration
	if err := runStep("CoreDNS Configuration", func() error {
		return deployCoreDNS(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 4: Local Path Storage
	if err := runStep("Local Path Storage", func() error {
		return deployLocalPathStorage(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 5: Falco Security
	if err := runStep("Falco Security", func() error {
		return deployFalco(ctx)
	}); err != nil {
		return err
	}

	// Step 6: NVIDIA GPU Operator
	if err := runStep("NVIDIA GPU Operator", func() error {
		return deployGPUOperator(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 7: Cert Manager
	if err := runStep("Cert Manager", func() error {
		return deployCertManager(ctx, cfg)
	}); err != nil {
		return err
	}

	// Step 8: Trust Manager
	if err := runStep("Trust Manager", func() error {
		return deployTrustManager(ctx)
	}); err != nil {
		return err
	}

	// Step 9: Envoy AI Gateway
	if err := runStep("Envoy AI Gateway", func() error {
		return deployEnvoyAIGateway(ctx)
	}); err != nil {
		return err
	}

	// Step 10: Envoy Gateway
	if err := runStep("Envoy Gateway", func() error {
		return deployEnvoyGateway(ctx, cfg)
	}); err != nil {
		return err
	}

	// Mark all steps complete
	progress.Complete()

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

// deployWithProgress is a helper for common Helm deployments within a tier.
// The dynamicValuesFn parameter is optional and can be used to generate runtime values.
func deployWithProgress(ctx context.Context, opts shared.HelmDeploymentOptions, dynamicValuesFn func(context.Context) (map[string]interface{}, error)) error {
	if dynamicValuesFn != nil {
		dynamicValues, err := dynamicValuesFn(ctx)
		if err != nil {
			return err
		}
		if opts.Values == nil {
			opts.Values = dynamicValues
		} else {
			// Merge dynamic values with existing values
			for k, v := range dynamicValues {
				opts.Values[k] = v
			}
		}
	}

	if err := shared.DeployHelmChart(ctx, opts); err != nil {
		return err
	}

	return nil
}

func addHelmRepos(ctx context.Context) error {
	repos := map[string]string{
		"cilium":        constants.HelmRepoCilium,
		"nvidia":        constants.HelmRepoNvidia,
		"falcosecurity": constants.HelmRepoFalco,
		"jetstack":      constants.HelmRepoJetstack,
		"dandydev":      constants.HelmRepoDandyDev,
	}

	// Create Helm client
	helmClient := helm.NewClient("")

	ui.Info("Adding %d Helm repositories...", len(repos))
	for name, url := range repos {
		ui.Debug("- Adding %s repository", name)
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

func deployCilium(ctx context.Context, cfg *config.Config) error {
	// Label namespace
	if err := k8s.LabelNamespace(ctx, "kube-system", "service-type", "nova"); err != nil {
		return fmt.Errorf("failed to label kube-system namespace for routing: %w", err)
	}

	// Function to generate dynamic values for Cilium
	dynamicValuesFn := func(ctx context.Context) (map[string]interface{}, error) {
		// Get minikube IP for Cilium configuration
		ui.Info("Getting Minikube control plane IP...")
		minikubeIP, err := minikube.GetIP(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get minikube IP: %w", err)
		}

		// Get API server port from kubectl
		ui.Info("Getting API server port...")
		apiPort, err := minikube.GetAPIServerPort(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get API server port: %w", err)
		}

		// Create dynamic overrides for runtime configuration
		// Note: We MUST set k8sServiceHost and k8sServicePort for Cilium to work properly in Minikube
		ui.Info("Installing Cilium CNI with API server %s:%s (may take a few minutes)...", minikubeIP, apiPort)
		return map[string]interface{}{
			"k8sServiceHost": minikubeIP,
			"k8sServicePort": apiPort,
		}, nil
	}

	return deployWithProgress(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "cilium",
		ChartRef:        "cilium/cilium",
		Namespace:       "kube-system",
		ValuesPath:      "resources/core/deployment/tier1/cilium/values.yaml",
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
	}, dynamicValuesFn)
}

func deployLocalPathStorage(ctx context.Context, cfg *config.Config) error {
	if err := k8s.CreateNamespace(ctx, constants.NamespaceLocalPathStorage); err != nil {
		return fmt.Errorf("failed to create local-path-storage namespace: %w", err)
	}

	// Apply local-path-provisioner
	ui.Info("Installing Local Path Provisioner...")
	if err := k8s.ApplyURL(ctx, constants.ManifestLocalPathProvisioner); err != nil {
		return fmt.Errorf("failed to deploy local-path-provisioner: %w", err)
	}

	// Apply additional standard storage class
	ui.Info("Applying standard storage class...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/local-path-provisioner/storageclasses/standard.yaml"); err != nil {
		return fmt.Errorf("failed to apply standard storage class: %w", err)
	}

	// Patch configmap for custom storage directory
	ui.Info("Patching storage directory configuration...")
	if err := k8s.PatchConfigMap(ctx, constants.NamespaceLocalPathStorage, "local-path-config", "resources/core/deployment/tier1/local-path-provisioner/patches/storage-dir.yaml"); err != nil {
		return fmt.Errorf("failed to patch local-path-config: %w", err)
	}

	return nil
}

// deployFalco deploys Falco security auditor.
func deployFalco(ctx context.Context) error {
	return deployWithProgress(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "falco",
		ChartRef:        "falcosecurity/falco",
		Namespace:       "falco",
		ValuesPath:      "resources/core/deployment/tier1/falco/values.yaml",
		Wait:            true,
		TimeoutSeconds:  600,
		InfoMessage:     "Installing Falco Security (may take a few minutes)...",
		CreateNamespace: true,
	}, nil)
}

func deployGPUOperator(ctx context.Context, cfg *config.Config) error {
	// Skip if no GPU configured or disabled
	if cfg.Minikube.GPUs == "" || cfg.Minikube.GPUs == "none" || cfg.Minikube.GPUs == "disabled" {
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

	return deployWithProgress(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "gpu-operator",
		ChartRef:        "nvidia/gpu-operator",
		Namespace:       "nvidia-gpu-operator",
		ValuesPath:      "resources/core/deployment/tier1/nvidia-gpu-operator/values.yaml",
		Wait:            true,
		TimeoutSeconds:  1200,
		InfoMessage:     "Installing NVIDIA GPU Operator (may take several minutes)...",
		SuccessMessage:  "NVIDIA GPU Operator deployed (using host drivers)",
		CreateNamespace: true,
	}, nil)
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
		ChartRef:       "jetstack/cert-manager",
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
	domainData := map[string]interface{}{
		"Domain": cfg.DNS.Domain,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/cert-manager/clusterissuers/nova-issuer.yaml", domainData); err != nil {
		return fmt.Errorf("failed to create cluster issuers: %w", err)
	}

	return nil
}

// deployTrustManager installs trust-manager and handles CA distribution.
func deployTrustManager(ctx context.Context) error {
	// Deploy Trust Manager
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "trust-manager",
		ChartRef:       "jetstack/trust-manager",
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
		ChartRef:       "cilium/cilium",
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

func deployEnvoyAIGateway(ctx context.Context) error {
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
	if err := k8s.ApplyURL(ctx, constants.ManifestGatewayAPIInferenceExtension); err != nil {
		return fmt.Errorf("failed to install inference extension CRDs: %w", err)
	}

	// Install AI Gateway CRDs using OCI
	if err := shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:    "aieg-crd",
		ChartRef:       "oci://docker.io/envoyproxy/ai-gateway-crds-helm:v0.4.0",
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
		ChartRef:       "oci://docker.io/envoyproxy/ai-gateway-helm:v0.4.0",
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
		ChartRef:       "dandydev/redis-ha",
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
		ChartRef:       "oci://docker.io/envoyproxy/gateway-helm:v1.6.1",
		Namespace:      "envoy-gateway-system",
		ValuesPath:     "resources/core/deployment/tier1/envoy-gateway/values.yaml",
		Wait:           true,
		TimeoutSeconds: 600,
		InfoMessage:    "Installing Envoy Gateway...",
	}); err != nil {
		return err
	}

	// Apply Envoy Gateway configuration manifests
	domainData := map[string]interface{}{
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

func deployCoreDNS(ctx context.Context, cfg *config.Config) error {
	data := map[string]interface{}{
		"AuthDomain": cfg.DNS.AuthDomain,
	}

	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/coredns/configmaps/config-dns-rewrite.yaml", data); err != nil {
		return fmt.Errorf("failed to apply coredns rewrite config: %w", err)
	}

	// Restart CoreDNS to pick up changes immediately
	if err := k8s.RestartDeployment(ctx, "kube-system", "coredns"); err != nil {
		return fmt.Errorf("failed to restart coredns: %w", err)
	}

	return nil
}
