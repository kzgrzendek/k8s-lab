package cmd

import (
	"github.com/kzgrzendek/nova/internal/minikube"
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

	// TODO: Stop host services
	ui.Step("Stopping NGINX gateway...")
	ui.Warn("NGINX stop not yet implemented")

	ui.Step("Stopping Bind9 DNS...")
	ui.Warn("Bind9 stop not yet implemented")

	ui.Header("NOVA Stopped")
	ui.Info("Run 'nova start' to restart")

	return nil
}
