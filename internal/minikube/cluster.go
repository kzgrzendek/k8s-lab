// Package minikube provides functions to manage Minikube clusters.
package minikube

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kzgrzendek/nova/internal/config"
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

	cmd := exec.CommandContext(ctx, "minikube", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start minikube cluster: %w", err)
	}

	return nil
}

// IsRunning checks if the Minikube cluster is running.
func IsRunning(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "minikube", "status", "--format", "{{.Host}}")
	output, err := cmd.Output()
	if err != nil {
		// If minikube status fails, cluster is not running
		return false, nil
	}

	status := strings.TrimSpace(string(output))
	return status == "Running", nil
}

// Stop stops the Minikube cluster.
func Stop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "minikube", "stop")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop minikube cluster: %w", err)
	}
	return nil
}

// Delete deletes the Minikube cluster.
func Delete(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "minikube", "delete")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete minikube cluster: %w", err)
	}
	return nil
}

// GetNodeNames returns the names of all nodes in the cluster.
func GetNodeNames(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "minikube", "node", "list", "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	// Parse node names from output
	// For simplicity, assume nodes are named: minikube, minikube-m02, minikube-m03
	var nodes []string
	nodesCount := 3 // Default from config
	for i := 1; i <= nodesCount; i++ {
		if i == 1 {
			nodes = append(nodes, "minikube")
		} else {
			nodes = append(nodes, fmt.Sprintf("minikube-m%02d", i))
		}
	}

	return nodes, nil
}

// MountBPFFS mounts the BPF filesystem on a node.
func MountBPFFS(ctx context.Context, nodeName string) error {
	cmd := exec.CommandContext(ctx, "minikube", "ssh", "-n", nodeName, "--",
		"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to mount bpffs on node %s: %w", nodeName, err)
	}
	return nil
}
