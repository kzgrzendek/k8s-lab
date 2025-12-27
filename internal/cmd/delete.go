package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/dns"
	"github.com/kzgrzendek/nova/internal/minikube"
	"github.com/kzgrzendek/nova/internal/ui"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var purge bool
	var force bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the NOVA lab environment",
		Long: `Deletes all NOVA lab components:

  • Deletes Minikube cluster
  • Removes NGINX gateway container
  • Removes Bind9 DNS container

Use --purge to also remove configuration, certificates, and generated files.
Use --force to skip confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, purge, force)
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false,
		"also remove config, certificates, and generated files")
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"skip confirmation prompt")

	return cmd
}

func runDelete(cmd *cobra.Command, purge, force bool) error {
	ui.Header("Delete NOVA")

	if !force {
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

	// Delete Minikube cluster
	ui.Step("Deleting Minikube cluster...")
	if err := minikube.Delete(cmd.Context()); err != nil {
		ui.Warn("Failed to delete Minikube: %v", err)
	} else {
		ui.Success("Minikube cluster deleted")
	}

	// TODO: Remove host services
	ui.Step("Removing NGINX gateway...")
	ui.Warn("NGINX remove not yet implemented")

	ui.Step("Removing Bind9 DNS...")
	ui.Warn("Bind9 remove not yet implemented")

	if purge {
		ui.Step("Purging configuration and certificates...")

		// Remove DNS configuration
		if err := dns.RemoveResolvconf(); err != nil {
			ui.Warn("Failed to remove DNS config: %v", err)
		} else {
			ui.Info("  • Removed DNS configuration")
		}

		// Remove config directory
		if err := os.RemoveAll(config.ConfigDir()); err != nil {
			ui.Warn("Failed to remove config directory: %v", err)
		} else {
			ui.Info("  • Removed configuration directory")
		}

		ui.Success("Configuration purged")
	}

	ui.Header("NOVA Deleted")

	return nil
}
