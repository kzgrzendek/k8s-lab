// Package k8s provides Kubernetes client operations.
package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// TaintNode adds a taint to a Kubernetes node.
func TaintNode(ctx context.Context, nodeName, key, value, effect string) error {
	taint := fmt.Sprintf("%s=%s:%s", key, value, effect)
	if value == "" {
		taint = fmt.Sprintf("%s:%s", key, effect)
	}

	cmd := exec.CommandContext(ctx, "kubectl", "taint", "node", nodeName, taint, "--overwrite")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to taint node %s: %w", nodeName, err)
	}
	return nil
}

// LabelNode adds or removes a label on a Kubernetes node.
func LabelNode(ctx context.Context, nodeName, label string, remove bool) error {
	args := []string{"label", "node", nodeName, label, "--overwrite"}

	// If remove is true, the label should end with '-'
	if remove && !strings.HasSuffix(label, "-") {
		args[3] = label + "-"
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to label node %s: %w", nodeName, err)
	}
	return nil
}

// LabelAllNodes applies a label to all nodes in the cluster.
func LabelAllNodes(ctx context.Context, label string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "label", "nodes", "--all", label, "--overwrite")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to label all nodes: %w", err)
	}
	return nil
}

// GetNodes returns a list of all node names.
func GetNodes(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	nodes := strings.Fields(string(output))
	return nodes, nil
}

// GetNodesByRole returns nodes with a specific role label.
func GetNodesByRole(ctx context.Context, role string) ([]string, error) {
	label := fmt.Sprintf("node-role.kubernetes.io/%s", role)
	cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "-l", label, "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by role %s: %w", role, err)
	}

	nodes := strings.Fields(string(output))
	return nodes, nil
}

// IsNodeControlPlane checks if a node is a control-plane node.
func IsNodeControlPlane(ctx context.Context, nodeName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "node", nodeName,
		"-o", "jsonpath={.metadata.labels.node-role\\.kubernetes\\.io/control-plane}")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check if node is control-plane: %w", err)
	}

	// If the label exists, the output will be non-empty
	return strings.TrimSpace(string(output)) != "", nil
}
