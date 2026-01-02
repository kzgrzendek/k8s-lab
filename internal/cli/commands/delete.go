package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/dns/bind9"
	"github.com/kzgrzendek/nova/internal/host/gateway/nginx"
	"github.com/kzgrzendek/nova/internal/setup/system/dns"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var purge bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the NOVA lab environment",
		Long: `Deletes all NOVA lab components:

  • Deletes Minikube cluster
  • Removes NGINX gateway container
  • Removes Bind9 DNS container

Use --purge to also remove configuration, certificates, and generated files.
Use --yes to skip confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, purge, yes)
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false,
		"also remove config, certificates, and generated files")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"skip confirmation prompt")

	return cmd
}

func runDelete(cmd *cobra.Command, purge, yes bool) error {
	ui.Header("Delete NOVA")

	if !yes {
		msg := "This will delete the entire lab environment."
		if purge {
			msg += " Including configuration and certificates."
		}
		msg += " Continue? [y/N] "

		fmt.Print(msg)
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			ui.Info("Aborted")
			return nil
		}
	}

	// Define delete steps
	steps := []string{
		"Minikube Cluster",
		"NGINX Gateway",
		"Bind9 DNS Server",
	}
	if purge {
		steps = append(steps, "DNS Configuration", "Configuration Directory")
	}

	// Create progress tracker
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Step 1: Delete Minikube cluster
	progress.StartStep(currentStep)
	ui.Info("Deleting Minikube cluster...")
	if err := minikube.Delete(cmd.Context()); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to delete Minikube cluster: %w", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 2: Remove NGINX gateway
	progress.StartStep(currentStep)
	ui.Info("Removing NGINX gateway...")
	if nginx.IsRunning(cmd.Context()) {
		if err := nginx.Delete(cmd.Context()); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to remove NGINX gateway: %w", err)
		}
	} else {
		ui.Warn("NGINX gateway not running (already deleted or never started)")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 3: Remove Bind9 DNS
	progress.StartStep(currentStep)
	ui.Info("Removing Bind9 DNS server...")
	if bind9.IsRunning(cmd.Context()) {
		if err := bind9.Delete(cmd.Context()); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to remove Bind9 DNS server: %w", err)
		}
	} else {
		ui.Warn("Bind9 DNS server not running (already deleted or never started)")
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Purge configuration if requested
	if purge {
		// Step 4: Remove DNS configuration
		progress.StartStep(currentStep)
		ui.Info("Removing DNS configuration...")
		ui.Info("You may be prompted for your sudo password to remove DNS configuration")
		if err := dns.RemoveResolvconf(); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to remove DNS configuration: %w", err)
		}
		progress.CompleteStep(currentStep)
		currentStep++

		// Step 5: Remove config directory
		progress.StartStep(currentStep)
		ui.Info("Removing configuration directory...")
		if err := os.RemoveAll(config.ConfigDir()); err != nil {
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to remove configuration directory: %w", err)
		}
		ui.Success("Removed %s", config.ConfigDir())
		progress.CompleteStep(currentStep)
	}

	// Mark all steps complete
	progress.Complete()

	ui.Header("NOVA Deleted")
	ui.Success("All components deleted successfully")

	return nil
}
