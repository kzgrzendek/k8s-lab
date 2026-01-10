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
	"github.com/kzgrzendek/nova/internal/core/constants"
	pki "github.com/kzgrzendek/nova/internal/setup/certificates"
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

	// Add GPU support only in GPU mode (not in forced CPU mode)
	if cfg.IsGPUMode() && cfg.Minikube.GPUs != "" {
		args = append(args, "--gpus", cfg.Minikube.GPUs)
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

// Delete deletes the Minikube cluster.
func Delete(ctx context.Context) error {
	// Use ephemeral output for minikube delete
	ephemeralWriter := ui.PipeWriter()
	defer ephemeralWriter.Done()

	if err := exec.New(ctx, "minikube", "-p", "nova", "delete").
		WithEnv(getEnglishLocale()).
		RunWithEphemeralOutput(ephemeralWriter); err != nil {
		ephemeralWriter.KeepOnDone()
		return fmt.Errorf("failed to delete minikube cluster: %w", err)
	}
	return nil
}

// ConfigureRegistryDNS adds a /etc/hosts entry on all minikube nodes to resolve
// the registry domain to the host's gateway IP. This allows nodes to access the
// registry running on the host machine.
func ConfigureRegistryDNS(ctx context.Context, cfg *config.Config) error {
	// Verify minikube is running
	running, err := IsRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check minikube status: %w", err)
	}
	if !running {
		return fmt.Errorf("minikube is not running")
	}

	// Get all node names
	nodes, err := GetNodeNames(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to get node names: %w", err)
	}

	ui.Debug("Configuring registry DNS on minikube nodes...")

	// Track success/failure for each node
	successCount := 0
	var lastError error

	for _, node := range nodes {
		ui.Debug("Configuring DNS on node %s...", node)

		// Get host gateway IP from inside the node
		// The gateway is the host machine's IP as seen from the container
		gatewayCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--",
			"ip", "route", "show", "default", "|", "awk", "'/default/ {print $3}'")
		setEnglishLocale(gatewayCmd)
		gatewayOutput, err := gatewayCmd.CombinedOutput()
		if err != nil {
			lastError = fmt.Errorf("failed to get gateway IP on node %s: %w", node, err)
			ui.Error("✗ Failed to get gateway IP on node %s: %v", node, err)
			continue
		}

		gatewayIP := strings.TrimSpace(string(gatewayOutput))
		if gatewayIP == "" {
			lastError = fmt.Errorf("empty gateway IP on node %s", node)
			ui.Error("✗ Empty gateway IP on node %s", node)
			continue
		}

		ui.Debug("Gateway IP on node %s: %s", node, gatewayIP)

		// Add registry.local to /etc/hosts
		// First check if entry already exists to avoid duplicates
		checkCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--",
			"grep", "-q", constants.RegistryDomain, "/etc/hosts")
		setEnglishLocale(checkCmd)
		entryExists := checkCmd.Run() == nil

		if !entryExists {
			// Add the hosts entry using echo piped to tee within the ssh session
			// Note: Can't use stdin with tee through minikube ssh - it doesn't forward stdin properly
			// Solution: Use the pipe within the ssh command string itself
			hostsEntry := fmt.Sprintf("%s %s", gatewayIP, constants.RegistryDomain)
			cmdString := fmt.Sprintf("echo '%s' | sudo tee -a /etc/hosts > /dev/null", hostsEntry)
			addHostsCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--", cmdString)
			setEnglishLocale(addHostsCmd)

			if output, err := addHostsCmd.CombinedOutput(); err != nil {
				lastError = fmt.Errorf("failed to add hosts entry on node %s: %w", node, err)
				ui.Error("✗ Failed to add hosts entry on node %s: %v", node, err)
				ui.Debug("Output: %s", string(output))
				continue
			}
		} else {
			ui.Debug("Registry entry already exists in /etc/hosts on node %s", node)
		}

		ui.Info("✓ Configured DNS on node %s (%s -> %s)", node, constants.RegistryDomain, gatewayIP)
		successCount++
	}

	// Check if any nodes were successful
	if successCount == 0 {
		return fmt.Errorf("failed to configure DNS on any node: %w", lastError)
	}
	if successCount < len(nodes) {
		ui.Warn("DNS configured on %d/%d nodes", successCount, len(nodes))
		return fmt.Errorf("DNS configuration incomplete (%d/%d nodes)", successCount, len(nodes))
	}

	ui.Success("✓ Configured registry DNS on all %d minikube nodes", len(nodes))
	return nil
}

// InstallNFSClient installs NFS client utilities on all minikube nodes.
// This allows nodes to mount NFS volumes for persistent storage.
func InstallNFSClient(ctx context.Context, cfg *config.Config) error {
	// Verify minikube is running
	running, err := IsRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check minikube status: %w", err)
	}
	if !running {
		return fmt.Errorf("minikube is not running")
	}

	// Get all node names
	nodes, err := GetNodeNames(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to get node names: %w", err)
	}

	ui.Debug("Installing NFS client on minikube nodes...")

	// Track success/failure for each node
	successCount := 0
	var lastError error

	for _, node := range nodes {
		ui.Debug("Installing NFS client on node %s...", node)

		// Install nfs-common package (Debian/Ubuntu) - required for kernel NFS mounting
		// Minikube uses Ubuntu, so we use apt-get
		// Note: The entire command must be a single string for bash -c
		installCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--",
			"sudo", "bash", "-c", "'apt-get update -qq && apt-get install -y -qq nfs-common'")
		setEnglishLocale(installCmd)

		if output, err := installCmd.CombinedOutput(); err != nil {
			lastError = fmt.Errorf("failed to install NFS client on node %s: %w", node, err)
			ui.Error("✗ Failed to install NFS client on node %s: %v", node, err)
			ui.Debug("Output: %s", string(output))
			continue
		}

		ui.Info("✓ Installed NFS client on node %s", node)
		successCount++
	}

	// Check if any nodes were successful
	if successCount == 0 {
		return fmt.Errorf("failed to install NFS client on any node: %w", lastError)
	}
	if successCount < len(nodes) {
		ui.Warn("NFS client installed on %d/%d nodes", successCount, len(nodes))
		return fmt.Errorf("NFS client installation incomplete (%d/%d nodes)", successCount, len(nodes))
	}

	ui.Success("✓ Installed NFS client on all %d minikube nodes", len(nodes))
	return nil
}

// InstallRegistryCA installs the mkcert CA certificate on all minikube nodes.
// This allows Docker on the nodes to trust the registry's TLS certificate.
func InstallRegistryCA(ctx context.Context, cfg *config.Config) error {
	// Verify minikube is running
	running, err := IsRunning(ctx)
	if err != nil {
		return fmt.Errorf("failed to check minikube status: %w", err)
	}
	if !running {
		return fmt.Errorf("minikube is not running")
	}

	// Get mkcert CA certificate path
	caCertPath, _, err := pki.GetCAPaths()
	if err != nil {
		return fmt.Errorf("failed to get mkcert CA certificate: %w", err)
	}

	// Read CA certificate content
	caCertData, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Get all node names
	nodes, err := GetNodeNames(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to get node names: %w", err)
	}

	ui.Debug("Installing mkcert CA certificate on minikube nodes...")

	// Create a temporary file for the CA certificate
	tmpFile, err := os.CreateTemp("", "nova-ca-*.crt")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write CA cert to temp file
	if _, err := tmpFile.Write(caCertData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write CA cert to temp file: %w", err)
	}
	tmpFile.Close()

	// Track success/failure for each node
	successCount := 0
	var lastError error

	// Install CA cert on each node using minikube cp
	for _, node := range nodes {
		ui.Debug("Installing CA certificate on node %s...", node)

		// Create directory for Docker registry certs
		certDir := fmt.Sprintf("/etc/docker/certs.d/%s", constants.RegistryHost)
		mkdirCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--",
			"sudo", "mkdir", "-p", certDir)
		setEnglishLocale(mkdirCmd)
		if _, err := mkdirCmd.CombinedOutput(); err != nil {
			lastError = fmt.Errorf("failed to create cert directory on node %s: %w", node, err)
			ui.Error("✗ Failed to create cert directory on node %s: %v", node, err)
			continue
		}

		// Copy CA cert to node using minikube cp (to temp location first)
		tempDest := fmt.Sprintf("%s:/tmp/nova-ca.crt", node)
		cpCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "cp", tmpFile.Name(), tempDest)
		setEnglishLocale(cpCmd)
		if output, err := cpCmd.CombinedOutput(); err != nil {
			lastError = fmt.Errorf("failed to copy CA cert to node %s: %w", node, err)
			ui.Error("✗ Failed to copy CA cert to node %s: %v", node, err)
			ui.Debug("Output: %s", string(output))
			continue
		}

		// Move cert to final location with sudo
		finalCertPath := fmt.Sprintf("%s/ca.crt", certDir)
		moveCmd := execCmd.CommandContext(ctx, "minikube", "-p", "nova", "ssh", "-n", node, "--",
			"sudo", "mv", "/tmp/nova-ca.crt", finalCertPath)
		setEnglishLocale(moveCmd)
		if output, err := moveCmd.CombinedOutput(); err != nil {
			lastError = fmt.Errorf("failed to move CA cert on node %s: %w", node, err)
			ui.Error("✗ Failed to move CA cert on node %s: %v", node, err)
			ui.Debug("Output: %s", string(output))
			continue
		}

		ui.Info("✓ Installed CA certificate on node %s", node)
		successCount++
	}

	// Check if any nodes were successful
	if successCount == 0 {
		return fmt.Errorf("failed to install CA certificate on any node: %w", lastError)
	}
	if successCount < len(nodes) {
		ui.Warn("CA certificate installed on %d/%d nodes", successCount, len(nodes))
		return fmt.Errorf("CA certificate installation incomplete (%d/%d nodes)", successCount, len(nodes))
	}

	ui.Success("✓ Installed mkcert CA certificate on all %d minikube nodes", len(nodes))
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
//   - Multi-node GPU mode: elect a GPU node (nova.local/node-type=gpu-nvidia)
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

	nodeCount := cfg.Minikube.Nodes
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
		// Multi-node mode: elect based on GPU/CPU mode
		if cfg.IsGPUMode() {
			// GPU mode: elect a GPU node
			ui.Info("Multi-node GPU mode: electing GPU node for llm-d")
			gpuNodes, err := GetNodesByLabel(ctx, cfg, "nova.local/node-type=gpu-nvidia")
			if err != nil {
				return "", fmt.Errorf("failed to get GPU nodes: %w", err)
			}
			if len(gpuNodes) == 0 {
				return "", fmt.Errorf("no GPU nodes found with label nova.local/node-type=gpu-nvidia")
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
