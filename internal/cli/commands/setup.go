package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
	"github.com/kzgrzendek/nova/internal/setup/preflight"
	"github.com/kzgrzendek/nova/internal/setup/system/dns"
	"github.com/kzgrzendek/nova/internal/setup/system/sysctl"
	"github.com/kzgrzendek/nova/internal/setup/system/systemd"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var skipDNS bool
	var rootless bool
	var gpuMode string
	var profile string
	var appProfiles string

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "One-time setup of the NOVA environment",
		Long: `Performs initial setup of the NOVA environment including:

  • Checking required dependencies (docker, minikube, mkcert, certutil)
  • Verifying Linux distribution (Ubuntu/Debian)
  • Configuring DNS via resolvconf (requires sudo)
  • Generating mkcert Root CA (requires sudo)
  • Creating initial configuration file

Resource profiles:
  • minimal: 1 node, 6 CPUs, 12GB RAM (default, supports CPU and GPU modes)
  • cluster: 3 nodes, 4 CPUs/node, 4GB RAM/node (GPU mode only)

GPU modes:
  • (none): CPU inference (default, slower but no GPU required)
  • nvidia: NVIDIA GPU acceleration via CUDA

App profiles (Tier 3 applications):
  • openwebui: Chat interface with Open WebUI (default)
  • lasuite: French government AI stack (OpenGateLLM + Conversations)
  • lab: JupyterHub for ML development (default)
  Multiple profiles can be combined: --app-profiles=openwebui,lab

This command should be run once before using 'nova start'.
Re-running setup with different --gpu or --profile requires 'nova delete' first.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, skipDNS, rootless, gpuMode, profile, appProfiles)
		},
	}

	cmd.Flags().BoolVar(&skipDNS, "skip-dns", false, "skip DNS configuration (fail if resolvconf unavailable)")
	cmd.Flags().BoolVar(&rootless, "rootless", false, "rootless mode - skip DNS and warn instead of failing")
	cmd.Flags().StringVar(&gpuMode, "gpu", "", "GPU mode: nvidia (omit for CPU mode)")
	cmd.Flags().StringVar(&profile, "profile", "", "resource profile: minimal (1 node) or cluster (3 nodes)")
	cmd.Flags().StringVar(&appProfiles, "app-profiles", "", "app profiles to activate (comma-separated): openwebui,lasuite,lab")

	return cmd
}

func runSetup(cmd *cobra.Command, skipDNS bool, rootless bool, gpuMode string, profile string, appProfiles string) error {
	ui.Header("NOVA Setup")

	// Define setup steps
	steps := []string{
		"Check dependencies",
		"Check system requirements",
		"Configure system limits",
		"Load configuration",
		"Check GPU configuration",
		"Configure DNS",
		"Install mkcert CA",
		"Generate CA secret",
		"Install shutdown hook",
		"Save configuration",
	}

	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// If not in rootless mode and not skipping DNS, prompt for sudo password upfront
	if !rootless && !skipDNS {
		ui.Step("Requesting sudo privileges for DNS configuration...")
		ui.Info("")
		ui.Info("NOVA needs sudo access for DNS configuration.")
		ui.Info("")
		ui.Info("What this will do:")
		ui.Info("Create /etc/resolvconf/resolv.conf.d/nova.conf (dedicated config file)")
		ui.Info("Configure nameserver for NOVA domains (*.nova.local)")
		ui.Info("This will NOT modify your existing DNS configuration")
		ui.Info("")
		ui.Info("If you prefer not to use sudo, rerun with --rootless flag")
		ui.Info("(You'll need to manually configure DNS in rootless mode)")
		ui.Info("")
		sudoCmd := exec.Command("sudo", "-v")
		if err := sudoCmd.Run(); err != nil {
			return fmt.Errorf("sudo authentication failed - DNS configuration requires sudo privileges")
		}
		ui.Success("Sudo privileges granted")
		ui.Info("")
	}

	// Step 1: Run preflight checks
	progress.StartStep(currentStep)
	checker := preflight.NewChecker()
	if err := checker.CheckAll(cmd.Context()); err != nil {
		progress.FailStep(currentStep, err)
		return err
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 2: Check system requirements
	progress.StartStep(currentStep)
	if err := checker.CheckSystem(); err != nil {
		progress.FailStep(currentStep, err)
		return err
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 3: Configure system limits (inotify for Kubernetes)
	progress.StartStep(currentStep)
	if !sysctl.IsConfigured() {
		limits := sysctl.DefaultInotifyLimits()
		if err := sysctl.ConfigureInotifyLimits(limits); err != nil {
			ui.Warn("Failed to configure inotify limits: %v", err)
			ui.Info("You may need to manually set:")
			ui.Info("  fs.inotify.max_user_instances=%d", limits.MaxUserInstances)
			ui.Info("  fs.inotify.max_user_watches=%d", limits.MaxUserWatches)
		} else {
			ui.Success("Configured inotify limits for Kubernetes")
		}
	} else {
		ui.Info("System limits already configured")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 4: Load or create config
	progress.StartStep(currentStep)
	cfg := config.LoadOrDefault()

	// Determine requested profile (CLI flag or existing config)
	requestedProfile := cfg.GetEffectiveResourceProfile()
	if profile != "" {
		switch profile {
		case "minimal":
			requestedProfile = config.ResourceProfileMinimal
		case "cluster":
			requestedProfile = config.ResourceProfileCluster
		default:
			progress.FailStep(currentStep, fmt.Errorf("invalid profile: %s", profile))
			return fmt.Errorf("invalid profile: %s (use: minimal or cluster)", profile)
		}
	}

	// Determine requested GPU mode (CLI flag or existing config)
	requestedGPUMode := cfg.Minikube.GPUMode
	if gpuMode != "" {
		switch gpuMode {
		case "nvidia":
			requestedGPUMode = config.GPUModeNVIDIA
		default:
			progress.FailStep(currentStep, fmt.Errorf("invalid GPU mode: %s", gpuMode))
			return fmt.Errorf("invalid GPU mode: %s (use: nvidia, or omit for CPU mode)", gpuMode)
		}
	}

	// Check if cluster was deployed with different settings
	if err := cfg.ValidateConfigChange(requestedProfile, requestedGPUMode); err != nil {
		progress.FailStep(currentStep, err)
		return err
	}

	// Apply requested settings
	cfg.ResourceProfile = requestedProfile
	cfg.Minikube.GPUMode = requestedGPUMode

	// Parse and apply app profiles (CLI flag or use defaults)
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
				progress.FailStep(currentStep, err)
				return fmt.Errorf("invalid app profile '%s': %w", p, err)
			}
			validProfiles = append(validProfiles, profileType)
		}
		if len(validProfiles) > 0 {
			cfg.SetActiveAppProfiles(validProfiles)
			ui.Info("App profiles: %v", validProfiles)
		}
	}

	progress.CompleteStep(currentStep)
	currentStep++

	// Step 5: Check GPU configuration
	progress.StartStep(currentStep)
	gpuCfg, err := checker.CheckGPU(cmd.Context(), string(cfg.Minikube.GPUMode))
	if err != nil {
		// GPU check failed - this could be an unsupported mode or validation error
		progress.FailStep(currentStep, err)
		return fmt.Errorf("GPU configuration failed: %w", err)
	}
	// Display detected/configured GPU mode
	if gpuCfg.Mode == shared.ModeNVIDIA {
		ui.Info("GPU mode: NVIDIA (CUDA acceleration enabled)")
	} else {
		ui.Info("GPU mode: CPU (inference will use CPU)")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 6: Configure DNS
	progress.StartStep(currentStep)
	if rootless {
		ui.Info("Skipping DNS configuration (--rootless mode)")
		ui.Warn("You'll need to manually configure DNS for:")
		ui.Info("%s", cfg.DNS.Domain)
		ui.Info("%s", cfg.DNS.AuthDomain)
		ui.Info("Add nameserver: 127.0.0.1#%d", cfg.DNS.Bind9Port)
		progress.CompleteStep(currentStep)
	} else if !skipDNS {
		// Check if resolvconf is available - FAIL if not available
		if err := dns.CheckResolvconfAvailable(); err != nil {
			ui.Error("DNS configuration failed")
			ui.Info("")
			ui.Info("resolvconf is required but not available:")
			ui.Info("  %v", err)
			ui.Info("")
			ui.Info("Options:")
			ui.Info("  1. Install resolvconf and run setup again")
			ui.Info("  2. Run setup with --rootless to skip DNS and continue")
			ui.Info("")
			progress.FailStep(currentStep, err)
			return fmt.Errorf("resolvconf not available - install it or use --rootless")
		}

		// Always reconfigure DNS (supports updates)
		domains := []string{cfg.DNS.Domain, cfg.DNS.AuthDomain}
		if err := dns.ConfigureResolvconf(domains, cfg.DNS.Bind9Port); err != nil {
			ui.Error("Failed to configure DNS")
			ui.Info("")
			ui.Info("Error: %v", err)
			ui.Info("")
			ui.Info("Make sure you have sudo privileges and try again")
			ui.Info("Or run setup with --rootless to skip DNS and continue")
			ui.Info("")
			progress.FailStep(currentStep, err)
			return fmt.Errorf("DNS configuration failed: %w", err)
		}
		ui.Success("DNS configured for %s and %s", cfg.DNS.Domain, cfg.DNS.AuthDomain)
		progress.CompleteStep(currentStep)
	} else {
		ui.Info("Skipping DNS configuration (--skip-dns)")
		ui.Warn("You'll need to manually configure DNS for:")
		ui.Info("%s", cfg.DNS.Domain)
		ui.Info("%s", cfg.DNS.AuthDomain)
		ui.Info("Add nameserver: 127.0.0.1#%d", cfg.DNS.Bind9Port)
		progress.CompleteStep(currentStep)
	}
	currentStep++

	// Step 7: Install mkcert CA
	progress.StartStep(currentStep)

	// Check if already installed
	installed, err := pki.IsInstalled()
	if err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to check mkcert status: %w", err)
	}

	if installed {
		ui.Info("mkcert CA already installed")
		// Show CA info
		if info, err := pki.GetCAInfo(); err == nil {
			ui.Debug("%s", info)
		}
	} else {
		// Install CA
		if err := pki.InstallRootCA(); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to install mkcert CA: %w", err)
		}
		ui.Success("mkcert Root CA installed")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 8: Generate Kubernetes secret YAML
	progress.StartStep(currentStep)
	secretPath := pki.GetDefaultSecretPath()
	if err := pki.GenerateKubernetesSecret(secretPath); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to generate CA secret: %w", err)
	}
	ui.Success("CA secret saved to %s", secretPath)
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 9: Install shutdown hook (systemd user service)
	progress.StartStep(currentStep)
	if systemd.IsInstalled() {
		ui.Info("Shutdown hook already installed")
	} else {
		// Get the current executable path for the systemd service
		novaBinary, err := os.Executable()
		if err != nil {
			ui.Warn("Could not determine nova binary path: %v", err)
			ui.Info("Skipping shutdown hook installation")
		} else {
			if err := systemd.Install(novaBinary); err != nil {
				ui.Warn("Failed to install shutdown hook: %v", err)
				ui.Info("Nova will not automatically stop on system shutdown/suspend")
				ui.Info("You can manually run 'nova stop' before shutting down")
			} else {
				ui.Success("Shutdown hook installed (nova will stop on shutdown/suspend)")
			}
		}
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 10: Save config
	progress.StartStep(currentStep)
	cfg.State.Initialized = true
	// Save deployed profile and GPU mode for change detection on re-setup
	cfg.State.DeployedProfile = cfg.GetEffectiveResourceProfile()
	cfg.State.DeployedGPUMode = cfg.Minikube.GPUMode
	if err := cfg.Save(); err != nil {
		progress.FailStep(currentStep, err)
		return err
	}
	ui.Success("Configuration saved to %s", config.DefaultConfigPath())
	progress.CompleteStep(currentStep)

	// Mark all steps complete
	progress.Complete()

	ui.Header("Setup Complete")
	ui.Info("Run 'nova start' to deploy the lab environment")
	ui.Info("")
	ui.Info("Configuration:")
	ui.Info("DNS domains: %s, %s", cfg.DNS.Domain, cfg.DNS.AuthDomain)
	ui.Info("Bind9 port: %d", cfg.DNS.Bind9Port)
	nodes, cpus, mem := config.ProfileTopology(cfg.GetEffectiveResourceProfile())
	totalCPUs := nodes * cpus
	totalRAM := nodes * mem
	ui.Info("Profile: %s (%d nodes, %d CPUs/node [%d total], %dMB RAM/node [%dMB total])",
		cfg.GetEffectiveResourceProfile(), nodes, cpus, totalCPUs, mem, totalRAM)
	ui.Info("App profiles: %v", cfg.GetActiveAppProfiles())

	return nil
}
