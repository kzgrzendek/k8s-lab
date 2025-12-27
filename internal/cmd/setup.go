package cmd

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/dns"
	"github.com/kzgrzendek/nova/internal/pki"
	"github.com/kzgrzendek/nova/internal/preflight"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	var skipDNS bool

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "One-time setup of the NOVA environment",
		Long: `Performs initial setup of the NOVA environment including:

  • Checking required dependencies (docker, minikube, mkcert, certutil)
  • Verifying Linux distribution (Ubuntu/Debian)
  • Configuring DNS via resolvconf
  • Generating mkcert Root CA
  • Creating initial configuration file

This command should be run once before using 'nova start'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(cmd, skipDNS)
		},
	}

	cmd.Flags().BoolVar(&skipDNS, "skip-dns", false, "skip DNS configuration")

	return cmd
}

func runSetup(cmd *cobra.Command, skipDNS bool) error {
	ui.Header("NOVA Setup")

	// Step 1: Run preflight checks
	ui.Step("Checking dependencies...")
	checker := preflight.NewChecker()
	if err := checker.CheckAll(cmd.Context()); err != nil {
		return err
	}
	ui.Success("All dependencies satisfied")

	// Step 2: Check system requirements
	ui.Step("Checking system requirements...")
	if err := checker.CheckSystem(); err != nil {
		return err
	}
	ui.Success("System requirements met")

	// Step 3: Load or create config
	cfg := config.LoadOrDefault()

	// Step 4: Configure DNS
	if !skipDNS {
		ui.Step("Configuring DNS (resolvconf)...")

		// Check if resolvconf is available
		if err := dns.CheckResolvconfAvailable(); err != nil {
			ui.Warn("resolvconf not available: %v", err)
			ui.Info("Skipping DNS configuration. You'll need to manually configure DNS for:")
			ui.Info("  • %s", cfg.DNS.Domain)
			ui.Info("  • %s", cfg.DNS.AuthDomain)
			ui.Info("Add this to your DNS: nameserver 127.0.0.1#%d", cfg.DNS.Bind9Port)
		} else {
			// Check if already configured
			if dns.IsConfigured() {
				ui.Info("DNS already configured")
			} else {
				// Configure DNS
				domains := []string{cfg.DNS.Domain, cfg.DNS.AuthDomain}
				if err := dns.ConfigureResolvconf(domains, cfg.DNS.Bind9Port); err != nil {
					ui.Warn("Failed to configure DNS: %v", err)
					ui.Info("You may need to configure DNS manually")
				} else {
					ui.Success("DNS configured for %s and %s", cfg.DNS.Domain, cfg.DNS.AuthDomain)
				}
			}
		}
	} else {
		ui.Info("Skipping DNS configuration (--skip-dns)")
	}

	// Step 5: Generate mkcert CA
	ui.Step("Installing mkcert Root CA...")

	// Check if already installed
	installed, err := pki.IsInstalled()
	if err != nil {
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
			return fmt.Errorf("failed to install mkcert CA: %w", err)
		}
		ui.Success("mkcert Root CA installed")
	}

	// Step 6: Generate Kubernetes secret YAML
	ui.Step("Generating Kubernetes CA secret...")
	secretPath := pki.GetDefaultSecretPath()
	if err := pki.GenerateKubernetesSecret(secretPath); err != nil {
		return fmt.Errorf("failed to generate CA secret: %w", err)
	}
	ui.Success("CA secret saved to %s", secretPath)

	// Step 7: Save config
	ui.Step("Saving configuration...")
	cfg.State.Initialized = true
	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Success("Configuration saved to %s", config.DefaultConfigPath())

	ui.Header("Setup Complete")
	ui.Info("Run 'nova start' to deploy the lab environment")
	ui.Info("")
	ui.Info("Configuration:")
	ui.Info("  • DNS domains: %s, %s", cfg.DNS.Domain, cfg.DNS.AuthDomain)
	ui.Info("  • Bind9 port: %d", cfg.DNS.Bind9Port)
	ui.Info("  • Minikube: %d nodes, %d CPUs, %dMB RAM", cfg.Minikube.Nodes, cfg.Minikube.CPUs, cfg.Minikube.Memory)

	return nil
}
