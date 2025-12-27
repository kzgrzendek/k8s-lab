package cmd

import (
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

	// TODO: Implement stop logic
	// 1. Stop Minikube
	// 2. Stop NGINX container
	// 3. Stop Bind9 container

	ui.Step("Stopping Minikube cluster...")
	ui.Warn("Minikube stop not yet implemented")

	ui.Step("Stopping NGINX gateway...")
	ui.Warn("NGINX stop not yet implemented")

	ui.Step("Stopping Bind9 DNS...")
	ui.Warn("Bind9 stop not yet implemented")

	ui.Header("NOVA Stopped")
	ui.Info("Run 'nova start' to restart")

	return nil
}
