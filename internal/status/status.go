package status

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kzgrzendek/nova/internal/bind9"
	"github.com/kzgrzendek/nova/internal/config"
	"github.com/kzgrzendek/nova/internal/minikube"
	"github.com/kzgrzendek/nova/internal/nginx"
)

// ComponentStatus represents the status of a single component.
type ComponentStatus struct {
	Name    string
	Status  string // "running", "stopped", "error", "unknown"
	Details string
	Healthy bool
}

// SystemStatus represents the overall system status.
type SystemStatus struct {
	Cluster      *ClusterStatus
	HostServices *HostServicesStatus
	Deployments  *DeploymentsStatus
	Config       *config.Config
}

// ClusterStatus represents Minikube cluster status.
type ClusterStatus struct {
	Running     bool
	Nodes       []NodeStatus
	Version     string
	GPU         string
	Healthy     bool
	ErrorDetail string
}

// NodeStatus represents a single node's status.
type NodeStatus struct {
	Name   string
	Status string // "Ready", "NotReady", etc.
	Roles  string
}

// HostServicesStatus represents host services status.
type HostServicesStatus struct {
	Bind9 ComponentStatus
	NGINX ComponentStatus
}

// DeploymentsStatus represents Kubernetes deployments status.
type DeploymentsStatus struct {
	Tier0Components []ComponentStatus
	Tier1Components []ComponentStatus
	Tier2Components []ComponentStatus
	Tier3Components []ComponentStatus
}

// Checker checks the status of all components.
type Checker struct {
	ctx context.Context
	cfg *config.Config
}

// NewChecker creates a new status checker.
func NewChecker(ctx context.Context, cfg *config.Config) *Checker {
	return &Checker{
		ctx: ctx,
		cfg: cfg,
	}
}

// GetSystemStatus gets the complete system status.
func (c *Checker) GetSystemStatus() (*SystemStatus, error) {
	status := &SystemStatus{
		Config: c.cfg,
	}

	// Check cluster status
	clusterStatus, err := c.GetClusterStatus()
	if err != nil {
		// Don't fail completely if cluster check fails
		clusterStatus = &ClusterStatus{
			Running:     false,
			Healthy:     false,
			ErrorDetail: err.Error(),
		}
	}
	status.Cluster = clusterStatus

	// Check host services
	status.HostServices = c.GetHostServicesStatus()

	// Check deployments (only if cluster is running)
	if clusterStatus.Running {
		status.Deployments = c.GetDeploymentsStatus()
	}

	return status, nil
}

// GetClusterStatus checks Minikube cluster status.
func (c *Checker) GetClusterStatus() (*ClusterStatus, error) {
	status := &ClusterStatus{
		GPU: c.cfg.Minikube.GPUs,
	}

	// Check if cluster is running
	running, err := minikube.IsRunning(c.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check cluster status: %w", err)
	}
	status.Running = running

	if !running {
		return status, nil
	}

	// Get Minikube version
	cmd := exec.CommandContext(c.ctx, "minikube", "version", "--short")
	output, err := cmd.Output()
	if err == nil {
		status.Version = strings.TrimSpace(string(output))
	}

	// Get node statuses
	nodes, err := c.getNodeStatuses()
	if err != nil {
		status.ErrorDetail = fmt.Sprintf("failed to get nodes: %v", err)
		status.Healthy = false
		return status, nil
	}
	status.Nodes = nodes

	// Cluster is healthy if all nodes are Ready
	status.Healthy = true
	for _, node := range nodes {
		if !strings.Contains(node.Status, "Ready") {
			status.Healthy = false
			break
		}
	}

	return status, nil
}

// getNodeStatuses gets the status of all nodes in the cluster.
func (c *Checker) getNodeStatuses() ([]NodeStatus, error) {
	cmd := exec.CommandContext(c.ctx, "kubectl", "get", "nodes",
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,ROLES:.metadata.labels.node-role\\.kubernetes\\.io/control-plane",
		"--no-headers")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var nodes []NodeStatus
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		node := NodeStatus{
			Name:   fields[0],
			Status: fields[1],
		}
		if len(fields) >= 3 && fields[2] != "<none>" {
			node.Roles = "control-plane"
		} else {
			node.Roles = "worker"
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetHostServicesStatus checks host services status.
func (c *Checker) GetHostServicesStatus() *HostServicesStatus {
	status := &HostServicesStatus{}

	// Check Bind9
	bind9Running := bind9.IsRunning(c.ctx)
	status.Bind9 = ComponentStatus{
		Name:    "Bind9 DNS",
		Healthy: bind9Running,
		Details: fmt.Sprintf("Port: %d", c.cfg.DNS.Bind9Port),
	}
	if bind9Running {
		status.Bind9.Status = "running"
	} else {
		status.Bind9.Status = "stopped"
	}

	// Check NGINX
	nginxRunning := nginx.IsRunning(c.ctx)
	status.NGINX = ComponentStatus{
		Name:    "NGINX Gateway",
		Healthy: nginxRunning,
		Details: "HTTP:80, HTTPS:443",
	}
	if nginxRunning {
		status.NGINX.Status = "running"
	} else {
		status.NGINX.Status = "stopped"
	}

	return status
}

// GetDeploymentsStatus checks Kubernetes deployments status.
func (c *Checker) GetDeploymentsStatus() *DeploymentsStatus {
	status := &DeploymentsStatus{}

	// Tier 0 is the cluster itself (already checked)
	status.Tier0Components = []ComponentStatus{
		{
			Name:    "Minikube Cluster",
			Status:  "running",
			Healthy: true,
			Details: fmt.Sprintf("%d nodes", c.cfg.Minikube.Nodes),
		},
	}

	// Check Tier 1 components
	status.Tier1Components = c.checkTier1Components()

	// Tier 2 and 3 would be checked here when implemented
	// For now, return empty slices
	status.Tier2Components = []ComponentStatus{}
	status.Tier3Components = []ComponentStatus{}

	return status
}

// checkTier1Components checks Tier 1 infrastructure components.
func (c *Checker) checkTier1Components() []ComponentStatus {
	components := []ComponentStatus{}

	// Check for Cilium
	if status := c.checkHelmRelease("cilium", "kube-system"); status != nil {
		components = append(components, *status)
	}

	// Check for Local Path Storage
	if status := c.checkHelmRelease("local-path-storage", "local-path-storage"); status != nil {
		components = append(components, *status)
	}

	// Check for Falco
	if status := c.checkHelmRelease("falco", "falco"); status != nil {
		components = append(components, *status)
	}

	// Check for GPU Operator (only if GPU enabled)
	if c.cfg.HasGPU() {
		if status := c.checkHelmRelease("gpu-operator", "gpu-operator"); status != nil {
			components = append(components, *status)
		}
	}

	// Check for Cert Manager
	if status := c.checkHelmRelease("cert-manager", "cert-manager"); status != nil {
		components = append(components, *status)
	}

	// Check for Trust Manager
	if status := c.checkHelmRelease("trust-manager", "cert-manager"); status != nil {
		components = append(components, *status)
	}

	// Check for Envoy Gateway
	if status := c.checkHelmRelease("eg", "envoy-gateway-system"); status != nil {
		status.Name = "Envoy Gateway"
		components = append(components, *status)
	}

	// Check for Envoy AI Gateway
	if status := c.checkHelmRelease("ai-gateway", "envoy-gateway-system"); status != nil {
		status.Name = "Envoy AI Gateway"
		components = append(components, *status)
	}

	return components
}

// checkHelmRelease checks if a Helm release is deployed and healthy.
func (c *Checker) checkHelmRelease(name, namespace string) *ComponentStatus {
	cmd := exec.CommandContext(c.ctx, "helm", "status", name, "-n", namespace, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		// Release not found
		return nil
	}

	// Basic check: if helm status succeeds, consider it deployed
	status := &ComponentStatus{
		Name:    name,
		Status:  "deployed",
		Healthy: true,
		Details: fmt.Sprintf("Namespace: %s", namespace),
	}

	// Check if it contains "deployed" status
	if !strings.Contains(string(output), `"status":"deployed"`) {
		status.Status = "unknown"
		status.Healthy = false
	}

	return status
}
