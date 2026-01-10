package commands

import (
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/foundation"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
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
  • Stops NFS server container

All data is preserved and can be restarted with 'nova start'.`,
		RunE: runStop,
	}
}

func runStop(cmd *cobra.Command, args []string) error {
	ui.Header("Stopping NOVA")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Define stop steps
	steps := []string{
		"Minikube Cluster",
		"Foundation Stack",
	}

	// Create progress tracker
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Step 1: Stop Minikube cluster
	progress.StartStep(currentStep)
	ui.Info("Stopping Minikube cluster...")
	if err := minikube.Stop(cmd.Context()); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to stop Minikube cluster: %w", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 2: Stop Foundation Stack (NGINX, Bind9, NFS, Registry)
	progress.StartStep(currentStep)
	foundationStack := foundation.New(cfg)
	if err := foundationStack.Stop(cmd.Context()); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to stop foundation stack: %w", err)
	}
	progress.CompleteStep(currentStep)

	// Mark all steps complete
	progress.Complete()

	ui.Header("NOVA Stopped")
	ui.Info("Run 'nova start' to restart")
	ui.Success("All components stopped successfully")

	return nil
}
