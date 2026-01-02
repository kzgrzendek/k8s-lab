// Package minikube provides a centralized wrapper for Minikube CLI operations.
//
// Unlike Docker and Helm, Minikube doesn't have an official Go SDK, so this
// package centralizes CLI command execution. All minikube operations in NOVA
// should go through this package to maintain consistency and ease future refactoring.
package minikube

import (
	"context"
	"fmt"
	"strings"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/exec"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// StartCluster starts a Minikube cluster with the given configuration.
func StartCluster(ctx context.Context, cfg *config.Config) error {
	// Build minikube start command
	args := []string{
		"start",
		"--install-addons=false",
		"--driver", cfg.Minikube.Driver,
		"--cpus", fmt.Sprintf("%d", cfg.Minikube.CPUs),
		"--memory", fmt.Sprintf("%d", cfg.Minikube.Memory),
		"--container-runtime", "docker",
		"--kubernetes-version", cfg.Minikube.KubernetesVersion,
		"--network-plugin", "cni",
		"--cni", "false",
		"--nodes", fmt.Sprintf("%d", cfg.Minikube.Nodes),
		"--extra-config", "kubelet.node-ip=0.0.0.0",
		"--extra-config", "kube-proxy.skip-headers=true",
	}

	// Add GPU support if configured
	if cfg.Minikube.GPUs != "" {
		args = append(args, "--gpus", cfg.Minikube.GPUs)
	}

	// Use ephemeral output for minikube startup
	// Shows progress in real-time but clears when done - similar to Docker build
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", args...).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		// Keep error visible, don't clear on failure
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to start minikube cluster: %w", err)
	}

	return nil
}

// IsRunning checks if the Minikube cluster is running.
func IsRunning(ctx context.Context) (bool, error) {
	output, err := exec.OutputStdout(ctx, "minikube", "status", "--format", "{{.Host}}")
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

	if err := exec.New(ctx, "minikube", "stop").
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to stop minikube cluster: %w", err)
	}
	return nil
}

// Delete deletes the Minikube cluster.
func Delete(ctx context.Context) error {
	// Use ephemeral output for minikube delete
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", "delete").
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to delete minikube cluster: %w", err)
	}
	return nil
}

// GetNodeNames returns the names of all nodes in the cluster based on config.
func GetNodeNames(ctx context.Context, cfg *config.Config) ([]string, error) {
	// Minikube naming convention: minikube, minikube-m02, minikube-m03, ...
	var nodes []string
	for i := 1; i <= cfg.Minikube.Nodes; i++ {
		if i == 1 {
			nodes = append(nodes, "minikube")
		} else {
			nodes = append(nodes, fmt.Sprintf("minikube-m%02d", i))
		}
	}

	return nodes, nil
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
	if err := exec.Run(ctx, "minikube", "ssh", "-n", nodeName, "--",
		"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf"); err != nil {
		return fmt.Errorf("failed to mount bpffs on node %s: %w", nodeName, err)
	}
	return nil
}

// GetIP returns the IP address of the Minikube control plane node.
func GetIP(ctx context.Context) (string, error) {
	ip, err := exec.OutputStdout(ctx, "minikube", "ip")
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
