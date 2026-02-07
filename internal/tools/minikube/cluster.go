// Package minikube provides a centralized wrapper for Minikube CLI operations.
//
// Unlike Docker and Helm, Minikube doesn't have an official Go SDK, so this
// package centralizes CLI command execution. All minikube operations in NOVA
// should go through this package to maintain consistency and ease future refactoring.
package minikube

import (
	"context"
	"fmt"
	"os"
	execCmd "os/exec"
	"strings"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/exec"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// getEnglishLocale returns environment variables that force English locale.
// This ensures consistent output parsing regardless of the user's system locale.
func getEnglishLocale() []string {
	return []string{
		"LC_ALL=C",
		"LANG=C",
	}
}


// setEnglishLocale sets English locale environment variables on an exec.Cmd.
// This ensures minikube output is in English for consistent parsing.
func setEnglishLocale(cmd *execCmd.Cmd) {
	// Preserve existing environment and append locale settings
	cmd.Env = append(os.Environ(), getEnglishLocale()...)
}

// StartCluster starts the minikube cluster with the given configuration.
// Uses profile name "nova" for consistent naming across all components.
func StartCluster(ctx context.Context, cfg *config.Config) error {
	// Build minikube start command
	args := []string{
		"start",
		"--profile", "nova", // Consistent profile name
		"--install-addons=false",
		"--driver", cfg.Minikube.Driver,
		"--network", "nova", // Use nova network (created before cluster start)
		"--cpus", fmt.Sprintf("%d", cfg.GetCPUs()),
		"--memory", fmt.Sprintf("%d", cfg.GetMemory()),
		"--container-runtime", "docker",
		"--kubernetes-version", cfg.Minikube.KubernetesVersion,
		"--network-plugin", "cni",
		"--cni", "false",
		"--nodes", fmt.Sprintf("%d", cfg.GetNodes()),
		"--extra-config", "kubelet.node-ip=0.0.0.0",
		"--extra-config", "kube-proxy.skip-headers=true",
	}

	// Add GPU passthrough for NVIDIA mode only
	// Intel GPUs don't need minikube --gpus flag (they use device plugins)
	if cfg.IsNVIDIAMode() {
		args = append(args, "--gpus", "all")
		ui.Info("NVIDIA GPU mode enabled - passing --gpus=all to minikube")
	} else {
		ui.Debug("GPU mode: %s (not NVIDIA, skipping --gpus flag)", cfg.Minikube.GPUMode)
	}

	// Configure Docker daemon for optimized image pulls
	if cfg.Performance.MaxConcurrentDownloads > 0 {
		args = append(args, "--docker-opt",
			fmt.Sprintf("max-concurrent-downloads=%d", cfg.Performance.MaxConcurrentDownloads))
		ui.Debug("Configuring Docker daemon with max-concurrent-downloads=%d", cfg.Performance.MaxConcurrentDownloads)
	}

	// Use ephemeral output for minikube startup
	// Shows progress in real-time but clears when done - similar to Docker build
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", args...).
		WithEnv(getEnglishLocale()).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error visible, don't clear on failure
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to start minikube cluster: %w", err)
	}

	return nil
}

// IsRunning checks if the Minikube cluster is running.
func IsRunning(ctx context.Context) (bool, error) {
	output, err := exec.New(ctx, "minikube", "-p", "nova", "status", "--format", "{{.Host}}").
		WithEnv(getEnglishLocale()).
		OutputStdout()
	if err != nil {
		// If minikube status fails, cluster is not running
		return false, nil
	}

	return output == "Running", nil
}

// Stop stops the Minikube cluster.
func Stop(ctx context.Context) error {
	// Use ephemeral output for minikube stop
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", "-p", "nova", "stop").
		WithEnv(getEnglishLocale()).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to stop minikube cluster: %w", err)
	}
	return nil
}

// Delete deletes the Minikube cluster with full purge.
// The --purge flag ensures Docker volumes are deleted, preventing
// old etcd data (Helm releases, secrets) from persisting across deletes.
func Delete(ctx context.Context) error {
	// Use ephemeral output for minikube delete
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", "-p", "nova", "delete", "--purge").
		WithEnv(getEnglishLocale()).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to delete minikube cluster: %w", err)
	}
	return nil
}

// GetNodeNames returns the names of all nodes in the cluster using kubectl discovery.
// This automatically discovers node names regardless of minikube profile naming.
func GetNodeNames(ctx context.Context, cfg *config.Config) ([]string, error) {
	// Use kubectl to discover all node names dynamically
	output, err := exec.OutputStdout(ctx, "kubectl", "get", "nodes",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("failed to discover cluster nodes: %w", err)
	}

	// Split the space-separated node names
	nodeNames := strings.Fields(strings.TrimSpace(output))
	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("no nodes found in cluster")
	}

	return nodeNames, nil
}

// GetNodesByLabel returns the names of nodes matching a specific label selector.
// labelSelector format: "key=value" (e.g., "nova.local/node-type=gpu-nvidia")
func GetNodesByLabel(ctx context.Context, cfg *config.Config, labelSelector string) ([]string, error) {
	// Use kubectl to get nodes with the label selector
	output, err := exec.OutputStdout(ctx, "kubectl", "get", "nodes",
		"-l", labelSelector,
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes with label %s: %w", labelSelector, err)
	}

	// Parse output (space-separated node names)
	nodeNames := strings.Fields(output)

	// Convert K8s node names to minikube node names if needed
	// K8s might use different naming, but in minikube they should match
	return nodeNames, nil
}

// ElectLLMDNode elects a node for llm-d deployment and labels it with nova.local/llmd-node=true.
// Election strategy:
//   - Multi-node GPU mode: elect a GPU node (nova.local/node-type=gpu-nvidia or gpu-intel)
//   - Multi-node CPU mode: randomly elect a CPU worker node (nova.local/node-type=cpu)
//   - Single-node mode: use master node (remove NoSchedule taint if present)
//
// Returns the elected node name.
func ElectLLMDNode(ctx context.Context, cfg *config.Config) (string, error) {
	// Check if a node is already elected
	existingNodes, err := GetNodesByLabel(ctx, cfg, "nova.local/llmd-node=true")
	if err == nil && len(existingNodes) > 0 {
		ui.Info("Node %s already elected for llm-d", existingNodes[0])
		return existingNodes[0], nil
	}

	nodeCount := cfg.GetNodes()
	var electedNode string

	if nodeCount == 1 {
		// Single-node mode: use master node (discover dynamically)
		ui.Info("Single-node mode: electing master node for llm-d")

		// Get the control plane node name dynamically
		nodes, err := GetNodeNames(ctx, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to discover master node: %w", err)
		}
		if len(nodes) == 0 {
			return "", fmt.Errorf("no nodes found in cluster")
		}
		electedNode = nodes[0] // In single-node mode, the only node is the master

		// Remove NoSchedule taint from master if present
		ui.Debug("Removing NoSchedule taint from master node...")
		if err := k8s.RemoveTaint(ctx, electedNode, "node-role.kubernetes.io/control-plane"); err != nil {
			ui.Debug("Failed to remove control-plane taint: %v (may not exist)", err)
		}
	} else {
		// Multi-node mode: elect based on GPU mode
		if cfg.IsGPUMode() {
			// GPU mode: elect a GPU node based on GPU type
			gpuLabel := cfg.GetGPUMode().NodeLabel()
			ui.Info("Multi-node GPU mode: electing GPU node for llm-d (%s)", gpuLabel)
			gpuNodes, err := GetNodesByLabel(ctx, cfg, "nova.local/node-type="+gpuLabel)
			if err != nil {
				return "", fmt.Errorf("failed to get GPU nodes: %w", err)
			}
			if len(gpuNodes) == 0 {
				return "", fmt.Errorf("no GPU nodes found with label nova.local/node-type=%s", gpuLabel)
			}
			// Use first GPU node
			electedNode = gpuNodes[0]
		} else {
			// CPU mode: elect a CPU worker node
			ui.Info("Multi-node CPU mode: electing CPU worker node for llm-d")
			cpuNodes, err := GetNodesByLabel(ctx, cfg, "nova.local/node-type=cpu")
			if err != nil {
				return "", fmt.Errorf("failed to get CPU nodes: %w", err)
			}
			if len(cpuNodes) == 0 {
				return "", fmt.Errorf("no CPU worker nodes found with label nova.local/node-type=cpu")
			}
			// Use first CPU worker node (random selection could be implemented here)
			electedNode = cpuNodes[0]
		}
	}

	// Label the elected node
	ui.Info("Labeling node %s with nova.local/llmd-node=true", electedNode)
	if err := k8s.LabelNode(ctx, electedNode, "nova.local/llmd-node=true", false); err != nil {
		return "", fmt.Errorf("failed to label elected node: %w", err)
	}

	ui.Success("Node %s elected for llm-d deployment", electedNode)
	return electedNode, nil
}

// GetNodeCount returns the number of nodes from a running cluster.
func GetNodeCount(ctx context.Context) (int, error) {
	nodes, err := k8s.GetNodes(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get nodes: %w", err)
	}

	return len(nodes), nil
}

// MountBPFFS mounts the BPF filesystem on a node.
func MountBPFFS(ctx context.Context, nodeName string) error {
	if err := exec.New(ctx, "minikube", "-p", "nova", "ssh", "-n", nodeName, "--",
		"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf").
		WithEnv(getEnglishLocale()).
		Run(); err != nil {
		return fmt.Errorf("failed to mount bpffs on node %s: %w", nodeName, err)
	}
	return nil
}

// GetIP returns the IP address of the Minikube control plane node.
func GetIP(ctx context.Context) (string, error) {
	ip, err := exec.New(ctx, "minikube", "-p", "nova", "ip").
		WithEnv(getEnglishLocale()).
		OutputStdout()
	if err != nil {
		return "", fmt.Errorf("failed to get minikube IP: %w", err)
	}

	if ip == "" {
		return "", fmt.Errorf("minikube ip returned empty result")
	}

	return ip, nil
}

// GetAPIServerPort returns the API server port from kubectl cluster-info.
func GetAPIServerPort(ctx context.Context) (string, error) {
	// Use kubectl config view to get the API server URL, then extract the port
	// This avoids directly reading config files from the user's system
	server, err := exec.OutputStdout(ctx, "kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		return "", fmt.Errorf("failed to get API server URL: %w", err)
	}

	if server == "" {
		return "", fmt.Errorf("API server URL is empty")
	}

	// Extract port from URL (e.g., https://192.168.49.2:8443 -> 8443)
	parts := strings.Split(server, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid server URL format: %s", server)
	}

	port := parts[len(parts)-1]
	return port, nil
}

// GetVersion returns the installed Minikube version.
// Returns version string in format "v1.37.0" or similar.
func GetVersion(ctx context.Context) (string, error) {
	output, err := exec.New(ctx, "minikube", "-p", "nova", "version", "--short").
		WithEnv(getEnglishLocale()).
		OutputStdout()
	if err != nil {
		return "", fmt.Errorf("failed to get minikube version: %w", err)
	}

	version := strings.TrimSpace(output)
	if version == "" {
		return "", fmt.Errorf("minikube version returned empty result")
	}

	return version, nil
}

// DockerEnv contains Docker daemon connection information for minikube.
type DockerEnv struct {
	Host      string // e.g., "tcp://192.168.49.2:2376"
	CertPath  string // e.g., "/home/user/.minikube/certs"
	TLSVerify bool   // Whether to verify TLS
}

// GetDockerEnv retrieves the Docker daemon environment variables from minikube.
// This allows direct access to minikube's Docker daemon for pulling images.
// Retries with exponential backoff if minikube's Docker daemon isn't ready yet.
func GetDockerEnv(ctx context.Context) (*DockerEnv, error) {
	const maxRetries = 5
	const initialDelay = 2 // seconds

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := initialDelay * (1 << (attempt - 1)) // Exponential backoff: 2s, 4s, 8s, 16s
			ui.Debug("Retrying docker-env in %ds (attempt %d/%d)...", delay, attempt+1, maxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(delay) * time.Second):
			}
		}

		// Get docker-env output from minikube
		output, err := exec.New(ctx, "minikube", "-p", "nova", "docker-env", "--shell", "bash").
			WithEnv(getEnglishLocale()).
			OutputStdout()
		if err != nil {
			lastErr = err
			ui.Debug("Failed to get docker-env (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}

		env := &DockerEnv{}

		// Parse the output which looks like:
		// export DOCKER_TLS_VERIFY="1"
		// export DOCKER_HOST="tcp://192.168.49.2:2376"
		// export DOCKER_CERT_PATH="/home/user/.minikube/certs"
		// export MINIKUBE_ACTIVE_DOCKERD="minikube"

		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "export ") {
				continue
			}

			// Remove "export " prefix
			line = strings.TrimPrefix(line, "export ")

			// Split by '='
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := parts[0]
			value := strings.Trim(parts[1], "\"")

			switch key {
			case "DOCKER_HOST":
				env.Host = value
			case "DOCKER_CERT_PATH":
				env.CertPath = value
			case "DOCKER_TLS_VERIFY":
				env.TLSVerify = value == "1"
			}
		}

		if env.Host == "" {
			lastErr = fmt.Errorf("failed to parse DOCKER_HOST from minikube docker-env output")
			ui.Debug("Failed to parse docker-env (attempt %d/%d): %v", attempt+1, maxRetries, lastErr)
			continue
		}

		// Success!
		return env, nil
	}

	return nil, fmt.Errorf("failed to get minikube docker-env after %d attempts: %w", maxRetries, lastErr)
}
