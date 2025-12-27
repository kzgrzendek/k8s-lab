package cmd

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/deployer"
	"github.com/kzgrzendek/nova/internal/minikube"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var tier int

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the NOVA lab environment",
		Long: `Starts the NOVA lab environment up to the specified tier:

  Tier 0 - Minikube Cluster (prerequisite):
    • 3-node Kubernetes cluster with GPU support
    • BPF filesystem for eBPF/Cilium
    • Control-plane taints and GPU configuration

  Tier 1 - Infrastructure:
    • Cilium CNI, Falco, NVIDIA GPU Operator
    • Cert-Manager, Trust-Manager
    • Envoy Gateway, Envoy AI Gateway

  Tier 2 - Platform:
    • Kyverno, Keycloak (IAM)
    • Hubble, Victoria Metrics/Logs

  Tier 3 - Applications:
    • llm-d (LLM serving), Open WebUI, HELIX

Tiers are cumulative: --tier=2 deploys Tier 0, 1, and 2.
Use --tier=0 to deploy only the Minikube cluster.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd, tier)
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 3, "deploy up to this tier (0, 1, 2, or 3)")

	return cmd
}

func runStart(cmd *cobra.Command, targetTier int) error {
	if targetTier < 0 || targetTier > 3 {
		return fmt.Errorf("tier must be 0, 1, 2, or 3 (got %d)", targetTier)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'nova setup' first)", err)
	}

	if !cfg.State.Initialized {
		return fmt.Errorf("nova not initialized, run 'nova setup' first")
	}

	ui.Header("Starting NOVA (Tier 0-%d)", targetTier)

	// Check if cluster is already running
	running, err := minikube.IsRunning(cmd.Context())
	if err != nil {
		ui.Warn("Failed to check cluster status: %v", err)
	}

	// Tier 0: Start Minikube cluster if not running
	if !running {
		if err := deployer.DeployTier0(cmd.Context(), cfg); err != nil {
			return fmt.Errorf("failed to deploy tier 0: %w", err)
		}
	} else {
		ui.Info("Minikube cluster already running")
	}

	// TODO: Deploy higher tiers
	if targetTier >= 1 {
		ui.Step("Deploying Tier 1 (Infrastructure)...")
		ui.Warn("Tier 1 deployment not yet implemented")
	}

	if targetTier >= 2 {
		ui.Step("Deploying Tier 2 (Platform)...")
		ui.Warn("Tier 2 deployment not yet implemented")
	}

	if targetTier >= 3 {
		ui.Step("Deploying Tier 3 (Applications)...")
		ui.Warn("Tier 3 deployment not yet implemented")
	}

	// TODO: Start host services
	ui.Step("Starting host services (NGINX, Bind9)...")
	ui.Warn("Host services not yet implemented")

	// Update state
	cfg.State.LastDeployedTier = targetTier
	if err := cfg.Save(); err != nil {
		ui.Warn("Failed to save state: %v", err)
	}

	ui.Header("NOVA Started")
	ui.Info("Access your cluster with: nova kubectl get nodes")

	return nil
}
