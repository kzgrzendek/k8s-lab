package cmd

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var tier int

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the NOVA lab environment",
		Long: `Starts the NOVA lab environment up to the specified tier:

  Tier 1 - Infrastructure:
    • Cilium CNI, Falco, NVIDIA GPU Operator
    • Cert-Manager, Trust-Manager
    • Envoy Gateway, Envoy AI Gateway

  Tier 2 - Platform:
    • Kyverno, Keycloak (IAM)
    • Hubble, Victoria Metrics/Logs

  Tier 3 - Applications:
    • llm-d (LLM serving), Open WebUI, HELIX

Tiers are cumulative: --tier=2 deploys both Tier 1 and Tier 2.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd, tier)
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 3, "deploy up to this tier (1, 2, or 3)")

	return cmd
}

func runStart(cmd *cobra.Command, targetTier int) error {
	if targetTier < 1 || targetTier > 3 {
		return fmt.Errorf("tier must be 1, 2, or 3 (got %d)", targetTier)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'nova setup' first)", err)
	}

	if !cfg.State.Initialized {
		return fmt.Errorf("nova not initialized, run 'nova setup' first")
	}

	ui.Header("Starting NOVA (Tier %d)", targetTier)

	// TODO: Implement deployment logic
	// 1. Start Minikube cluster if not running
	// 2. Deploy tiers sequentially
	// 3. Start NGINX and Bind9 containers

	ui.Step("Starting Minikube cluster...")
	ui.Warn("Minikube start not yet implemented")

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
