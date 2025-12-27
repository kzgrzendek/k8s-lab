package cmd

import (
	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/preflight"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "One-time setup of the NOVA environment",
		Long: `Performs initial setup of the NOVA environment including:

  • Checking required dependencies (docker, minikube, mkcert, certutil)
  • Verifying Linux distribution (Ubuntu/Debian)
  • Configuring DNS via systemd-resolved
  • Generating mkcert Root CA
  • Creating initial configuration file

This command should be run once before using 'nova start'.`,
		RunE: runSetup,
	}
}

func runSetup(cmd *cobra.Command, args []string) error {
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

	// Step 3: Configure DNS (TODO: implement in dns package)
	ui.Step("Configuring DNS (systemd-resolved)...")
	ui.Warn("DNS configuration not yet implemented - you may need to configure manually")

	// Step 4: Generate mkcert CA (TODO: implement in pki package)
	ui.Step("Generating Root CA with mkcert...")
	ui.Warn("mkcert CA generation not yet implemented")

	// Step 5: Create config file
	ui.Step("Creating configuration file...")
	cfg := config.Default()
	cfg.State.Initialized = true
	if err := cfg.Save(); err != nil {
		return err
	}
	ui.Success("Configuration saved to %s", config.DefaultConfigPath())

	ui.Header("Setup Complete")
	ui.Info("Run 'nova start' to deploy the lab environment")

	return nil
}
