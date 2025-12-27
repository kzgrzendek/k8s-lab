package cmd

import (
	"fmt"
	"os/exec"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/dns"
	"github.com/kzgrzendek/nova/internal/pki"
	"github.com/kzgrzendek/nova/internal/preflight"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var skipDNS bool
	var rootless bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "One-time setup of the NOVA environment",
		Long: `Performs initial setup of the NOVA environment including:

  • Checking required dependencies (docker, minikube, mkcert, certutil)
  • Verifying Linux distribution (Ubuntu/Debian)
  • Configuring DNS via resolvconf (requires sudo)
  • Generating mkcert Root CA (requires sudo)
  • Creating initial configuration file

This command should be run once before using 'nova start'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, skipDNS, rootless)
		},
	}

	cmd.Flags().BoolVar(&skipDNS, "skip-dns", false, "skip DNS configuration (fail if resolvconf unavailable)")
	cmd.Flags().BoolVar(&rootless, "rootless", false, "rootless mode - skip DNS and warn instead of failing")

	return cmd
}

func runSetup(cmd *cobra.Command, skipDNS bool, rootless bool) error {
	ui.Header("NOVA Setup")

	// Define setup steps
	steps := []string{
		"Check dependencies",
		"Check system requirements",
		"Load configuration",
		"Check GPU configuration",
		"Configure DNS",
		"Install mkcert CA",
		"Generate CA secret",
		"Save configuration",
	}

	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// If not in rootless mode and not skipping DNS, prompt for sudo password upfront
	if !rootless && !skipDNS {
		ui.Step("Requesting sudo privileges...")
		ui.Info("DNS configuration requires sudo access")
		sudoCmd := exec.Command("sudo", "-v")
		if err := sudoCmd.Run(); err != nil {
			return fmt.Errorf("sudo authentication failed - DNS configuration requires sudo privileges")
		}
		ui.Success("Sudo privileges granted")
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

	// Step 3: Load or create config
	progress.StartStep(currentStep)
	cfg := config.LoadOrDefault()
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 4: Check GPU configuration
	progress.StartStep(currentStep)
	gpuCfg, err := checker.CheckGPU(cmd.Context(), cfg.Minikube.GPUs)
	if err != nil {
		ui.Warn("GPU check failed: %v", err)
		ui.Info("Continuing with CPU-only mode...")
		cfg.Minikube.GPUs = "none"
	} else if !gpuCfg.Enabled {
		ui.Info("Proceeding in CPU-only mode")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 5: Configure DNS
	progress.StartStep(currentStep)
	if rootless {
		ui.Info("Skipping DNS configuration (--rootless mode)")
		ui.Warn("You'll need to manually configure DNS for:")
		ui.Info("  • %s", cfg.DNS.Domain)
		ui.Info("  • %s", cfg.DNS.AuthDomain)
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
		ui.Info("  • %s", cfg.DNS.Domain)
		ui.Info("  • %s", cfg.DNS.AuthDomain)
		ui.Info("Add nameserver: 127.0.0.1#%d", cfg.DNS.Bind9Port)
		progress.CompleteStep(currentStep)
	}
	currentStep++

	// Step 5: Generate mkcert CA
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
			ui.Debug(info)
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

	// Step 6: Generate Kubernetes secret YAML
	progress.StartStep(currentStep)
	secretPath := pki.GetDefaultSecretPath()
	if err := pki.GenerateKubernetesSecret(secretPath); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to generate CA secret: %w", err)
	}
	ui.Success("CA secret saved to %s", secretPath)
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 7: Save config
	progress.StartStep(currentStep)
	cfg.State.Initialized = true
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
	ui.Info("  • DNS domains: %s, %s", cfg.DNS.Domain, cfg.DNS.AuthDomain)
	ui.Info("  • Bind9 port: %d", cfg.DNS.Bind9Port)
	totalCPUs := cfg.Minikube.Nodes * cfg.Minikube.CPUs
	totalRAM := cfg.Minikube.Nodes * cfg.Minikube.Memory
	ui.Info("  • Minikube: %d nodes, %d CPUs/node (%d total), %dMB RAM/node (%dMB total)",
		cfg.Minikube.Nodes, cfg.Minikube.CPUs, totalCPUs, cfg.Minikube.Memory, totalRAM)

	return nil
}
