// Package tier0 handles the deployment of NOVA Tier 0 (Minikube cluster).
package tier0

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
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

	// Rename minikube context to cluster-admin for consistency
	if k8s.ContextExists(ctx, "minikube") {
		if err := k8s.RenameContext(ctx, "minikube", "cluster-admin"); err != nil {
			ui.Warn("Failed to rename context 'minikube' to 'cluster-admin': %v", err)
		} else {
			ui.Info("Renamed kubectl context 'minikube' to 'cluster-admin'")
		}
	}

	// Step 2: Mount BPF filesystem on all nodes
	ui.Step("Mounting BPF filesystem on nodes...")
	nodes, err := minikube.GetNodeNames(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to get node names: %w", err)
	}

	clusterMode := "multi-node"
	if cfg.Minikube.Nodes == 1 {
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

	// Step 4: Configure node types and GPU operator
	ui.Step("Configuring node types and GPU operator...")

	// Get list of worker nodes
	var workerNodes []string
	var masterNode string
	for _, node := range allNodes {
		isControlPlane, err := k8s.IsNodeControlPlane(ctx, node)
		if err != nil {
			continue
		}
		if isControlPlane {
			masterNode = node
		} else {
			workerNodes = append(workerNodes, node)
		}
	}

	if cfg.IsGPUMode() {
		// GPU MODE
		if len(workerNodes) > 0 {
			// Multi-node GPU cluster: elect one GPU worker, rest are CPU
			ui.Info("GPU mode: multi-node cluster")

			// Disable GPU operands on all nodes first
			if err := k8s.LabelAllNodes(ctx, "nvidia.com/gpu.deploy.operands=false"); err != nil {
				ui.Warn("Failed to disable GPU operands cluster-wide: %v", err)
			}

			// Elect first worker as GPU node
			gpuNode := workerNodes[0]
			if err := k8s.LabelNode(ctx, gpuNode, "nova.local/node-type=gpu-nvidia", false); err != nil {
				ui.Warn("Failed to label %s as GPU node: %v", gpuNode, err)
			} else {
				ui.Info("Labeled %s as GPU node", gpuNode)
			}

			// Enable GPU operands on GPU node
			if err := k8s.LabelNode(ctx, gpuNode, "nvidia.com/gpu.deploy.operands=true", false); err != nil {
				ui.Warn("Failed to enable GPU operands on %s: %v", gpuNode, err)
			}

			// Label remaining workers as CPU
			for i := 1; i < len(workerNodes); i++ {
				if err := k8s.LabelNode(ctx, workerNodes[i], "nova.local/node-type=cpu", false); err != nil {
					ui.Warn("Failed to label %s as CPU node: %v", workerNodes[i], err)
				} else {
					ui.Info("Labeled %s as CPU node", workerNodes[i])
				}
			}
		} else {
			// Single-node GPU cluster: use master as GPU, remove taint
			ui.Info("GPU mode: single-node cluster")

			if masterNode != "" {
				// Label master as GPU
				if err := k8s.LabelNode(ctx, masterNode, "nova.local/node-type=gpu-nvidia", false); err != nil {
					ui.Warn("Failed to label %s as GPU node: %v", masterNode, err)
				} else {
					ui.Info("Labeled %s as GPU node", masterNode)
				}

				// Enable GPU operands on master
				if err := k8s.LabelNode(ctx, masterNode, "nvidia.com/gpu.deploy.operands=true", false); err != nil {
					ui.Warn("Failed to enable GPU operands on %s: %v", masterNode, err)
				}

				// Remove taint from master to allow workloads
				if err := k8s.RemoveTaint(ctx, masterNode, "node-role.kubernetes.io/control-plane"); err != nil {
					ui.Warn("Failed to remove control-plane taint from %s: %v", masterNode, err)
				} else {
					ui.Info("Removed control-plane taint from %s", masterNode)
				}
				if err := k8s.RemoveTaint(ctx, masterNode, "node-role.kubernetes.io/master"); err != nil {
					// This might fail on newer K8s versions, which is expected
					ui.Debug("Master taint removal from %s (expected on newer K8s)", masterNode)
				}
			}
		}
		ui.Success("GPU mode configured")
	} else {
		// CPU MODE
		if len(workerNodes) > 0 {
			// Multi-node CPU cluster: label all workers as CPU
			ui.Info("CPU mode: multi-node cluster")

			for _, worker := range workerNodes {
				if err := k8s.LabelNode(ctx, worker, "nova.local/node-type=cpu", false); err != nil {
					ui.Warn("Failed to label %s as CPU node: %v", worker, err)
				} else {
					ui.Info("Labeled %s as CPU node", worker)
				}
			}
		} else {
			// Single-node CPU cluster: label master as CPU, remove taint
			ui.Info("CPU mode: single-node cluster")

			if masterNode != "" {
				// Label master as CPU
				if err := k8s.LabelNode(ctx, masterNode, "nova.local/node-type=cpu", false); err != nil {
					ui.Warn("Failed to label %s as CPU node: %v", masterNode, err)
				} else {
					ui.Info("Labeled %s as CPU node", masterNode)
				}

				// Remove taint from master to allow workloads
				if err := k8s.RemoveTaint(ctx, masterNode, "node-role.kubernetes.io/control-plane"); err != nil {
					ui.Warn("Failed to remove control-plane taint from %s: %v", masterNode, err)
				} else {
					ui.Info("Removed control-plane taint from %s", masterNode)
				}
				if err := k8s.RemoveTaint(ctx, masterNode, "node-role.kubernetes.io/master"); err != nil {
					// This might fail on newer K8s versions, which is expected
					ui.Debug("Master taint removal from %s (expected on newer K8s)", masterNode)
				}
			}
		}
		ui.Success("CPU mode configured")
	}

	ui.Header("Tier 0 Deployment Complete")
	ui.Info("Minikube cluster is ready")
	ui.Info("Run 'nova kubectl get nodes' to verify cluster status")

	return nil
}

// plural returns "s" for counts != 1, empty string for count == 1.
func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
