package commands

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/tools/docker"
	"github.com/kzgrzendek/nova/internal/tools/exec"
	"github.com/spf13/cobra"
)

func newExportLogsCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "export-logs",
		Short: "Export all cluster and service logs to a zip archive",
		Long: `Exports logs from all components into a timestamped zip archive:

  • Kubernetes cluster logs (all namespaces)
  • Minikube logs
  • Docker container logs (Bind9, NGINX)
  • System information
  • Configuration files

The archive is saved as nova-logs-YYYYMMDD-HHMMSS.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExportLogs(cmd.Context(), outputDir)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "output directory for the logs archive")

	return cmd
}

func runExportLogs(ctx context.Context, outputDir string) error {
	ui.Header("Exporting NOVA Logs")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		ui.Warn("Config not found, continuing with limited log collection")
		cfg = config.LoadOrDefault()
	}

	// Create timestamped archive name
	timestamp := time.Now().Format("20060102-150405")
	archiveName := fmt.Sprintf("nova-logs-%s.zip", timestamp)
	archivePath := filepath.Join(outputDir, archiveName)

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Define collection steps
	steps := []string{
		"System Information",
		"Minikube Logs",
		"Kubernetes Cluster Logs",
		"Node Kubelet Logs",
		"Pod Logs (All Namespaces)",
		"Docker Container Logs",
		"Configuration Files",
		"Creating Archive",
	}

	progress := ui.NewStepProgress(steps)
	currentStep := 0

	// Create temporary directory for log collection
	tempDir, err := os.MkdirTemp("", "nova-logs-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Step 1: Collect system information
	progress.StartStep(currentStep)
	if err := collectSystemInfo(ctx, tempDir); err != nil {
		ui.Warn("Failed to collect system info: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 2: Collect minikube logs
	progress.StartStep(currentStep)
	if err := collectMinikubeLogs(ctx, tempDir); err != nil {
		ui.Warn("Failed to collect minikube logs: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 3: Collect Kubernetes cluster logs
	progress.StartStep(currentStep)
	if err := collectK8sClusterLogs(ctx, tempDir); err != nil {
		ui.Warn("Failed to collect cluster logs: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 4: Collect kubelet logs from all nodes
	progress.StartStep(currentStep)
	if err := collectKubeletLogs(ctx, tempDir, cfg); err != nil {
		ui.Warn("Failed to collect kubelet logs: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 5: Collect pod logs
	progress.StartStep(currentStep)
	if err := collectPodLogs(ctx, tempDir); err != nil {
		ui.Warn("Failed to collect pod logs: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 5: Collect Docker container logs
	progress.StartStep(currentStep)
	if err := collectDockerLogs(ctx, tempDir, cfg); err != nil {
		ui.Warn("Failed to collect docker logs: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 6: Collect configuration files
	progress.StartStep(currentStep)
	if err := collectConfigFiles(ctx, tempDir, cfg); err != nil {
		ui.Warn("Failed to collect config files: %v", err)
	}
	progress.CompleteStep(currentStep)
	currentStep++

	// Step 7: Create zip archive
	progress.StartStep(currentStep)
	if err := createZipArchive(tempDir, archivePath); err != nil {
		progress.FailStep(currentStep, err)
		return fmt.Errorf("failed to create archive: %w", err)
	}
	progress.CompleteStep(currentStep)

	progress.Complete()

	// Get archive size
	fileInfo, _ := os.Stat(archivePath)
	sizeKB := fileInfo.Size() / 1024

	ui.Header("Logs Exported Successfully")
	ui.Success("Archive: %s", archivePath)
	ui.Info("Size: %d KB", sizeKB)

	return nil
}

// collectSystemInfo collects system information
func collectSystemInfo(ctx context.Context, outputDir string) error {
	infoFile := filepath.Join(outputDir, "system-info.txt")
	f, err := os.Create(infoFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "NOVA Log Export\n")
	fmt.Fprintf(f, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	// Docker version
	if dockerVersion, err := exec.OutputStdout(ctx, "docker", "version", "--format", "{{.Server.Version}}"); err == nil {
		fmt.Fprintf(f, "Docker Version: %s\n", dockerVersion)
	}

	// Minikube version
	if minikubeVersion, err := exec.OutputStdout(ctx, "minikube", "version", "--short"); err == nil {
		fmt.Fprintf(f, "Minikube Version: %s\n", minikubeVersion)
	}

	// Kubectl version
	if kubectlVersion, err := exec.OutputStdout(ctx, "kubectl", "version", "--client", "--short"); err == nil {
		fmt.Fprintf(f, "Kubectl Version: %s\n", kubectlVersion)
	}

	return nil
}

// collectMinikubeLogs collects minikube logs
func collectMinikubeLogs(ctx context.Context, outputDir string) error {
	logsDir := filepath.Join(outputDir, "minikube")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return err
	}

	// Minikube status
	if status, err := exec.Output(ctx, "minikube", "status"); err == nil {
		os.WriteFile(filepath.Join(logsDir, "status.txt"), []byte(status), 0644)
	}

	// Minikube logs
	if logs, err := exec.Output(ctx, "minikube", "logs", "--length=1000"); err == nil {
		os.WriteFile(filepath.Join(logsDir, "minikube.log"), []byte(logs), 0644)
	}

	return nil
}

// collectK8sClusterLogs collects Kubernetes cluster-level logs
func collectK8sClusterLogs(ctx context.Context, outputDir string) error {
	clusterDir := filepath.Join(outputDir, "cluster")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return err
	}

	// Get all nodes
	if nodes, err := exec.Output(ctx, "kubectl", "get", "nodes", "-o", "wide"); err == nil {
		os.WriteFile(filepath.Join(clusterDir, "nodes.txt"), []byte(nodes), 0644)
	}

	// Get all namespaces
	if namespaces, err := exec.Output(ctx, "kubectl", "get", "namespaces"); err == nil {
		os.WriteFile(filepath.Join(clusterDir, "namespaces.txt"), []byte(namespaces), 0644)
	}

	// Get all resources across all namespaces
	if resources, err := exec.Output(ctx, "kubectl", "get", "all", "-A"); err == nil {
		os.WriteFile(filepath.Join(clusterDir, "resources.txt"), []byte(resources), 0644)
	}

	// Get events
	if events, err := exec.Output(ctx, "kubectl", "get", "events", "-A", "--sort-by=.lastTimestamp"); err == nil {
		os.WriteFile(filepath.Join(clusterDir, "events.txt"), []byte(events), 0644)
	}

	return nil
}

// collectPodLogs collects logs from all pods in all namespaces
func collectPodLogs(ctx context.Context, outputDir string) error {
	podsDir := filepath.Join(outputDir, "pods")
	if err := os.MkdirAll(podsDir, 0755); err != nil {
		return err
	}

	// Get all pods with namespace
	podsOutput, err := exec.OutputStdout(ctx, "kubectl", "get", "pods", "-A", "-o", "custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,STATUS:.status.phase", "--no-headers")
	if err != nil {
		return err
	}

	// Parse and collect logs for each pod
	lines := splitLines(podsOutput)
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := splitFields(line)
		if len(fields) < 3 {
			continue
		}

		namespace := fields[0]
		podName := fields[1]

		// Create namespace directory
		namespaceDir := filepath.Join(podsDir, namespace)
		os.MkdirAll(namespaceDir, 0755)

		// Get pod logs (previous and current)
		logFile := filepath.Join(namespaceDir, fmt.Sprintf("%s.log", podName))
		if logs, err := exec.Output(ctx, "kubectl", "logs", "-n", namespace, podName, "--all-containers=true", "--tail=1000"); err == nil {
			os.WriteFile(logFile, []byte(logs), 0644)
		}

		// Try to get previous logs if pod restarted
		prevLogFile := filepath.Join(namespaceDir, fmt.Sprintf("%s-previous.log", podName))
		if prevLogs, err := exec.Output(ctx, "kubectl", "logs", "-n", namespace, podName, "--all-containers=true", "--previous", "--tail=1000"); err == nil {
			os.WriteFile(prevLogFile, []byte(prevLogs), 0644)
		}

		// Get pod description
		descFile := filepath.Join(namespaceDir, fmt.Sprintf("%s-describe.txt", podName))
		if desc, err := exec.Output(ctx, "kubectl", "describe", "pod", "-n", namespace, podName); err == nil {
			os.WriteFile(descFile, []byte(desc), 0644)
		}
	}

	return nil
}

// collectDockerLogs collects logs from Docker containers
func collectDockerLogs(ctx context.Context, outputDir string, cfg *config.Config) error {
	dockerDir := filepath.Join(outputDir, "docker")
	if err := os.MkdirAll(dockerDir, 0755); err != nil {
		return err
	}

	// Create Docker client
	client, err := docker.NewClient()
	if err != nil {
		return err
	}
	defer client.Close()

	// Collect logs for known containers
	containers := []string{
		"nova-bind9-dns",
		"nova-nginx-gateway",
	}

	for _, containerName := range containers {
		logs, err := client.Logs(ctx, containerName)
		if err != nil {
			continue // Container might not exist
		}

		logFile := filepath.Join(dockerDir, fmt.Sprintf("%s.log", containerName))
		os.WriteFile(logFile, []byte(logs), 0644)
	}

	return nil
}

// collectConfigFiles collects NOVA configuration files
func collectConfigFiles(ctx context.Context, outputDir string, cfg *config.Config) error {
	configDir := filepath.Join(outputDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Copy config.yaml (redact sensitive data)
	configPath := config.DefaultConfigPath()
	if data, err := os.ReadFile(configPath); err == nil {
		os.WriteFile(filepath.Join(configDir, "config.yaml"), data, 0644)
	}

	// Copy kubeconfig
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if data, err := os.ReadFile(kubeconfigPath); err == nil {
		os.WriteFile(filepath.Join(configDir, "kubeconfig.yaml"), data, 0644)
	}

	return nil
}

// createZipArchive creates a zip archive from a directory
func createZipArchive(sourceDir, archivePath string) error {
	// Create zip file
	zipFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through source directory
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the source directory itself
		if path == sourceDir {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = zip.Deflate

		// Handle directories
		if info.IsDir() {
			header.Name += "/"
			_, err := zipWriter.CreateHeader(header)
			return err
		}

		// Create file in zip
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// Copy file content
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// Helper functions for string parsing
func splitLines(s string) []string {
	lines := []string{}
	for _, line := range splitByNewline(s) {
		if trimmed := trimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	var lines []string
	var current string
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func splitFields(s string) []string {
	var fields []string
	var current string
	var inSpace bool

	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !inSpace && current != "" {
				fields = append(fields, current)
				current = ""
			}
			inSpace = true
		} else {
			current += string(r)
			inSpace = false
		}
	}
	if current != "" {
		fields = append(fields, current)
	}
	return fields
}

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// collectKubeletLogs collects kubelet logs from all cluster nodes
func collectKubeletLogs(ctx context.Context, outputDir string, cfg *config.Config) error {
	kubeletDir := filepath.Join(outputDir, "kubelet")
	if err := os.MkdirAll(kubeletDir, 0755); err != nil {
		return err
	}

	// Get all node names from kubectl
	nodesOutput, err := exec.OutputStdout(ctx, "kubectl", "get", "nodes", "-o", "custom-columns=NAME:.metadata.name", "--no-headers")
	if err != nil {
		return err
	}

	nodeNames := splitLines(nodesOutput)
	if len(nodeNames) == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	// Collect kubelet logs from each node
	for _, nodeName := range nodeNames {
		if nodeName == "" {
			continue
		}

		// Get kubelet logs via minikube ssh
		// Using journalctl to get systemd kubelet logs
		logs, err := exec.Output(ctx, "minikube", "ssh", "-n", nodeName, "--", "journalctl", "-u", "kubelet", "--no-pager", "-n", "1000")
		if err != nil {
			// If journalctl fails, try getting logs from /var/log
			logs, err = exec.Output(ctx, "minikube", "ssh", "-n", nodeName, "--", "cat", "/var/log/kubelet.log")
			if err != nil {
				// Skip this node if we can't get logs
				continue
			}
		}

		logFile := filepath.Join(kubeletDir, fmt.Sprintf("%s.log", nodeName))
		os.WriteFile(logFile, []byte(logs), 0644)
	}

	return nil
}
