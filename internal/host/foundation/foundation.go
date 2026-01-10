// Package foundation provides orchestration for NOVA's foundation stack services.
//
// The foundation stack consists of the nova Docker network and minikube cluster,
// which must be running before other host services start:
//   - Nova Docker network
//   - Minikube cluster (started first, gets automatic IPs from Docker)
//   - NGINX Gateway (reverse proxy to cluster, discovers minikube IP)
//   - Bind9 DNS (local DNS for *.nova.local domains)
//   - NFS Server (persistent storage for models, tier 3 only)
//   - Registry (local container registry, tier 3 only)
//
// All services use Docker's automatic IP allocation to avoid conflicts.
package foundation

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/host/dns/bind9"
	"github.com/kzgrzendek/nova/internal/host/gateway/nginx"
	"github.com/kzgrzendek/nova/internal/host/nfs"
	"github.com/kzgrzendek/nova/internal/host/registry"
	"github.com/kzgrzendek/nova/internal/tools/docker"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// Foundation manages NOVA's foundation stack of host services.
// The foundation includes the network and minikube cluster.
type Foundation struct {
	cfg *config.Config
}

// New creates a new Foundation manager.
func New(cfg *config.Config) *Foundation {
	return &Foundation{
		cfg: cfg,
	}
}

// Start deploys the complete foundation stack.
//
// Service startup order:
//  1. Create nova Docker network
//  2. Start Minikube cluster (gets automatic IPs from Docker)
//  3. Start NGINX Gateway (discovers minikube IP after it's running)
//  4. Start Bind9 DNS
//  5. Start NFS Server (tier >= 3 only)
//  6. Start Registry (tier >= 3 only, needs Bind9 for DNS)
//
// Error handling: Fails fast with clear errors, no automatic rollback.
// If an error occurs, use Stop() or Delete() to clean up.
func (f *Foundation) Start(ctx context.Context, tier int) error {
	ui.Header("Foundation Stack")

	// Step 1: Create nova Docker network
	ui.Step("Creating nova Docker network...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	if err := dockerClient.CreateNetwork(ctx, "nova"); err != nil {
		return fmt.Errorf("failed to create nova network: %w", err)
	}
	ui.Success("Nova network ready")

	// Step 2: Start Minikube cluster (gets automatic IPs from Docker)
	ui.Step("Starting Minikube cluster...")
	running, err := minikube.IsRunning(ctx)
	if err != nil {
		ui.Warn("Failed to check cluster status: %v", err)
	}

	if running {
		ui.Info("Minikube cluster already running")
	} else {
		if err := minikube.StartCluster(ctx, f.cfg); err != nil {
			return fmt.Errorf("failed to start Minikube: %w", err)
		}
	}
	ui.Success("Minikube cluster started")

	// Step 3: Start NGINX Gateway (discovers minikube IP now that it's running)
	ui.Step("Starting NGINX Gateway...")
	if err := nginx.Start(ctx, f.cfg); err != nil {
		return fmt.Errorf("failed to start NGINX: %w", err)
	}
	ui.Success("NGINX Gateway started")

	// Step 4: Start Bind9 DNS
	ui.Step("Starting Bind9 DNS server...")
	if err := bind9.Start(ctx, f.cfg); err != nil {
		return fmt.Errorf("failed to start Bind9: %w", err)
	}
	ui.Success("Bind9 DNS server started on port %d", f.cfg.DNS.Bind9Port)

	// Steps 5-6: Optional services for tier 3
	if tier >= 3 {
		// Step 5: Start NFS Server
		ui.Step("Starting NFS server...")
		if err := nfs.Start(ctx, f.cfg); err != nil {
			return fmt.Errorf("failed to start NFS: %w", err)
		}
		ui.Success("NFS server started")

		// Step 6: Start Registry (needs Bind9 for registry.local resolution)
		ui.Step("Starting local registry...")
		if err := registry.Start(ctx, f.cfg); err != nil {
			return fmt.Errorf("failed to start registry: %w", err)
		}
		ui.Success("Registry started")
	}

	ui.Success("Foundation stack ready")
	return nil
}

// Stop stops host services in reverse order (LIFO).
// This preserves the nova network and minikube for faster restarts.
//
// Stop order: Registry → NFS → Bind9 → NGINX
// Minikube and network are NOT stopped (must be stopped separately).
func (f *Foundation) Stop(ctx context.Context) error {
	ui.Header("Stopping Foundation Stack")

	errors := []error{}

	// Stop Registry
	ui.Step("Stopping Registry...")
	if err := registry.Stop(ctx); err != nil {
		ui.Warn("Failed to stop Registry: %v", err)
		errors = append(errors, err)
	} else {
		ui.Success("Registry stopped")
	}

	// Stop NFS
	ui.Step("Stopping NFS...")
	if err := nfs.Stop(ctx); err != nil {
		ui.Warn("Failed to stop NFS: %v", err)
		errors = append(errors, err)
	} else {
		ui.Success("NFS stopped")
	}

	// Stop Bind9
	ui.Step("Stopping Bind9...")
	if err := bind9.Stop(ctx); err != nil {
		ui.Warn("Failed to stop Bind9: %v", err)
		errors = append(errors, err)
	} else {
		ui.Success("Bind9 stopped")
	}

	// Stop NGINX
	ui.Step("Stopping NGINX...")
	if err := nginx.Stop(ctx); err != nil {
		ui.Warn("Failed to stop NGINX: %v", err)
		errors = append(errors, err)
	} else {
		ui.Success("NGINX stopped")
	}

	// Note: Minikube and network are not stopped here - they persist for faster restarts

	if len(errors) > 0 {
		return fmt.Errorf("some services failed to stop: %v", errors)
	}

	ui.Success("Foundation stack stopped")
	return nil
}

// Delete removes host services and the nova network.
// This is a destructive operation that removes all state.
//
// Delete order: Registry → NFS → Bind9 → NGINX → Nova Network
// Note: Minikube must be deleted separately before calling this.
func (f *Foundation) Delete(ctx context.Context) error {
	ui.Header("Deleting Foundation Stack")

	// Delete services (best effort, ignore errors)
	ui.Step("Removing services...")
	registry.Delete(ctx)
	nfs.Delete(ctx)
	bind9.Delete(ctx)
	nginx.Delete(ctx)
	ui.Success("Services removed")

	// Remove nova network
	ui.Step("Removing nova network...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	if err := dockerClient.RemoveNetwork(ctx, "nova"); err != nil {
		ui.Warn("Failed to remove nova network: %v", err)
		// Don't return error, network might not exist
	} else {
		ui.Success("Nova network removed")
	}

	ui.Success("Foundation stack deleted")
	return nil
}
