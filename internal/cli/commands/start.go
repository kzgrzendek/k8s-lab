package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var tier int
	var hfToken string
	var model string
	var gpuMode string
	var profile string
	var k8sVersion string
	var background bool
	var appProfiles string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the NOVA lab environment",
		Long: `Starts the NOVA lab environment up to the specified tier:

  Tier 0 - Minikube Cluster (prerequisite):
    • Kubernetes cluster with GPU support
    • BPF filesystem for eBPF/Cilium
    • Control-plane taints and GPU configuration

  Tier 1 - Infrastructure:
    • Cilium CNI, Falco, GPU Operator (NVIDIA)
    • Cert-Manager, Trust-Manager
    • Envoy Gateway, Envoy AI Gateway

  Tier 2 - Platform:
    • Kyverno, Keycloak (IAM)
    • Hubble, Victoria Metrics/Logs

  Tier 3 - Applications (based on --app-profiles):
    • llm-d (always deployed - inference engine)
    • openwebui: Open WebUI (chat interface)
    • lasuite: OpenGateLLM + Conversations
    • lab: HELIX JupyterHub

Tiers are cumulative: --tier=2 deploys Tier 0, 1, and 2.
Use --tier=0 to deploy only the Minikube cluster.

Resource profiles:
  • minimal: 1 node, 6 CPUs, 12GB RAM (CPU mode) or 6GB RAM (GPU mode)
  • cluster: 3 nodes, 4 CPUs/node, 4GB RAM/node (GPU inference only)

App profiles (Tier 3, default: openwebui,lab):
  • openwebui: Chat interface with Open WebUI
  • lasuite: French government AI stack (OpenGateLLM + Conversations)
  • lab: JupyterHub for ML development (HELIX)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if background {
				return runStartBackground(cmd, tier, hfToken, model, gpuMode, profile, k8sVersion, appProfiles)
			}
			return runStart(cmd, tier, hfToken, model, gpuMode, profile, k8sVersion, appProfiles)
		},
	}

	cmd.Flags().IntVar(&tier, "tier", 3, "deploy up to this tier (0, 1, 2, or 3)")
	cmd.Flags().StringVar(&hfToken, "hf-token", "", "Hugging Face token for faster model downloads (optional)")
	cmd.Flags().StringVar(&model, "model", "", "Hugging Face model to serve (e.g., google/gemma-3-4b-it, default: use config)")
	cmd.Flags().StringVar(&gpuMode, "gpu", "", "enable NVIDIA GPU acceleration (use: --gpu=nvidia)")
	cmd.Flags().StringVar(&profile, "profile", "", "resource profile: minimal (1 node) or cluster (3 nodes), default: use config")
	cmd.Flags().StringVar(&k8sVersion, "k8s-version", "", "Kubernetes version for minikube (e.g., v1.33.5, default: use config)")
	cmd.Flags().BoolVar(&background, "background", false, "run start in the background (detached from terminal)")
	cmd.Flags().StringVar(&appProfiles, "app-profiles", "", "app profiles to deploy (comma-separated): openwebui,lasuite,lab (default: openwebui,lab)")

	return cmd
}

// runStartBackground spawns nova start as a detached background process.
func runStartBackground(cmd *cobra.Command, targetTier int, hfToken string, model string, gpuMode string, profile string, k8sVersion string, appProfiles string) error {
	// Get the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build arguments for the background process (without --background to avoid recursion)
	args := []string{"start", fmt.Sprintf("--tier=%d", targetTier)}
	if hfToken != "" {
		args = append(args, fmt.Sprintf("--hf-token=%s", hfToken))
	}
	if model != "" {
		args = append(args, fmt.Sprintf("--model=%s", model))
	}
	if gpuMode != "" {
		args = append(args, fmt.Sprintf("--gpu=%s", gpuMode))
	}
	if profile != "" {
		args = append(args, fmt.Sprintf("--profile=%s", profile))
	}
	if k8sVersion != "" {
		args = append(args, fmt.Sprintf("--k8s-version=%s", k8sVersion))
	}
	if appProfiles != "" {
		args = append(args, fmt.Sprintf("--app-profiles=%s", appProfiles))
	}

	// Create log file for background output
	logFile := config.LogFilePath("nova-start")
	f, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// Start the process detached
	bgCmd := exec.Command(executable, args...)
	bgCmd.Stdout = f
	bgCmd.Stderr = f
	// Detach from parent process group
	bgCmd.SysProcAttr = nil // Will be set by Start() to create new process group

	if err := bgCmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("failed to start background process: %w", err)
	}

	ui.Success("NOVA starting in background (PID: %d)", bgCmd.Process.Pid)
	ui.Info("Log file: %s", logFile)
	ui.Info("Use 'nova status' to check progress")
	ui.Info("Use 'tail -f %s' to follow logs", logFile)

	return nil
}

func runStart(cmd *cobra.Command, targetTier int, hfToken string, model string, gpuMode string, profile string, k8sVersion string, appProfiles string) error {
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
	if profile != "" {
		switch profile {
		case "minimal":
			cfg.ResourceProfile = config.ResourceProfileMinimal
			ui.Info("Using 'minimal' profile: 1 node, 6 CPUs, 12GB RAM")
		case "cluster":
			cfg.ResourceProfile = config.ResourceProfileCluster
			ui.Info("Using 'cluster' profile: 3 nodes, 4 CPUs/node, 4GB RAM/node")
		default:
			return fmt.Errorf("invalid profile: %s (use: minimal or cluster)", profile)
		}
	}
	if gpuMode != "" {
		switch gpuMode {
		case "nvidia":
			cfg.Minikube.GPUMode = config.GPUModeNVIDIA
			ui.Info("GPU mode: NVIDIA (CUDA acceleration enabled)")
		default:
			return fmt.Errorf("invalid GPU mode: %s (use: nvidia)", gpuMode)
		}
	}
	if k8sVersion != "" {
		cfg.Versions.Kubernetes = k8sVersion
		cfg.Minikube.KubernetesVersion = k8sVersion
		ui.Info("Using Kubernetes version %s", k8sVersion)
	}

	// Parse and apply app profiles (CLI flag overrides config)
	if appProfiles != "" {
		profiles := strings.Split(appProfiles, ",")
		var validProfiles []config.AppProfileType
		for _, p := range profiles {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			profileType := config.AppProfileType(p)
			if err := cfg.ValidateAppProfile(profileType); err != nil {
				return fmt.Errorf("invalid app profile '%s': %w", p, err)
			}
			validProfiles = append(validProfiles, profileType)
		}
		if len(validProfiles) > 0 {
			cfg.SetActiveAppProfiles(validProfiles)
		}
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

	// Show profile info if not already displayed via --profile flag
	if profile == "" {
		nodes, cpus, mem := config.ProfileTopology(cfg.GetEffectiveResourceProfile())
		ui.Info("Using '%s' profile: %d node(s), %d CPUs, %dGB RAM", cfg.GetEffectiveResourceProfile(), nodes, cpus, mem/1024)
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

	// Step 3: Tier 0: Configure Minikube cluster (already started by Foundation Stack)
	progress.StartStep(currentStep)

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

	// Tier 3 URLs (based on active app profiles)
	if targetTier >= 3 {
		ui.Info("  llm-d API: https://llmd.internal.%s/v1", cfg.DNS.Domain)
		// Show URLs based on active app profiles
		if cfg.IsAppEnabled("openwebui") {
			ui.Info("  Open WebUI: https://chat.%s", cfg.DNS.Domain)
		}
		if cfg.IsAppEnabled("helix") {
			ui.Info("  HELIX: https://helix.%s", cfg.DNS.Domain)
		}
		if cfg.IsAppEnabled("opengatellm") {
			ui.Info("  OpenGateLLM: https://opengatellm.%s", cfg.DNS.Domain)
		}
		if cfg.IsAppEnabled("conversations") {
			ui.Info("  Conversations: https://conversations.%s", cfg.DNS.Domain)
		}
		ui.Info("")
		ui.Info("Active app profiles: %v", cfg.GetActiveAppProfiles())
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
