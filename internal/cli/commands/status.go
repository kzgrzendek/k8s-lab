package commands

import (
	"fmt"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/status"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show status of all NOVA components",
		Long: `Displays the current status of all NOVA components including:

  • Minikube cluster (nodes, health, version)
  • Host services (Bind9 DNS, NGINX gateway)
  • Deployed tiers and their components
  • Configuration summary

Use --verbose for detailed component information.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed component information")

	return cmd
}

func runStatus(cmd *cobra.Command, verbose bool) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		ui.Warn("Failed to load config: %v", err)
		ui.Info("Run 'nova setup' to initialize configuration")
		return nil
	}

	ui.Header("NOVA Status")

	// Create status checker
	checker := status.NewChecker(cmd.Context(), cfg)

	// Get system status
	sysStatus, err := checker.GetSystemStatus()
	if err != nil {
		return fmt.Errorf("failed to get system status: %w", err)
	}

	// Display cluster status
	displayClusterStatus(sysStatus.Cluster, verbose)

	// Display host services status
	displayHostServicesStatus(sysStatus.HostServices, verbose)

	// Display deployments status (if cluster is running)
	if sysStatus.Cluster.Running {
		displayDeploymentsStatus(sysStatus.Deployments, verbose, cfg)
	}

	// Display configuration summary
	displayConfigSummary(cfg, verbose)

	return nil
}

func displayClusterStatus(cluster *status.ClusterStatus, verbose bool) {
	ui.Header("Minikube Cluster")

	if cluster.ErrorDetail != "" {
		ui.Error("Error: %s", cluster.ErrorDetail)
		return
	}

	if !cluster.Running {
		ui.Warn("Status: stopped")
		ui.Info("Run 'nova start' to start the cluster")
		return
	}

	if cluster.Healthy {
		ui.Success("Status: running (healthy)")
	} else {
		ui.Warn("Status: running (unhealthy)")
	}

	if cluster.Version != "" {
		ui.Info("Version: %s", cluster.Version)
	}

	if cluster.GPU != "" && cluster.GPU != "none" && cluster.GPU != "disabled" {
		ui.Info("GPU Mode: %s", cluster.GPU)
	} else {
		ui.Info("GPU Mode: disabled (CPU-only)")
	}

	// Display nodes
	if len(cluster.Nodes) > 0 {
		ui.Info("")
		ui.Info("Nodes (%d):", len(cluster.Nodes))
		for _, node := range cluster.Nodes {
			statusIcon := "✓"
			if !strings.Contains(node.Status, "Ready") {
				statusIcon = "✗"
			}

			// Build node display with optional GPU indicator
			nodeName := node.Name
			if node.HasGPU {
				nodeName += " (GPU)"
			}

			if verbose {
				ui.Info("  %s %s - %s (%s)", statusIcon, nodeName, node.Status, node.Roles)
			} else {
				ui.Info("  %s %s - %s", statusIcon, nodeName, node.Status)
			}
		}
	}
}

func displayHostServicesStatus(services *status.HostServicesStatus, verbose bool) {
	ui.Info("")
	ui.Header("Host Services")

	displayComponentStatus(services.Bind9, verbose)
	displayComponentStatus(services.NGINX, verbose)
}

func displayDeploymentsStatus(deployments *status.DeploymentsStatus, verbose bool, cfg *config.Config) {
	ui.Info("")
	ui.Header("Deployed Components")

	// Display Tier 0
	if len(deployments.Tier0Components) > 0 {
		ui.Info("")
		ui.Info("Tier 0 - Minikube Cluster:")
		for _, comp := range deployments.Tier0Components {
			displayComponentStatus(comp, verbose)
		}
	}

	// Display Tier 1
	if len(deployments.Tier1Components) > 0 {
		ui.Info("")
		ui.Info("Tier 1 - Infrastructure:")
		for _, comp := range deployments.Tier1Components {
			displayComponentStatus(comp, verbose)
		}
	} else {
		ui.Info("")
		ui.Info("Tier 1: not deployed")
	}

	if len(deployments.Tier2Components) > 0 {
		ui.Info("")
		ui.Info("Tier 2 - Platform:")
		for _, comp := range deployments.Tier2Components {
			displayComponentStatus(comp, verbose)
		}
	} else {
		ui.Info("")
		ui.Info("Tier 2: not deployed")
	}

	if len(deployments.Tier3Components) > 0 {
		ui.Info("")
		ui.Info("Tier 3 - Applications:")
		for _, comp := range deployments.Tier3Components {
			displayComponentStatus(comp, verbose)
		}
	}

	// Display Tier 2 credentials if available
	if deployments.Tier2Credentials != nil && len(deployments.Tier2Credentials.KeycloakUsers) > 0 {
		DisplayKeycloakCredentials(deployments.Tier2Credentials, cfg)
	}
}

func displayComponentStatus(comp status.ComponentStatus, verbose bool) {
	var statusIcon string
	switch comp.Status {
	case "running", "deployed", "mounted", "configured":
		if comp.Healthy {
			statusIcon = "✓"
		} else {
			statusIcon = "⚠"
		}
	case "stopped", "not configured":
		statusIcon = "✗"
	default:
		statusIcon = "?"
	}

	if verbose && comp.Details != "" {
		ui.Info("  %s %s - %s (%s)", statusIcon, comp.Name, comp.Status, comp.Details)
	} else {
		ui.Info("  %s %s - %s", statusIcon, comp.Name, comp.Status)
	}
}

func displayConfigSummary(cfg *config.Config, verbose bool) {
	ui.Info("")
	ui.Header("Configuration")

	ui.Info("Cluster:")
	ui.Info("Nodes: %d", cfg.Minikube.Nodes)
	ui.Info("CPUs per node: %d", cfg.Minikube.CPUs)
	ui.Info("Memory per node: %dMB", cfg.Minikube.Memory)

	if verbose {
		ui.Info("")
		ui.Info("DNS:")
		ui.Info("Domain: %s", cfg.DNS.Domain)
		ui.Info("Auth Domain: %s", cfg.DNS.AuthDomain)
		ui.Info("Bind9 Port: %d", cfg.DNS.Bind9Port)

		ui.Info("")
		ui.Info("State:")
		ui.Info("Initialized: %v", cfg.State.Initialized)
		ui.Info("Last Deployed Tier: %d", cfg.State.LastDeployedTier)
	}
}
