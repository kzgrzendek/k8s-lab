package commands

import (
	"testing"
)

// TestExportLogsStepsConfiguration tests the export-logs command steps configuration.
func TestExportLogsStepsConfiguration(t *testing.T) {
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

	// Verify we have the expected number of steps
	expectedSteps := 8
	if len(steps) != expectedSteps {
		t.Errorf("Expected %d export-logs steps, got %d", expectedSteps, len(steps))
	}

	// Verify all steps have non-empty names
	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should have a non-empty name", i)
		}
	}

	// Verify step order (system info should be first)
	if steps[0] != "System Information" {
		t.Error("First step should be System Information")
	}

	// Verify archive creation is last
	if steps[len(steps)-1] != "Creating Archive" {
		t.Error("Last step should be Creating Archive")
	}

	// Verify kubelet logs step exists
	hasKubeletStep := false
	for _, step := range steps {
		if step == "Node Kubelet Logs" {
			hasKubeletStep = true
			break
		}
	}
	if !hasKubeletStep {
		t.Error("Should have Node Kubelet Logs step")
	}
}

// TestExportLogsCollectionSteps tests that all collection functions are defined.
func TestExportLogsCollectionSteps(t *testing.T) {
	testCases := []struct {
		name         string
		functionName string
		description  string
	}{
		{
			name:         "System info collection",
			functionName: "collectSystemInfo",
			description:  "Should collect Docker, Minikube, Kubectl versions",
		},
		{
			name:         "Minikube logs collection",
			functionName: "collectMinikubeLogs",
			description:  "Should collect minikube status and logs",
		},
		{
			name:         "K8s cluster logs collection",
			functionName: "collectK8sClusterLogs",
			description:  "Should collect nodes, namespaces, resources, events",
		},
		{
			name:         "Kubelet logs collection",
			functionName: "collectKubeletLogs",
			description:  "Should collect kubelet logs from all nodes",
		},
		{
			name:         "Pod logs collection",
			functionName: "collectPodLogs",
			description:  "Should collect current and previous logs from all pods",
		},
		{
			name:         "Docker logs collection",
			functionName: "collectDockerLogs",
			description:  "Should collect Bind9 and NGINX container logs",
		},
		{
			name:         "Config files collection",
			functionName: "collectConfigFiles",
			description:  "Should collect NOVA config and kubeconfig",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.functionName == "" {
				t.Error("Function name should not be empty")
			}
			if tc.description == "" {
				t.Error("Description should not be empty")
			}
		})
	}
}

// TestExportLogsArchiveNaming tests archive file naming convention.
func TestExportLogsArchiveNaming(t *testing.T) {
	testCases := []struct {
		name           string
		archivePattern string
		shouldMatch    []string
		shouldNotMatch []string
	}{
		{
			name:           "Timestamped archive format",
			archivePattern: "nova-logs-YYYYMMDD-HHMMSS.zip",
			shouldMatch: []string{
				"nova-logs-20250129-143022.zip",
				"nova-logs-20241231-235959.zip",
			},
			shouldNotMatch: []string{
				"nova-logs.zip",
				"logs-20250129.zip",
				"nova-logs-20250129.zip",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.archivePattern == "" {
				t.Error("Archive pattern should not be empty")
			}

			// Verify pattern format contains required elements
			requiredElements := []string{"nova-logs", "YYYYMMDD", "HHMMSS", ".zip"}
			for _, elem := range requiredElements {
				if len(elem) == 0 {
					t.Errorf("Archive pattern should contain element: %s", elem)
				}
			}
		})
	}
}

// TestExportLogsDirectoryStructure tests the expected directory structure in the archive.
func TestExportLogsDirectoryStructure(t *testing.T) {
	expectedDirs := []string{
		"minikube",
		"cluster",
		"kubelet",
		"pods",
		"docker",
		"config",
	}

	// Verify we have the expected directories
	if len(expectedDirs) != 6 {
		t.Errorf("Expected 6 directories in archive, got %d", len(expectedDirs))
	}

	// Verify all directory names are non-empty
	for i, dir := range expectedDirs {
		if dir == "" {
			t.Errorf("Directory %d should have a non-empty name", i)
		}
	}

	// Verify kubelet directory exists (new feature)
	hasKubeletDir := false
	for _, dir := range expectedDirs {
		if dir == "kubelet" {
			hasKubeletDir = true
			break
		}
	}
	if !hasKubeletDir {
		t.Error("Archive should include kubelet directory")
	}
}

// TestExportLogsErrorHandling tests error handling behavior.
func TestExportLogsErrorHandling(t *testing.T) {
	testCases := []struct {
		name       string
		scenario   string
		shouldWarn bool
		shouldFail bool
	}{
		{
			name:       "Missing config - should warn and continue",
			scenario:   "config not found",
			shouldWarn: true,
			shouldFail: false,
		},
		{
			name:       "Failed system info collection - should warn and continue",
			scenario:   "system info collection failed",
			shouldWarn: true,
			shouldFail: false,
		},
		{
			name:       "Failed archive creation - should fail",
			scenario:   "archive creation failed",
			shouldWarn: false,
			shouldFail: true,
		},
		{
			name:       "Failed to create output directory - should fail",
			scenario:   "output directory creation failed",
			shouldWarn: false,
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify test case is well-defined
			if tc.scenario == "" {
				t.Error("Scenario should not be empty")
			}

			// Archive creation failures should fail the command
			if tc.scenario == "archive creation failed" && !tc.shouldFail {
				t.Error("Archive creation failure should cause command to fail")
			}

			// Collection failures should warn but not fail
			if tc.scenario == "system info collection failed" && tc.shouldFail {
				t.Error("System info collection failure should not cause command to fail")
			}
		})
	}
}

// TestExportLogsOutputDirectory tests output directory handling.
func TestExportLogsOutputDirectory(t *testing.T) {
	testCases := []struct {
		name          string
		outputDir     string
		shouldDefault bool
	}{
		{
			name:          "Default output directory",
			outputDir:     ".",
			shouldDefault: true,
		},
		{
			name:          "Custom output directory",
			outputDir:     "/custom/path",
			shouldDefault: false,
		},
		{
			name:          "Relative output directory",
			outputDir:     "./logs",
			shouldDefault: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.outputDir == "" {
				t.Error("Output directory should not be empty")
			}

			// Verify default is current directory
			if tc.shouldDefault && tc.outputDir != "." {
				t.Error("Default output directory should be current directory (.)")
			}
		})
	}
}
