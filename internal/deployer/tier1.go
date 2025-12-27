package deployer

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/ui"
)

// DeployTier1 deploys tier 1: Infrastructure layer (Cilium, Falco, GPU Operator, Cert-Manager, Envoy Gateway).
func DeployTier1(ctx context.Context, cfg *config.Config) error {
	ui.Header("Tier 1: Infrastructure")

	// Check prerequisites
	if err := checkPrerequisites(ctx); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	// Add Helm repositories
	if err := addHelmRepos(ctx); err != nil {
		return fmt.Errorf("failed to add helm repos: %w", err)
	}

	// Deploy components in order
	steps := []struct {
		name string
		fn   func(context.Context, *config.Config) error
	}{
		{"Cilium CNI", deployCilium},
		{"Local Path Storage", deployLocalPathStorage},
		{"Falco Security", deployFalco},
		{"NVIDIA GPU Operator", deployGPUOperator},
		{"Cert Manager", deployCertManager},
		{"Trust Manager", deployTrustManager},
		{"Envoy AI Gateway", deployEnvoyAIGateway},
		{"Envoy Gateway", deployEnvoyGateway},
	}

	for i, step := range steps {
		ui.Step("[%d/%d] Deploying %s...", i+1, len(steps), step.name)
		if err := step.fn(ctx, cfg); err != nil {
			ui.Warn("Failed to deploy %s: %v (continuing...)", step.name, err)
		} else {
			ui.Success("%s deployed", step.name)
		}
	}

	ui.Header("Tier 1 Deployment Complete")
	return nil
}

func checkPrerequisites(ctx context.Context) error {
	// Check kubectl
	if err := exec.CommandContext(ctx, "kubectl", "version", "--client").Run(); err != nil {
		return fmt.Errorf("kubectl not available: %w", err)
	}

	// Check helm
	if err := exec.CommandContext(ctx, "helm", "version").Run(); err != nil {
		return fmt.Errorf("helm not available: %w", err)
	}

	return nil
}

func addHelmRepos(ctx context.Context) error {
	repos := map[string]string{
		"cilium":         "https://helm.cilium.io/",
		"nvidia":         "https://helm.ngc.nvidia.com/nvidia",
		"falcosecurity":  "https://falcosecurity.github.io/charts",
		"jetstack":       "https://charts.jetstack.io",
		"dandydev":       "https://dandydeveloper.github.io/charts",
	}

	ui.Info("  • Adding Helm repositories...")
	for name, url := range repos {
		cmd := exec.CommandContext(ctx, "helm", "repo", "add", name, url, "--force-update")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add %s repo: %w", name, err)
		}
	}

	// Update repos
	cmd := exec.CommandContext(ctx, "helm", "repo", "update")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update helm repos: %w", err)
	}

	return nil
}

func deployCilium(ctx context.Context, cfg *config.Config) error {
	// Create kube-system namespace if not exists (should exist)
	createNamespace(ctx, "kube-system")

	// Install Cilium with basic configuration
	args := []string{
		"upgrade", "--install", "cilium", "cilium/cilium",
		"--namespace", "kube-system",
		"--set", "ipam.mode=kubernetes",
		"--set", "kubeProxyReplacement=true",
		"--set", "k8sServiceHost=localhost",
		"--set", "k8sServicePort=8443",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install cilium: %w", err)
	}

	return nil
}

func deployLocalPathStorage(ctx context.Context, cfg *config.Config) error {
	createNamespace(ctx, "local-path-storage")

	// Apply local-path-provisioner
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f",
		"https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.24/deploy/local-path-storage.yaml")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to deploy local-path-provisioner: %w", err)
	}

	return nil
}

func deployFalco(ctx context.Context, cfg *config.Config) error {
	createNamespace(ctx, "falco")

	args := []string{
		"upgrade", "--install", "falco", "falcosecurity/falco",
		"--namespace", "falco",
		"--set", "tty=true",
		"--set", "driver.kind=modern_ebpf",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install falco: %w", err)
	}

	return nil
}

func deployGPUOperator(ctx context.Context, cfg *config.Config) error {
	// Skip if no GPU configured or disabled
	if cfg.Minikube.GPUs == "" || cfg.Minikube.GPUs == "none" || cfg.Minikube.GPUs == "disabled" {
		ui.Info("  • GPU mode disabled - skipping NVIDIA GPU operator")
		return nil
	}

	ui.Info("  • GPU mode: %s", cfg.Minikube.GPUs)

	createNamespace(ctx, "nvidia-gpu-operator")

	args := []string{
		"upgrade", "--install", "gpu-operator", "nvidia/gpu-operator",
		"--namespace", "nvidia-gpu-operator",
		"--set", "driver.enabled=false", // Use host drivers
		"--set", "toolkit.enabled=true",
		"--set", "operator.defaultRuntime=docker",
		"--wait",
		"--timeout", "10m",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install gpu-operator: %w", err)
	}

	ui.Info("  • NVIDIA GPU operator deployed (using host drivers)")
	return nil
}

func deployCertManager(ctx context.Context, cfg *config.Config) error {
	createNamespace(ctx, "cert-manager")

	args := []string{
		"upgrade", "--install", "cert-manager", "jetstack/cert-manager",
		"--namespace", "cert-manager",
		"--set", "crds.enabled=true",
		"--set", "crds.keep=true",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install cert-manager: %w", err)
	}

	return nil
}

func deployTrustManager(ctx context.Context, cfg *config.Config) error {
	args := []string{
		"upgrade", "--install", "trust-manager", "jetstack/trust-manager",
		"--namespace", "cert-manager",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install trust-manager: %w", err)
	}

	return nil
}

func deployEnvoyAIGateway(ctx context.Context, cfg *config.Config) error {
	createNamespace(ctx, "envoy-ai-gateway-system")

	// Install AI Gateway CRDs
	args := []string{
		"upgrade", "--install", "ai-gateway-crds",
		"oci://docker.io/envoyproxy/ai-gateway-crds-helm",
		"--version", "v0.4.0",
		"--namespace", "envoy-ai-gateway-system",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install ai-gateway CRDs: %w", err)
	}

	// Install AI Gateway
	args = []string{
		"upgrade", "--install", "ai-gateway",
		"oci://docker.io/envoyproxy/ai-gateway-helm",
		"--version", "v0.4.0",
		"--namespace", "envoy-ai-gateway-system",
		"--wait",
	}

	cmd = exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install ai-gateway: %w", err)
	}

	return nil
}

func deployEnvoyGateway(ctx context.Context, cfg *config.Config) error {
	createNamespace(ctx, "envoy-gateway-system")

	// Install Redis for rate limiting
	ui.Info("  • Installing Redis backend...")
	args := []string{
		"upgrade", "--install", "redis-ha", "dandydev/redis-ha",
		"--namespace", "envoy-gateway-system",
		"--set", "replicas=1",
		"--wait",
	}

	cmd := exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install redis: %w", err)
	}

	// Install Envoy Gateway
	ui.Info("  • Installing Envoy Gateway...")
	args = []string{
		"upgrade", "--install", "envoy-gateway",
		"oci://docker.io/envoyproxy/gateway-helm",
		"--version", "v1.2.4",
		"--namespace", "envoy-gateway-system",
		"--wait",
	}

	cmd = exec.CommandContext(ctx, "helm", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install envoy-gateway: %w", err)
	}

	return nil
}

func createNamespace(ctx context.Context, namespace string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace,
		"--dry-run=client", "-o", "yaml")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	cmd = exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = bytes.NewReader(output)
	return cmd.Run()
}
