// Package tier0 handles the deployment of NOVA Tier 0 (Minikube cluster).
package tier0

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/kubectl"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// DeployTier0 deploys tier 0: Minikube cluster with post-configuration.
func DeployTier0(ctx context.Context, cfg *config.Config) error {
	ui.Header("Tier 0: Minikube Cluster")

	// Step 1: Start Minikube cluster
	ui.Step("Starting Minikube cluster...")
	ui.Info("Driver: %s", cfg.Minikube.Driver)
	ui.Info("Nodes: %d", cfg.Minikube.Nodes)
	ui.Info("CPUs/node: %d", cfg.Minikube.CPUs)
	ui.Info("RAM/node: %dMB", cfg.Minikube.Memory)
	ui.Info("Kubernetes: %s", cfg.Minikube.KubernetesVersion)
	ui.Info("GPUs: %s", cfg.Minikube.GPUs)

	if err := minikube.StartCluster(ctx, cfg); err != nil {
		return fmt.Errorf("failed to start cluster: %w", err)
	}
	ui.Success("Minikube cluster started")

	// Step 2: Mount BPF filesystem on all nodes
	ui.Step("Mounting BPF filesystem on nodes...")
	nodes, err := minikube.GetNodeNames(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to get node names: %w", err)
	}

	clusterMode := "multi-node"
	if cfg.IsSingleNode() {
		clusterMode = "single-node"
	}
	ui.Info("Cluster mode: %s (%d node%s)", clusterMode, cfg.Minikube.Nodes, plural(cfg.Minikube.Nodes))

	for _, node := range nodes {
		if err := minikube.MountBPFFS(ctx, node); err != nil {
			ui.Warn("Failed to mount BPF filesystem on %s: %v", node, err)
		} else {
			ui.Info("Mounted BPF filesystem on %s", node)
		}
	}
	ui.Success("BPF filesystem mounted")

	// Step 3: Taint control-plane nodes
	ui.Step("Tainting control-plane nodes...")
	allNodes, err := k8s.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	for _, node := range allNodes {
		isControlPlane, err := k8s.IsNodeControlPlane(ctx, node)
		if err != nil {
			ui.Warn("Failed to check if %s is control-plane: %v", node, err)
			continue
		}

		if isControlPlane {
			// Taint with control-plane role
			if err := k8s.TaintNode(ctx, node, "node-role.kubernetes.io/control-plane", "", "NoSchedule"); err != nil {
				ui.Warn("Failed to taint %s with control-plane: %v", node, err)
			}

			// Also try master taint (for older K8s versions)
			if err := k8s.TaintNode(ctx, node, "node-role.kubernetes.io/master", "", "NoSchedule"); err != nil {
				// This might fail on newer K8s versions, which is expected
				ui.Debug("Master taint not applied to %s (expected on newer K8s)", node)
			}

			ui.Info("Tainted control-plane node: %s", node)
		}
	}
	ui.Success("Control-plane nodes tainted")

	// Step 4: Configure GPU operator
	if cfg.Minikube.GPUs != "" && cfg.Minikube.GPUs != "none" {
		ui.Step("Configuring GPU operator...")

		// Disable GPU operands on all nodes
		if err := k8s.LabelAllNodes(ctx, "nvidia.com/gpu.deploy.operands=false"); err != nil {
			ui.Warn("Failed to disable GPU operands cluster-wide: %v", err)
		} else {
			ui.Info("Disabled GPU operands cluster-wide")
		}

		// Find a worker node (prefer non-control-plane)
		var targetNode string
		for _, node := range allNodes {
			isControlPlane, err := k8s.IsNodeControlPlane(ctx, node)
			if err != nil {
				continue
			}
			if !isControlPlane {
				targetNode = node
				break
			}
		}

		// If no worker node found, use first node
		if targetNode == "" && len(allNodes) > 0 {
			targetNode = allNodes[0]
		}

		// Enable GPU operands on target node
		if targetNode != "" {
			if err := k8s.LabelNode(ctx, targetNode, "nvidia.com/gpu.deploy.operands", true); err != nil {
				ui.Warn("Failed to enable GPU operands on %s: %v", targetNode, err)
			} else {
				ui.Info("Enabled GPU operands on node: %s", targetNode)
			}
		}

		ui.Success("GPU operator configured")
	}

	// Step 5: Create developer context
	ui.Step("Creating developer kubectl context...")
	if err := setupDeveloperContext(ctx); err != nil {
		ui.Warn("Failed to create developer context: %v", err)
	} else {
		ui.Success("Developer kubectl context created")
	}

	ui.Header("Tier 0 Deployment Complete")
	ui.Info("Minikube cluster is ready")
	ui.Info("Run 'nova kubectl get nodes' to verify cluster status")

	return nil
}

// setupDeveloperContext creates a developer namespace, RBAC, and kubectl context.
// This provides a restricted context for developers to work in the 'developer' namespace.
func setupDeveloperContext(ctx context.Context) error {
	const developerNamespace = "developer"
	const developerContextName = "nova-developer"
	const developerServiceAccount = "developer"

	// Check if context already exists
	if k8s.ContextExists(ctx, developerContextName) {
		ui.Info("Developer context already exists - skipping")
		return nil
	}

	// Apply namespace
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier0/developer-context/namespace.yaml"); err != nil {
		return fmt.Errorf("failed to create developer namespace: %w", err)
	}

	// Apply service account
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier0/developer-context/serviceaccount.yaml"); err != nil {
		return fmt.Errorf("failed to create developer service account: %w", err)
	}

	// Apply role
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier0/developer-context/role.yaml"); err != nil {
		return fmt.Errorf("failed to create developer role: %w", err)
	}

	// Apply role binding
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier0/developer-context/rolebinding.yaml"); err != nil {
		return fmt.Errorf("failed to create developer role binding: %w", err)
	}

	// Apply secret (for service account token)
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier0/developer-context/secret.yaml"); err != nil {
		return fmt.Errorf("failed to create developer token secret: %w", err)
	}

	// Wait for the token to be populated
	if err := k8s.WaitForSecret(ctx, developerNamespace, developerServiceAccount+"-token", 30); err != nil {
		return fmt.Errorf("failed waiting for developer token: %w", err)
	}

	// Create kubectl context
	if err := k8s.CreateKubectlContext(ctx, developerContextName, developerNamespace, developerServiceAccount); err != nil {
		return fmt.Errorf("failed to create kubectl context: %w", err)
	}

	return nil
}

// plural returns "s" for counts != 1, empty string for count == 1.
func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
