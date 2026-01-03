package commands

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier0"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier1"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier2"
	"github.com/kzgrzendek/nova/internal/host/dns/bind9"
	"github.com/kzgrzendek/nova/internal/host/gateway/nginx"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
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
		if err := tier0.DeployTier0(cmd.Context(), cfg); err != nil {
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
		if err := tier1.DeployTier1(cmd.Context(), cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 1: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++
	}

	var tier2Result *tier2.DeployResult
	if targetTier >= 2 {
		progress.StartStep(currentStep)
		var err error
		tier2Result, err = tier2.DeployTier2(cmd.Context(), cfg)
		if err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 2: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++
	}

	if targetTier >= 3 {
		progress.StartStep(currentStep)
		ui.Warn("Tier 3 deployment not yet implemented")
		currentStep++
	}

	// Start host services (Bind9 DNS and NGINX gateway)
	progress.StartStep(currentStep)

	var hostServicesErr error

	// Start Bind9 DNS
	ui.Info("Starting Bind9 DNS server...")
	if err := bind9.Start(cmd.Context(), cfg); err != nil {
		ui.Error("Failed to start Bind9: %v", err)
		hostServicesErr = fmt.Errorf("failed to start Bind9: %w", err)
	} else {
		ui.Success("Bind9 DNS server started on port %d", cfg.DNS.Bind9Port)
	}

	// Start NGINX gateway (only if Bind9 succeeded)
	if hostServicesErr == nil {
		ui.Info("Starting NGINX gateway...")
		if err := nginx.Start(cmd.Context(), cfg); err != nil {
			ui.Error("Failed to start NGINX: %v", err)
			hostServicesErr = fmt.Errorf("failed to start NGINX: %w", err)
		} else {
			ui.Success("NGINX gateway started (HTTP:80, HTTPS:443)")
		}
	}

	// Handle host services errors
	if hostServicesErr != nil {
		progress.FailStep(currentStep, hostServicesErr)
		return fmt.Errorf("host services failed: %w", hostServicesErr)
	}

	progress.CompleteStep(currentStep)

	// Mark all steps complete
	progress.Complete()

	// Update state
	cfg.State.LastDeployedTier = targetTier
	if err := cfg.Save(); err != nil {
		ui.Warn("Failed to save state: %v", err)
	}

	// Check if developer context exists and switch to it if configured
	if k8s.ContextExists(cmd.Context(), "nova-developer") {
		ui.Info("")
		ui.Info("Switching to developer context...")
		if err := k8s.SwitchContext(cmd.Context(), "nova-developer"); err != nil {
			ui.Warn("Failed to switch to nova-developer context: %v", err)
		} else {
			ui.Success("Switched to kubectl context 'nova-developer'")
		}
	}

	// Display deployment summary
	displayDeploymentSummary(cfg, targetTier, tier2Result)

	return nil
}

// displayDeploymentSummary shows the final deployment summary with URLs and credentials.
func displayDeploymentSummary(cfg *config.Config, targetTier int, tier2Result *tier2.DeployResult) {
	ui.Header("Cluster deployed")
	ui.Info("")
	ui.Info("You can now access the following applications:")
	ui.Info("")

	// Tier 0 - always available
	ui.Info("  Kubernetes Dashboard: https://dashboard.%s", cfg.DNS.Domain)

	// Tier 1 URLs
	if targetTier >= 1 {
		ui.Info("  Envoy Gateway: https://gateway.%s", cfg.DNS.Domain)
	}

	// Tier 2 URLs
	if targetTier >= 2 {
		ui.Info("  Keycloak: https://%s", cfg.DNS.AuthDomain)
		ui.Info("  Hubble UI: https://hubble.%s", cfg.DNS.Domain)
		ui.Info("  Grafana: https://grafana.%s", cfg.DNS.Domain)
	}

	// Tier 3 URLs (when implemented)
	if targetTier >= 3 {
		ui.Info("  Open WebUI: https://webui.%s", cfg.DNS.Domain)
	}

	// Display Keycloak credentials if Tier 2 was deployed
	if tier2Result != nil && len(tier2Result.KeycloakUsers) > 0 {
		ui.Info("")
		ui.Header("Log in via Keycloak")
		ui.Info("")

		// Find credentials by username
		var clusterAdminPassword, adminPassword, userPassword, developerPassword string
		for _, user := range tier2Result.KeycloakUsers {
			switch user.Username {
			case "cluster-admin":
				clusterAdminPassword = user.Password
			case "admin":
				adminPassword = user.Password
			case "user":
				userPassword = user.Password
			case "developer":
				developerPassword = user.Password
			}
		}

		if clusterAdminPassword != "" {
			ui.Info("Cluster administrator (master realm):")
			ui.Info("  Username: cluster-admin")
			ui.Info("  Password: %s", clusterAdminPassword)
			ui.Info("")
		}

		if adminPassword != "" {
			ui.Info("As administrator:")
			ui.Info("  Username: admin")
			ui.Info("  Password: %s", adminPassword)
			ui.Info("")
		}

		if userPassword != "" {
			ui.Info("As user:")
			ui.Info("  Username: user")
			ui.Info("  Password: %s", userPassword)
			ui.Info("")
		}

		if developerPassword != "" {
			ui.Info("As developer:")
			ui.Info("  Username: developer")
			ui.Info("  Password: %s", developerPassword)
		}
	}

	// Show developer context info
	ui.Info("")
	ui.Header("Developer kubectl context")
	ui.Info("")
	ui.Info("A restricted kubectl context 'nova-developer' has been created.")
	ui.Info("It provides full access to the 'developer' namespace only.")
	ui.Info("")
	ui.Info("To switch back to admin context:")
	ui.Info("  kubectl config use-context nova-admin")
}
