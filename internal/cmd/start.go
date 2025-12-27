package cmd

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/bind9"
	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/deployer"
	"github.com/kzgrzendek/nova/internal/minikube"
	"github.com/kzgrzendek/nova/internal/nginx"
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

	// Build deployment steps based on target tier
	steps := []string{"Tier 0: Minikube Cluster"}
	if targetTier >= 1 {
		steps = append(steps, "Tier 1: Infrastructure")
	}
	if targetTier >= 2 {
		steps = append(steps, "Tier 2: Platform")
	}
	if targetTier >= 3 {
		steps = append(steps, "Tier 3: Applications")
	}
	steps = append(steps, "Host Services (Bind9 & NGINX)")

	// Create progress tracker
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Check if cluster is already running
	running, err := minikube.IsRunning(cmd.Context())
	if err != nil {
		ui.Warn("Failed to check cluster status: %v", err)
	}

	// Tier 0: Start Minikube cluster if not running
	progress.StartStep(currentStep)
	if !running {
		if err := deployer.DeployTier0(cmd.Context(), cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 0: %w", err)
		}
		progress.CompleteStep(currentStep)
	} else {
		ui.Info("Minikube cluster already running - skipping")
		progress.CompleteStep(currentStep)
	}
	currentStep++

	// Deploy higher tiers
	if targetTier >= 1 {
		progress.StartStep(currentStep)
		ui.Warn("Tier 1 deployment not yet implemented")
		currentStep++
	}

	if targetTier >= 2 {
		progress.StartStep(currentStep)
		ui.Warn("Tier 2 deployment not yet implemented")
		currentStep++
	}

	if targetTier >= 3 {
		progress.StartStep(currentStep)
		ui.Warn("Tier 3 deployment not yet implemented")
		currentStep++
	}

	// Start host services (Bind9 DNS and NGINX gateway)
	progress.StartStep(currentStep)

	// Start Bind9 DNS
	ui.Info("  • Starting Bind9 DNS server...")
	if err := bind9.Start(cmd.Context(), cfg); err != nil {
		ui.Warn("Failed to start Bind9: %v", err)
	} else {
		ui.Success("Bind9 DNS server started on port %d", cfg.DNS.Bind9Port)
	}

	// Start NGINX gateway
	ui.Info("  • Starting NGINX gateway...")
	if err := nginx.Start(cmd.Context(), cfg); err != nil {
		ui.Warn("Failed to start NGINX: %v", err)
	} else {
		ui.Success("NGINX gateway started (HTTP:80, HTTPS:443)")
	}

	progress.CompleteStep(currentStep)

	// Mark all steps complete
	progress.Complete()

	// Update state
	cfg.State.LastDeployedTier = targetTier
	if err := cfg.Save(); err != nil {
		ui.Warn("Failed to save state: %v", err)
	}

	ui.Header("NOVA Started")
	ui.Info("Access your cluster with: nova kubectl get nodes")

	return nil
}
