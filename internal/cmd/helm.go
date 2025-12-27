package cmd

import (
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newHelmCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "helm",
		Short:              "Run helm against the NOVA cluster",
		Long:               `Runs helm commands using the embedded Helm SDK against the NOVA cluster.`,
		DisableFlagParsing: true,
		RunE:               runHelm,
	}
}

func runHelm(cmd *cobra.Command, args []string) error {
	// TODO: Implement using Helm SDK
	// This will use the embedded Helm SDK rather than calling an external binary
	ui.Warn("Helm SDK integration not yet implemented")
	ui.Info("For now, use: minikube kubectl -- ... or helm directly with --kubeconfig")
	return nil
}
