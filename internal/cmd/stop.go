package cmd

import (
	"github.com/kzgrzendek/nova/internal/bind9"
	"github.com/kzgrzendek/nova/internal/minikube"
	"github.com/kzgrzendek/nova/internal/nginx"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the NOVA lab environment",
		Long: `Stops all NOVA lab components while preserving state:

  • Stops Minikube cluster (preserves disk state)
  • Stops NGINX gateway container
  • Stops Bind9 DNS container

All data is preserved and can be restarted with 'nova start'.`,
		RunE: runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	ui.Header("Stopping NOVA")

	// Stop Minikube cluster
	ui.Step("Stopping Minikube cluster...")
	if err := minikube.Stop(cmd.Context()); err != nil {
		ui.Warn("Failed to stop Minikube: %v", err)
	} else {
		ui.Success("Minikube cluster stopped")
	}

	// Stop host services
	ui.Step("Stopping host services...")

	// Stop NGINX gateway
	ui.Info("  • Stopping NGINX gateway...")
	if err := nginx.Stop(cmd.Context()); err != nil {
		ui.Warn("Failed to stop NGINX: %v", err)
	} else {
		ui.Success("NGINX gateway stopped")
	}

	// Stop Bind9 DNS
	ui.Info("  • Stopping Bind9 DNS server...")
	if err := bind9.Stop(cmd.Context()); err != nil {
		ui.Warn("Failed to stop Bind9: %v", err)
	} else {
		ui.Success("Bind9 DNS server stopped")
	}

	ui.Header("NOVA Stopped")
	ui.Info("Run 'nova start' to restart")

	return nil
}
