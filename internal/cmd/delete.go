package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kzgrzendek/nova/internal/config"
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

	// TODO: Implement delete logic
	// 1. Delete Minikube cluster
	// 2. Remove NGINX container
	// 3. Remove Bind9 container
	// 4. If purge: remove config directory and DNS config

	ui.Step("Deleting Minikube cluster...")
	ui.Warn("Minikube delete not yet implemented")

	ui.Step("Removing NGINX gateway...")
	ui.Warn("NGINX remove not yet implemented")

	ui.Step("Removing Bind9 DNS...")
	ui.Warn("Bind9 remove not yet implemented")

	if purge {
		ui.Step("Purging configuration and certificates...")
		if err := os.RemoveAll(config.ConfigDir()); err != nil {
			ui.Warn("Failed to remove config directory: %v", err)
		}
		ui.Success("Configuration purged")
	}

	ui.Header("NOVA Deleted")

	return nil
}
