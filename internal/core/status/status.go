package status

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/deployment/tier2"
	"github.com/kzgrzendek/nova/internal/host/dns/bind9"
	"github.com/kzgrzendek/nova/internal/host/gateway/nginx"
	"github.com/kzgrzendek/nova/internal/tools/minikube"
)

// ComponentStatus represents the status of a single component.
type ComponentStatus struct {
	Name    string
	Status  string // "running", "stopped", "error", "unknown"
	Details string
	Healthy bool
}

// releaseCheck defines a Helm release to check.
type releaseCheck struct {
	name        string
	namespace   string
	displayName string // optional, defaults to name
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
	HasGPU bool   // Whether this node is labeled for GPU workloads
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
	// Tier2Credentials contains Keycloak/Grafana admin credentials if tier 2 is deployed
	Tier2Credentials *tier2.DeployResult
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
		status.Deployments = c.GetDeploymentsStatus(clusterStatus)
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
		"--context=cluster-admin",
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.conditions[-1].type,ROLES:.metadata.labels.node-role\\.kubernetes\\.io/control-plane,GPU:.metadata.labels.nvidia\\.com/gpu\\.count",
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
		// GPU count will be a number (e.g., "1") or "<none>" if not present
		if len(fields) >= 4 && fields[3] != "<none>" {
			node.HasGPU = true
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetHostServicesStatus checks host services status.
func (c *Checker) GetHostServicesStatus() *HostServicesStatus {
	status := &HostServicesStatus{}

	// Check both services concurrently
	type serviceResult struct {
		isBind9 bool
		running bool
	}

	resultChan := make(chan serviceResult, 2)

	// Check Bind9 concurrently
	go func() {
		resultChan <- serviceResult{
			isBind9: true,
			running: bind9.IsRunning(c.ctx),
		}
	}()

	// Check NGINX concurrently
	go func() {
		resultChan <- serviceResult{
			isBind9: false,
			running: nginx.IsRunning(c.ctx),
		}
	}()

	// Collect results
	for i := 0; i < 2; i++ {
		result := <-resultChan
		if result.isBind9 {
			status.Bind9 = ComponentStatus{
				Name:    "Bind9 DNS",
				Healthy: result.running,
				Details: fmt.Sprintf("Port: %d", c.cfg.DNS.Bind9Port),
			}
			if result.running {
				status.Bind9.Status = "running"
			} else {
				status.Bind9.Status = "stopped"
			}
		} else {
			status.NGINX = ComponentStatus{
				Name:    "NGINX Gateway",
				Healthy: result.running,
				Details: "HTTP:80, HTTPS:443",
			}
			if result.running {
				status.NGINX.Status = "running"
			} else {
				status.NGINX.Status = "stopped"
			}
		}
	}

	return status
}

// GetDeploymentsStatus checks Kubernetes deployments status.
func (c *Checker) GetDeploymentsStatus(clusterStatus *ClusterStatus) *DeploymentsStatus {
	status := &DeploymentsStatus{}

	// Check Tier 0 components (pass cluster status to avoid re-checking)
	status.Tier0Components = c.checkTier0Components(clusterStatus)

	// Check Tier 1 components
	status.Tier1Components = c.checkTier1Components()

	// Check Tier 2 components
	status.Tier2Components = c.checkTier2Components()

	// Get Tier 2 credentials if deployed
	if tier2.IsDeployed(c.ctx) {
		status.Tier2Credentials = tier2.GetCredentials(c.ctx)
	}

	// Tier 3 would be checked here when implemented
	status.Tier3Components = []ComponentStatus{}

	return status
}

// checkTier0Components checks Tier 0 (Minikube cluster) components.
func (c *Checker) checkTier0Components(clusterStatus *ClusterStatus) []ComponentStatus {
	components := []ComponentStatus{}

	// Use the passed cluster status (no need to re-check)
	if !clusterStatus.Running {
		components = append(components, ComponentStatus{
			Name:    "Minikube Cluster",
			Status:  "stopped",
			Healthy: false,
			Details: "Not running",
		})
		return components
	}

	// Cluster is running
	components = append(components, ComponentStatus{
		Name:    "Minikube Cluster",
		Status:  "running",
		Healthy: clusterStatus.Healthy,
		Details: fmt.Sprintf("%d nodes", len(clusterStatus.Nodes)),
	})

	// Run additional checks concurrently
	type checkResult struct {
		component *ComponentStatus
	}

	checksToRun := 0
	if c.cfg.HasGPU() {
		checksToRun = 2 // BPF + GPU
	} else {
		checksToRun = 1 // BPF only
	}

	resultChan := make(chan checkResult, checksToRun)

	// Check BPF filesystem concurrently
	go func() {
		if c.checkBPFMounted() {
			resultChan <- checkResult{
				component: &ComponentStatus{
					Name:    "BPF Filesystem",
					Status:  "mounted",
					Healthy: true,
					Details: "Required for Cilium CNI",
				},
			}
		} else {
			resultChan <- checkResult{component: nil}
		}
	}()

	// Check GPU labeling concurrently if GPU is enabled
	if c.cfg.HasGPU() {
		go func() {
			gpuLabeled := c.checkGPULabeling()
			resultChan <- checkResult{
				component: &ComponentStatus{
					Name:    "GPU Node Labeling",
					Status:  map[bool]string{true: "configured", false: "not configured"}[gpuLabeled],
					Healthy: gpuLabeled,
					Details: fmt.Sprintf("Mode: %s", c.cfg.Minikube.GPUs),
				},
			}
		}()
	}

	// Collect concurrent check results
	for i := 0; i < checksToRun; i++ {
		result := <-resultChan
		if result.component != nil {
			components = append(components, *result.component)
		}
	}

	return components
}

// checkBPFMounted checks if BPF filesystem is mounted on nodes.
func (c *Checker) checkBPFMounted() bool {
	// Check if bpf is mounted on the first node
	cmd := exec.CommandContext(c.ctx, "kubectl", "get", "nodes", "--context=cluster-admin", "-o", "jsonpath={.items[0].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	nodeName := strings.TrimSpace(string(output))
	if nodeName == "" {
		return false
	}

	// Check mount via minikube ssh
	checkCmd := exec.CommandContext(c.ctx, "minikube", "ssh", "-n", nodeName, "--", "mount | grep bpf")
	err = checkCmd.Run()
	return err == nil
}

// checkGPULabeling checks if GPU nodes are properly labeled.
func (c *Checker) checkGPULabeling() bool {
	// Check for nodes with GPU hardware detected by GPU operator
	cmd := exec.CommandContext(c.ctx, "kubectl", "get", "nodes", "--context=cluster-admin", "-l", "nvidia.com/gpu.count", "-o", "name")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// checkTier1Components checks Tier 1 infrastructure components.
func (c *Checker) checkTier1Components() []ComponentStatus {
	// Define all Helm releases to check
	releases := []releaseCheck{
		{name: "cilium", namespace: "kube-system"},
		{name: "local-path-storage", namespace: "local-path-storage"},
		{name: "falco", namespace: "falco"},
		{name: "cert-manager", namespace: "cert-manager"},
		{name: "trust-manager", namespace: "cert-manager"},
		{name: "eg", namespace: "envoy-gateway-system", displayName: "Envoy Gateway"},
		{name: "ai-gateway", namespace: "envoy-gateway-system", displayName: "Envoy AI Gateway"},
	}

	// Add GPU operator if GPU is enabled
	if c.cfg.HasGPU() {
		releases = append(releases, releaseCheck{name: "gpu-operator", namespace: "gpu-operator"})
	}

	// Check all releases concurrently
	return c.checkHelmReleasesConcurrent(releases)
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

// checkHelmReleasesConcurrent checks multiple Helm releases concurrently for faster status checks.
func (c *Checker) checkHelmReleasesConcurrent(releases []releaseCheck) []ComponentStatus {
	type result struct {
		status      *ComponentStatus
		displayName string
	}

	resultChan := make(chan result, len(releases))

	// Launch concurrent checks
	for _, rel := range releases {
		go func(name, namespace, displayName string) {
			status := c.checkHelmRelease(name, namespace)
			resultChan <- result{status: status, displayName: displayName}
		}(rel.name, rel.namespace, rel.displayName)
	}

	// Collect results
	components := []ComponentStatus{}
	for i := 0; i < len(releases); i++ {
		res := <-resultChan
		if res.status != nil {
			// Apply display name if provided
			if res.displayName != "" {
				res.status.Name = res.displayName
			}
			components = append(components, *res.status)
		}
	}

	return components
}

// checkTier2Components checks Tier 2 platform components.
func (c *Checker) checkTier2Components() []ComponentStatus {
	components := []ComponentStatus{}

	// Define Helm releases to check
	releases := []releaseCheck{
		{name: "kyverno", namespace: "kyverno"},
		{name: "vls", namespace: "victorialogs", displayName: "VictoriaLogs"},
		{name: "collector", namespace: "victorialogs", displayName: "VictoriaLogs Collector"},
		{name: "vmks", namespace: "victoriametrics", displayName: "VictoriaMetrics Stack"},
	}

	// Check Helm releases concurrently
	components = append(components, c.checkHelmReleasesConcurrent(releases)...)

	// Check for Keycloak (via CRD status, not Helm)
	if c.checkKeycloakDeployed() {
		components = append(components, ComponentStatus{
			Name:    "Keycloak",
			Status:  "deployed",
			Healthy: true,
			Details: "Namespace: keycloak",
		})
	}

	return components
}

// checkKeycloakDeployed checks if Keycloak is deployed.
func (c *Checker) checkKeycloakDeployed() bool {
	cmd := exec.CommandContext(c.ctx, "kubectl", "get", "keycloaks.k8s.keycloak.org", "keycloak", "-n", "keycloak", "--context=cluster-admin", "-o", "name")
	err := cmd.Run()
	return err == nil
}
