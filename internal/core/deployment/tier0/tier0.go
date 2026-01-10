// Package tier0 handles the deployment of NOVA Tier 0 (Minikube cluster).
package tier0

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// DeployTier0 deploys tier 0: Post-configuration of the Minikube cluster.
// Note: The cluster itself is already started by the Foundation Stack.
func DeployTier0(ctx context.Context, cfg *config.Config) error {
	ui.Header("Tier 0: Cluster Configuration")

	// Minikube cluster is already started by Foundation Stack
	ui.Info("Configuring cluster (already started by Foundation Stack)...")

	// Configure DNS mapping for registry.local -> host gateway IP
	ui.Step("Configuring registry DNS on nodes...")
	if err := minikube.ConfigureRegistryDNS(ctx, cfg); err != nil {
		ui.Warn("Failed to configure registry DNS: %v", err)
		ui.Info("Registry access from nodes may fail")
	}

	// Install mkcert CA certificate on all nodes for registry TLS
	ui.Step("Installing TLS certificates on nodes...")
	if err := minikube.InstallRegistryCA(ctx, cfg); err != nil {
		ui.Warn("Failed to install CA certificate: %v", err)
		ui.Info("Registry TLS verification may fail")
	}

	// Install NFS client on all nodes for persistent storage
	ui.Step("Installing NFS client on nodes...")
	if err := minikube.InstallNFSClient(ctx, cfg); err != nil {
		ui.Warn("Failed to install NFS client: %v", err)
		ui.Info("NFS-based storage may not work")
	}

	// Rename nova context to cluster-admin for consistency
	// With --profile=nova, minikube creates a context named "nova" instead of "minikube"
	if k8s.ContextExists(ctx, "nova") && !k8s.ContextExists(ctx, "cluster-admin") {
		if err := k8s.RenameContext(ctx, "nova", "cluster-admin"); err != nil {
			ui.Warn("Failed to rename context 'nova' to 'cluster-admin': %v", err)
		} else {
			ui.Info("Renamed kubectl context 'nova' to 'cluster-admin'")
		}
	} else if k8s.ContextExists(ctx, "cluster-admin") {
		ui.Debug("Context 'cluster-admin' already exists, skipping rename")
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
	if err := configureNodeLabelsAndTaints(ctx, cfg, allNodes); err != nil {
		return fmt.Errorf("failed to configure node labels and taints: %w", err)
	}

	// Step 4.5: Elect llm-d node (must happen after node labeling for warmup to work)
	ui.Step("Electing llm-d node...")
	electedNode, err := minikube.ElectLLMDNode(ctx, cfg)
	if err != nil {
		ui.Warn("Failed to elect llm-d node: %v", err)
		ui.Warn("Image warmup may not be able to pre-distribute to target node")
		// Continue despite election failure
	} else {
		ui.Success("Node elected for llm-d: %s", electedNode)
	}

	// Step 5: Add Helm repositories for cluster basics
	ui.Step("Adding Helm repositories...")
	repos := map[string]string{
		"cilium": constants.HelmRepoCilium,
	}
	if err := shared.AddHelmRepositories(ctx, repos); err != nil {
		return fmt.Errorf("failed to add Helm repositories: %w", err)
	}
	ui.Success("Helm repositories added")

	// Step 6: Deploy Cilium CNI (fundamental networking)
	ui.Step("Deploying Cilium CNI...")
	if err := deployCilium(ctx, cfg); err != nil {
		return fmt.Errorf("failed to deploy Cilium: %w", err)
	}
	ui.Success("Cilium CNI deployed")

	// Step 7: Configure CoreDNS (fundamental DNS)
	ui.Step("Configuring CoreDNS...")
	if err := deployCoreDNS(ctx, cfg); err != nil {
		return fmt.Errorf("failed to configure CoreDNS: %w", err)
	}
	ui.Success("CoreDNS configured")

	// Step 8: Deploy Local Path Storage (fundamental storage)
	ui.Step("Deploying Local Path Storage...")
	if err := deployLocalPathStorage(ctx, cfg); err != nil {
		return fmt.Errorf("failed to deploy storage: %w", err)
	}
	ui.Success("Local Path Storage deployed")

	ui.Header("Tier 0 Deployment Complete")
	ui.Info("Cluster ready with networking, DNS, and storage")
	ui.Info("Run 'nova kubectl get nodes' to verify cluster status")

	return nil
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// classifyNodes separates nodes into master and worker nodes.
func classifyNodes(ctx context.Context, allNodes []string) (string, []string) {
	var masterNode string
	var workerNodes []string

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

	return masterNode, workerNodes
}

// configureNodeLabelsAndTaints configures node labels and taints based on GPU/CPU mode.
func configureNodeLabelsAndTaints(ctx context.Context, cfg *config.Config, allNodes []string) error {
	masterNode, workerNodes := classifyNodes(ctx, allNodes)

	if cfg.IsGPUMode() {
		return configureGPUMode(ctx, masterNode, workerNodes)
	}
	return configureCPUMode(ctx, masterNode, workerNodes)
}

// configureGPUMode configures nodes for GPU mode.
func configureGPUMode(ctx context.Context, masterNode string, workerNodes []string) error {
	if len(workerNodes) > 0 {
		return configureMultiNodeGPU(ctx, workerNodes)
	}
	return configureSingleNodeGPU(ctx, masterNode)
}

// configureCPUMode configures nodes for CPU mode.
func configureCPUMode(ctx context.Context, masterNode string, workerNodes []string) error {
	if len(workerNodes) > 0 {
		return configureMultiNodeCPU(ctx, workerNodes)
	}
	return configureSingleNodeCPU(ctx, masterNode)
}

// configureMultiNodeGPU configures a multi-node cluster in GPU mode.
func configureMultiNodeGPU(ctx context.Context, workerNodes []string) error {
	ui.Info("GPU mode: multi-node cluster")

	// Disable GPU operands on all nodes first
	if err := k8s.LabelAllNodes(ctx, constants.LabelGPUOperands+"=false"); err != nil {
		ui.Warn("Failed to disable GPU operands cluster-wide: %v", err)
	}

	// First worker is GPU node
	gpuNode := workerNodes[0]
	if err := k8s.LabelNode(ctx, gpuNode, constants.LabelNodeTypeGPU+"=gpu-nvidia", false); err != nil {
		return fmt.Errorf("failed to label GPU node: %w", err)
	}
	ui.Info("Labeled %s as GPU node", gpuNode)

	if err := k8s.LabelNode(ctx, gpuNode, constants.LabelGPUOperands+"=true", false); err != nil {
		ui.Warn("Failed to enable GPU operands: %v", err)
	}

	// Remaining workers are CPU nodes
	for i := 1; i < len(workerNodes); i++ {
		if err := k8s.LabelNode(ctx, workerNodes[i], constants.LabelNodeTypeGPU+"=cpu", false); err != nil {
			ui.Warn("Failed to label CPU node %s: %v", workerNodes[i], err)
		} else {
			ui.Info("Labeled %s as CPU node", workerNodes[i])
		}
	}

	ui.Success("GPU mode configured")
	return nil
}

// configureSingleNodeGPU configures a single-node cluster in GPU mode.
func configureSingleNodeGPU(ctx context.Context, masterNode string) error {
	ui.Info("GPU mode: single-node cluster")

	if masterNode == "" {
		return fmt.Errorf("no master node found")
	}

	if err := k8s.LabelNode(ctx, masterNode, constants.LabelNodeTypeGPU+"=gpu-nvidia", false); err != nil {
		return fmt.Errorf("failed to label GPU node: %w", err)
	}
	ui.Info("Labeled %s as GPU node", masterNode)

	if err := k8s.LabelNode(ctx, masterNode, constants.LabelGPUOperands+"=true", false); err != nil {
		ui.Warn("Failed to enable GPU operands: %v", err)
	}

	if err := removeMasterTaints(ctx, masterNode); err != nil {
		return err
	}

	ui.Success("GPU mode configured")
	return nil
}

// configureMultiNodeCPU configures a multi-node cluster in CPU mode.
func configureMultiNodeCPU(ctx context.Context, workerNodes []string) error {
	ui.Info("CPU mode: multi-node cluster")

	for _, worker := range workerNodes {
		if err := k8s.LabelNode(ctx, worker, constants.LabelNodeTypeGPU+"=cpu", false); err != nil {
			ui.Warn("Failed to label CPU node %s: %v", worker, err)
		} else {
			ui.Info("Labeled %s as CPU node", worker)
		}
	}

	ui.Success("CPU mode configured")
	return nil
}

// configureSingleNodeCPU configures a single-node cluster in CPU mode.
func configureSingleNodeCPU(ctx context.Context, masterNode string) error {
	ui.Info("CPU mode: single-node cluster")

	if masterNode == "" {
		return fmt.Errorf("no master node found")
	}

	if err := k8s.LabelNode(ctx, masterNode, constants.LabelNodeTypeGPU+"=cpu", false); err != nil {
		return fmt.Errorf("failed to label CPU node: %w", err)
	}
	ui.Info("Labeled %s as CPU node", masterNode)

	if err := removeMasterTaints(ctx, masterNode); err != nil {
		return err
	}

	ui.Success("CPU mode configured")
	return nil
}

// removeMasterTaints removes taints from the master node to allow workload scheduling.
func removeMasterTaints(ctx context.Context, masterNode string) error {
	if err := k8s.RemoveTaint(ctx, masterNode, constants.LabelControlPlane); err != nil {
		ui.Warn("Failed to remove control-plane taint: %v", err)
	} else {
		ui.Info("Removed control-plane taint from %s", masterNode)
	}

	// Try removing legacy master taint (older K8s versions)
	if err := k8s.RemoveTaint(ctx, masterNode, constants.LabelMaster); err != nil {
		ui.Debug("Master taint removal from %s (expected on newer K8s)", masterNode)
	}

	return nil
}

// deployCilium deploys Cilium CNI with runtime configuration for Minikube.
func deployCilium(ctx context.Context, cfg *config.Config) error {
	// Label namespace
	if err := k8s.LabelNamespace(ctx, "kube-system", "service-type", "nova"); err != nil {
		return fmt.Errorf("failed to label kube-system namespace for routing: %w", err)
	}

	// Get minikube IP for Cilium configuration
	ui.Info("Getting Minikube control plane IP...")
	minikubeIP, err := minikube.GetIP(ctx)
	if err != nil {
		return fmt.Errorf("failed to get minikube IP: %w", err)
	}

	// Get API server port from kubectl
	ui.Info("Getting API server port...")
	apiPort, err := minikube.GetAPIServerPort(ctx)
	if err != nil {
		return fmt.Errorf("failed to get API server port: %w", err)
	}

	// Create dynamic overrides for runtime configuration
	// Note: We MUST set k8sServiceHost and k8sServicePort for Cilium to work properly in Minikube
	ui.Info("Installing Cilium CNI with API server %s:%s (may take a few minutes)...", minikubeIP, apiPort)

	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName: "cilium",
		ChartRef:    cfg.Versions.Tier1.Cilium.ChartRef(),
		Version:     cfg.Versions.Tier1.Cilium.GetVersion(),
		Namespace:   "kube-system",
		ValuesPath:  "resources/core/deployment/tier1/cilium/values.yaml",
		Values: map[string]any{
			"k8sServiceHost": minikubeIP,
			"k8sServicePort": apiPort,
		},
		Wait:            true,
		TimeoutSeconds:  600,
		CreateNamespace: true,
	})
}

// deployLocalPathStorage deploys local-path-provisioner for dynamic PVC provisioning.
func deployLocalPathStorage(ctx context.Context, cfg *config.Config) error {
	// Apply local-path-provisioner (includes namespace creation)
	ui.Info("Installing Local Path Provisioner...")
	if err := k8s.ApplyURL(ctx, cfg.GetLocalPathProvisionerManifestURL()); err != nil {
		return fmt.Errorf("failed to deploy local-path-provisioner: %w", err)
	}

	// Apply additional standard storage class
	ui.Info("Applying standard storage class...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/local-path-provisioner/storageclasses/standard.yaml"); err != nil {
		return fmt.Errorf("failed to apply standard storage class: %w", err)
	}

	// Apply NFS models storage class (for tier 3 LLM model storage)
	ui.Info("Applying NFS models storage class...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/nfs/sc-nfs-models.yaml"); err != nil {
		return fmt.Errorf("failed to apply NFS models storage class: %w", err)
	}

	// Patch configmap for custom storage directory
	ui.Info("Patching storage directory configuration...")
	if err := k8s.PatchConfigMap(ctx, constants.NamespaceLocalPathStorage, "local-path-config", "resources/core/deployment/tier1/local-path-provisioner/patches/storage-dir.yaml"); err != nil {
		return fmt.Errorf("failed to patch local-path-config: %w", err)
	}

	return nil
}

// deployCoreDNS configures CoreDNS with domain rewrites for NOVA services.
func deployCoreDNS(ctx context.Context, cfg *config.Config) error {
	data := map[string]any{
		"AuthDomain": cfg.DNS.AuthDomain,
		"Domain":     cfg.DNS.Domain,
	}

	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier1/coredns/configmaps/config-dns-rewrite.yaml", data); err != nil {
		return fmt.Errorf("failed to apply coredns rewrite config: %w", err)
	}

	// Restart CoreDNS to pick up changes immediately
	ui.Info("Restarting CoreDNS to apply configuration...")
	if err := k8s.RestartDeployment(ctx, "kube-system", "coredns"); err != nil {
		return fmt.Errorf("failed to restart coredns: %w", err)
	}

	// Wait for CoreDNS to be fully ready before proceeding
	// This prevents DNS resolution failures in subsequent components (e.g., Falco)
	ui.Info("Waiting for CoreDNS to be ready...")
	if err := k8s.WaitForDeploymentReady(ctx, "kube-system", "coredns", 120); err != nil {
		return fmt.Errorf("coredns did not become ready: %w", err)
	}

	return nil
}
