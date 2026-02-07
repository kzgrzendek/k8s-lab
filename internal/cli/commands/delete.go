package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/foundation"
	"github.com/kzgrzendek/nova/internal/host/mount"
	"github.com/kzgrzendek/nova/internal/setup/system/dns"
	"github.com/kzgrzendek/nova/internal/setup/system/systemd"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	var purge bool
	var yes bool
	var rootless bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the NOVA lab environment",
		Long: `Deletes all NOVA lab components:

  • Deletes Minikube cluster
  • Removes NGINX gateway container
  • Removes Bind9 DNS container
  • Removes NFS server container
  • Removes local registry container

Use --purge to also remove configuration, certificates, generated files, and Docker network.
Use --rootless to skip DNS cleanup (requires manual cleanup).
Use --yes to skip confirmation prompt.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDelete(cmd, purge, yes, rootless)
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false,
		"also remove config, certificates, and generated files")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false,
		"skip confirmation prompt")
	cmd.Flags().BoolVar(&rootless, "rootless", false,
		"rootless mode - skip DNS cleanup and warn instead")

	return cmd
}

func runDelete(cmd *cobra.Command, purge, yes, rootless bool) error {
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

	// Load config (needed for foundation stack)
	cfg, err := config.Load()
	if err != nil {
		// If config doesn't exist, we can still delete minikube and foundation
		ui.Warn("Failed to load config: %v", err)
		cfg = nil
	}

	// Define delete steps
	steps := []string{
		"Cluster & Foundation",
	}
	if purge {
		steps = append(steps, "DNS Configuration", "Shutdown Hook", "Configuration Directory")
	}

	// Create progress tracker
	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Step 1: Delete Minikube cluster and Foundation Stack
	// Order: containers → minikube → network (network must be removed last)
	progress.StartStep(currentStep)

	// Kill any orphaned mount processes FIRST
	// This ensures cleanup happens even if minikube delete fails or was interrupted previously
	ui.Info("Stopping minikube mount processes...")
	_ = mount.Delete(cmd.Context()) // Best effort, ignore errors

	// Delete foundation containers BEFORE minikube (but NOT the network yet)
	// This ensures they're cleaned up even if minikube delete fails
	var foundationStack *foundation.Foundation
	if cfg != nil {
		ui.Info("Deleting foundation containers...")
		foundationStack = foundation.New(cfg)
		foundationStack.DeleteContainers(cmd.Context()) // Best effort
	}

	// Delete minikube cluster (this disconnects it from the nova network)
	ui.Info("Deleting Minikube cluster...")
	if err := minikube.Delete(cmd.Context()); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to delete Minikube cluster: %w", err)
	}

	// Now remove the nova network (after minikube is deleted)
	if foundationStack != nil {
		_ = foundationStack.DeleteNetwork(cmd.Context()) // Best effort
	}

	// Reset deployment state (allows profile/GPU mode changes without --purge)
	// This preserves config, cached images, and models while allowing reconfiguration
	if cfg != nil {
		cfg.State.LastDeployedTier = 0
		cfg.State.DeployedProfile = ""
		cfg.State.DeployedGPUMode = ""
		if err := cfg.Save(); err != nil {
			ui.Warn("Failed to reset deployment state: %v", err)
		}
	}

	progress.CompleteStep(currentStep)
	currentStep++

	// Purge configuration if requested
	if purge {
		// Step 2: Remove DNS configuration
		progress.StartStep(currentStep)
		if rootless {
			ui.Info("Skipping DNS cleanup (--rootless mode)")
			ui.Warn("You'll need to manually remove DNS configuration:")
			ui.Info("Run: sudo rm -f /etc/resolvconf/resolv.conf.d/nova.conf")
			ui.Info("Then: sudo resolvconf -u")
			progress.CompleteStep(currentStep)
		} else {
			ui.Info("Removing DNS configuration...")
			ui.Info("You may be prompted for your sudo password to remove DNS configuration")
			if err := dns.RemoveResolvconf(); err != nil {
				progress.FailStep(currentStep, err)
				return fmt.Errorf("failed to remove DNS configuration: %w", err)
			}
			progress.CompleteStep(currentStep)
		}
		currentStep++

		// Step 3: Remove shutdown hook (systemd service)
		progress.StartStep(currentStep)
		if systemd.IsInstalled() {
			ui.Info("Removing shutdown hook...")
			if err := systemd.Uninstall(); err != nil {
				ui.Warn("Failed to remove shutdown hook: %v", err)
			} else {
				ui.Success("Shutdown hook removed")
			}
		} else {
			ui.Info("Shutdown hook not installed")
		}
		progress.CompleteStep(currentStep)
		currentStep++

		// Step 4: Remove config directory
		progress.StartStep(currentStep)
		ui.Info("Removing configuration directory...")
		configDir := config.ConfigDir()
		if err := os.RemoveAll(configDir); err != nil {
			if os.IsPermission(err) {
				progress.FailStep(currentStep, err)
				return fmt.Errorf("failed to remove configuration directory (permission denied).\nFix with: sudo rm -rf %s", configDir)
			}
			progress.FailStep(currentStep, err)
			return fmt.Errorf("failed to remove configuration directory: %w", err)
		}
		ui.Success("Removed %s", configDir)
		progress.CompleteStep(currentStep)
		currentStep++
	}

	// Mark all steps complete
	progress.Complete()

	ui.Header("NOVA Deleted")
	ui.Success("All components deleted successfully")

	return nil
}
