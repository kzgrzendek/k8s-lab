package commands

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier0"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier1"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier2"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier3"
	"github.com/kzgrzendek/nova/internal/core/deployment/warmup"
	"github.com/kzgrzendek/nova/internal/host/foundation"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var tier int
	var hfToken string
	var model string
	var cpuMode bool
	var nodes int
	var k8sVersion string

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
			return runStart(cmd, tier, hfToken, model, cpuMode, nodes, k8sVersion)
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 3, "deploy up to this tier (0, 1, 2, or 3)")
	cmd.Flags().StringVar(&hfToken, "hf-token", "", "Hugging Face token for faster model downloads (optional)")
	cmd.Flags().StringVar(&model, "model", "", "Hugging Face model to serve (e.g., google/gemma-3-4b-it, default: use config)")
	cmd.Flags().BoolVar(&cpuMode, "cpu-mode", false, "force CPU mode (disable GPU even if available)")
	cmd.Flags().IntVar(&nodes, "nodes", -1, "number of total nodes (1 master + N-1 workers, -1 = use config)")
	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "", "Kubernetes version for minikube (e.g., v1.33.5, default: use config)")

	return cmd
}

func runStart(cmd *cobra.Command, targetTier int, hfToken string, model string, cpuMode bool, nodes int, k8sVersion string) error {
	if targetTier < 0 || targetTier > 3 {
		return fmt.Errorf("tier must be 0, 1, 2, or 3 (got %d)", targetTier)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'nova setup' first)", err)
	}

	// Override config with CLI flags (CLI takes precedence)
	if hfToken != "" {
		cfg.LLM.HfToken = hfToken
	}
	if model != "" {
		cfg.LLM.Model = model
	}
	if cpuMode {
		cfg.Minikube.CPUModeForced = true
		ui.Info("CPU mode forced via --cpu-mode flag")
	}
	if nodes > 0 {
		cfg.Minikube.Nodes = nodes
		ui.Info("Using %d total nodes (%d master + %d workers)", nodes, 1, nodes-1)
	}
	if k8sVersion != "" {
		cfg.Versions.Kubernetes = k8sVersion
		cfg.Minikube.KubernetesVersion = k8sVersion
		ui.Info("Using Kubernetes version %s", k8sVersion)
	}

	if !cfg.State.Initialized {
		return fmt.Errorf("nova not initialized, run 'nova setup' first")
	}

	// Verify mkcert CA is still installed (setup might have been run a while ago)
	installed, err := pki.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check mkcert status: %w", err)
	}
	if !installed {
		return fmt.Errorf("mkcert CA not found - run 'nova setup' again to reinstall")
	}

	ui.Header("Starting NOVA (Tier 0-%d)", targetTier)

	// Build deployment steps based on target tier
	steps := []string{"Foundation Stack", "Tier 0: Minikube Cluster"}
	if targetTier >= 1 {
		steps = append(steps, "Tier 1: Infrastructure")
	}
	if targetTier >= 2 {
		steps = append(steps, "Tier 2: Platform")
	}
	if targetTier >= 3 {
		steps = append(steps, "Tier 3: Applications")
	}

	// Create progress tracker
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Step 1: Foundation Stack (network, NGINX, DNS, NFS, Registry)
	progress.StartStep(currentStep)
	foundationStack := foundation.New(cfg)
	if err := foundationStack.Start(cmd.Context(), targetTier); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to start foundation stack: %w", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 2: Warmup operations (tier 3 only, runs in background)
	var warmupOrch *warmup.Orchestrator
	if targetTier >= 3 {
		warmupOrch = warmup.New(cmd.Context(), cfg)
		if err := warmupOrch.Start(); err != nil {
			return fmt.Errorf("failed to start warmup operations: %w", err)
		}
	}

	// Use warmup context for deployments if tier 3, otherwise use cmd.Context()
	// This ensures fail-fast behavior if warmup fails
	deployCtx := cmd.Context()
	if warmupOrch != nil {
		deployCtx = warmupOrch.Context()
	}

	// Check if cluster is already running
	running, err := minikube.IsRunning(deployCtx)
	if err != nil {
		ui.Warn("Failed to check cluster status: %v", err)
	}

	// Step 3: Tier 0: Configure Minikube cluster (already started by Foundation Stack)
	progress.StartStep(currentStep)
	if !running {
		// Check if warmup failed and cancelled context
		if deployCtx.Err() != nil {
			progress.FailStep(currentStep, deployCtx.Err())
			return fmt.Errorf("deployment cancelled due to warmup failure: %w", deployCtx.Err())
		}
		if err := tier0.DeployTier0(deployCtx, cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 0: %w", err)
		}
		progress.CompleteStep(currentStep)
	} else {
		ui.Info("Minikube cluster already running - applying configuration")
		if err := tier0.DeployTier0(deployCtx, cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 0: %w", err)
		}
		progress.CompleteStep(currentStep)
	}
	currentStep++

	// Deploy higher tiers
	if targetTier >= 1 {
		// Check if warmup failed and cancelled context
		if deployCtx.Err() != nil {
			return fmt.Errorf("deployment cancelled due to warmup failure: %w", deployCtx.Err())
		}
		progress.StartStep(currentStep)
		if err := tier1.DeployTier1(deployCtx, cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 1: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++
	}

	var tier2Result *tier2.DeployResult
	if targetTier >= 2 {
		// Check if warmup failed and cancelled context
		if deployCtx.Err() != nil {
			return fmt.Errorf("deployment cancelled due to warmup failure: %w", deployCtx.Err())
		}
		progress.StartStep(currentStep)
		var err error
		tier2Result, err = tier2.DeployTier2(deployCtx, cfg)
		if err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 2: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++
	}

	if targetTier >= 3 {
		// Wait for warmup operations to complete before deploying tier 3
		if warmupOrch != nil {
			if err := warmupOrch.Wait(); err != nil {
				return fmt.Errorf("warmup operations failed: %w", err)
			}
		}

		progress.StartStep(currentStep)
		if err := tier3.DeployTier3(deployCtx, cfg); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to deploy tier 3: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++
	}

	// Mark all steps complete
	progress.Complete()

	// Update state
	cfg.State.LastDeployedTier = targetTier
	if err := cfg.Save(); err != nil {
		ui.Warn("Failed to save state: %v", err)
	}

	// Check if developer context exists and switch to it (if tier >= 1)
	if targetTier >= 1 && k8s.ContextExists(cmd.Context(), "cluster-admin") {
		ui.Info("")
		ui.Info("Switching to cluster-admin context...")
		if err := k8s.SwitchContext(cmd.Context(), "cluster-admin"); err != nil {
			ui.Warn("Failed to switch to cluster-admin context: %v", err)
		} else {
			ui.Success("Switched to kubectl context 'cluster-admin'")
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

	// Tier 3 URLs
	if targetTier >= 3 {
		ui.Info("  llm-d API: https://llmd.internal.%s/v1", cfg.DNS.Domain)
		ui.Info("  Open WebUI: https://chat.%s", cfg.DNS.Domain)
		ui.Info("  HELIX: https://helix.%s", cfg.DNS.Domain)
	}

	// Display Keycloak credentials if Tier 2 was deployed
	if tier2Result != nil && len(tier2Result.KeycloakUsers) > 0 {
		DisplayKeycloakCredentials(tier2Result, cfg)
	}

	// Show developer context info
	ui.Info("")
	ui.Header("Developer kubectl context")
	ui.Info("")
	ui.Info("A restricted kubectl context 'developer' has been created.")
	ui.Info("It provides full access to the 'nova' namespace only.")
	ui.Info("")
	ui.Info("To switch back to admin context:")
	ui.Info("  kubectl config use-context cluster-admin")
}
