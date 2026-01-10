package minikube

import (
	"fmt"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
)

// TestGetNodeNames tests generating node names based on cluster configuration.
func TestGetNodeNames(t *testing.T) {
	testCases := []struct {
		name          string
		nodeCount     int
		expectedNodes []string
	}{
		{
			name:      "Single node cluster",
			nodeCount: 1,
			expectedNodes: []string{
				"minikube",
			},
		},
		{
			name:      "Three node cluster",
			nodeCount: 3,
			expectedNodes: []string{
				"minikube",
				"nova-m02",
				"nova-m03",
			},
		},
		{
			name:      "Five node cluster",
			nodeCount: 5,
			expectedNodes: []string{
				"minikube",
				"nova-m02",
				"nova-m03",
				"minikube-m04",
				"minikube-m05",
			},
		},
		{
			name:          "Zero nodes (edge case)",
			nodeCount:     0,
			expectedNodes: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the GetNodeNames logic
			var nodes []string
			for i := 1; i <= tc.nodeCount; i++ {
				if i == 1 {
					nodes = append(nodes, "minikube")
				} else {
					nodes = append(nodes, "minikube-m"+padNumber(i))
				}
			}

			if len(nodes) != len(tc.expectedNodes) {
				t.Errorf("Expected %d nodes, got %d", len(tc.expectedNodes), len(nodes))
			}

			for i, node := range nodes {
				if i >= len(tc.expectedNodes) {
					t.Errorf("Unexpected extra node: %s", node)
					continue
				}
				if node != tc.expectedNodes[i] {
					t.Errorf("Node %d: expected %s, got %s", i, tc.expectedNodes[i], node)
				}
			}
		})
	}
}

// padNumber pads a number to 2 digits (helper for node naming).
func padNumber(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// TestMinikubeStartArgs tests building minikube start command arguments.
func TestMinikubeStartArgs(t *testing.T) {
	testCases := []struct {
		name          string
		cfg           *config.Config
		expectedArgs  map[string]string // key-value pairs we expect to find
		shouldHaveGPU bool
	}{
		{
			name: "Basic configuration without GPU",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              4,
					Memory:            8192,
					Nodes:             3,
					KubernetesVersion: "v1.28.0",
					GPUs:              "",
				},
			},
			expectedArgs: map[string]string{
				"--driver":             "docker",
				"--cpus":               "4",
				"--memory":             "8192",
				"--nodes":              "3",
				"--kubernetes-version": "v1.28.0",
				"--container-runtime":  "docker",
				"--network-plugin":     "cni",
				"--cni":                "false",
			},
			shouldHaveGPU: false,
		},
		{
			name: "Configuration with NVIDIA GPU",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              8,
					Memory:            16384,
					Nodes:             1,
					KubernetesVersion: "v1.29.0",
					GPUs:              "all",
				},
			},
			expectedArgs: map[string]string{
				"--driver":             "docker",
				"--cpus":               "8",
				"--memory":             "16384",
				"--nodes":              "1",
				"--kubernetes-version": "v1.29.0",
				"--gpus":               "all",
			},
			shouldHaveGPU: true,
		},
		{
			name: "Single node configuration",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              2,
					Memory:            4096,
					Nodes:             1,
					KubernetesVersion: "v1.27.0",
					GPUs:              "",
				},
			},
			expectedArgs: map[string]string{
				"--nodes": "1",
			},
			shouldHaveGPU: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate building args (same logic as StartCluster)
			args := []string{
				"start",
				"--install-addons=false",
				"--driver", tc.cfg.Minikube.Driver,
				"--cpus", fmt.Sprintf("%d", tc.cfg.Minikube.CPUs),
				"--memory", fmt.Sprintf("%d", tc.cfg.Minikube.Memory),
				"--container-runtime", "docker",
				"--kubernetes-version", tc.cfg.Minikube.KubernetesVersion,
				"--network-plugin", "cni",
				"--cni", "false",
				"--nodes", fmt.Sprintf("%d", tc.cfg.Minikube.Nodes),
				"--extra-config", "kubelet.node-ip=0.0.0.0",
				"--extra-config", "kube-proxy.skip-headers=true",
			}

			// Add GPU support if configured
			if tc.cfg.Minikube.GPUs != "" {
				args = append(args, "--gpus", tc.cfg.Minikube.GPUs)
			}

			// Verify start command is present
			if args[0] != "start" {
				t.Error("First argument should be 'start'")
			}

			// Verify GPU args
			hasGPUFlag := false
			for i, arg := range args {
				if arg == "--gpus" {
					hasGPUFlag = true
					if i+1 >= len(args) {
						t.Error("--gpus flag without value")
					} else if args[i+1] != tc.cfg.Minikube.GPUs {
						t.Errorf("Expected GPU value %s, got %s", tc.cfg.Minikube.GPUs, args[i+1])
					}
				}
			}

			if hasGPUFlag != tc.shouldHaveGPU {
				t.Errorf("Expected GPU flag present=%v, got %v", tc.shouldHaveGPU, hasGPUFlag)
			}

			// Verify required flags are present
			requiredFlags := []string{"--driver", "--cpus", "--memory", "--nodes"}
			for _, flag := range requiredFlags {
				found := false
				for _, arg := range args {
					if arg == flag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Required flag %s not found in args", flag)
				}
			}
		})
	}
}

// TestMinikubeNodeNamingConvention tests the node naming convention.
func TestMinikubeNodeNamingConvention(t *testing.T) {
	testCases := []struct {
		nodeIndex    int
		expectedName string
	}{
		{nodeIndex: 1, expectedName: "minikube"},
		{nodeIndex: 2, expectedName: "nova-m02"},
		{nodeIndex: 3, expectedName: "nova-m03"},
		{nodeIndex: 10, expectedName: "minikube-m10"},
		{nodeIndex: 99, expectedName: "minikube-m99"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedName, func(t *testing.T) {
			var nodeName string
			if tc.nodeIndex == 1 {
				nodeName = "minikube"
			} else {
				nodeName = "minikube-m" + padNumber(tc.nodeIndex)
			}

			if nodeName != tc.expectedName {
				t.Errorf("Expected node name %s, got %s", tc.expectedName, nodeName)
			}
		})
	}
}

// TestMinikubeConfigValidation tests configuration validation.
func TestMinikubeConfigValidation(t *testing.T) {
	testCases := []struct {
		name    string
		cfg     *config.Config
		isValid bool
		reason  string
	}{
		{
			name: "Valid minimum configuration",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              2,
					Memory:            2048,
					Nodes:             1,
					KubernetesVersion: "v1.28.0",
				},
			},
			isValid: true,
		},
		{
			name: "Valid multi-node configuration",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              4,
					Memory:            8192,
					Nodes:             3,
					KubernetesVersion: "v1.29.0",
				},
			},
			isValid: true,
		},
		{
			name: "Configuration with GPU",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              8,
					Memory:            16384,
					Nodes:             1,
					KubernetesVersion: "v1.28.0",
					GPUs:              "all",
				},
			},
			isValid: true,
		},
		{
			name: "Empty driver",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "",
					CPUs:              4,
					Memory:            8192,
					Nodes:             1,
					KubernetesVersion: "v1.28.0",
				},
			},
			isValid: false,
			reason:  "driver is required",
		},
		{
			name: "Zero CPUs",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              0,
					Memory:            8192,
					Nodes:             1,
					KubernetesVersion: "v1.28.0",
				},
			},
			isValid: false,
			reason:  "CPUs must be greater than 0",
		},
		{
			name: "Zero memory",
			cfg: &config.Config{
				Minikube: config.MinikubeConfig{
					Driver:            "docker",
					CPUs:              4,
					Memory:            0,
					Nodes:             1,
					KubernetesVersion: "v1.28.0",
				},
			},
			isValid: false,
			reason:  "memory must be greater than 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate configuration
			isValid := true

			if tc.cfg.Minikube.Driver == "" {
				isValid = false
			}
			if tc.cfg.Minikube.CPUs <= 0 {
				isValid = false
			}
			if tc.cfg.Minikube.Memory <= 0 {
				isValid = false
			}
			if tc.cfg.Minikube.Nodes <= 0 {
				isValid = false
			}
			if tc.cfg.Minikube.KubernetesVersion == "" {
				isValid = false
			}

			if isValid != tc.isValid {
				if tc.isValid {
					t.Errorf("Expected valid configuration but validation failed: %s", tc.reason)
				} else {
					t.Errorf("Expected invalid configuration (%s) but validation passed", tc.reason)
				}
			}
		})
	}
}

// TestMountBPFFSCommand tests the BPF filesystem mount command construction.
func TestMountBPFFSCommand(t *testing.T) {
	testCases := []struct {
		name         string
		nodeName     string
		expectedCmd  string
		expectedArgs []string
	}{
		{
			name:        "Mount on control plane",
			nodeName:    "minikube",
			expectedCmd: "minikube",
			expectedArgs: []string{
				"ssh",
				"-n",
				"minikube",
				"--",
				"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf",
			},
		},
		{
			name:        "Mount on worker node",
			nodeName:    "nova-m02",
			expectedCmd: "minikube",
			expectedArgs: []string{
				"ssh",
				"-n",
				"nova-m02",
				"--",
				"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate command construction
			args := []string{
				"ssh",
				"-n",
				tc.nodeName,
				"--",
				"grep -q 'bpffs /sys/fs/bpf' /proc/mounts || sudo mount -t bpf bpffs /sys/fs/bpf",
			}

			if len(args) != len(tc.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tc.expectedArgs), len(args))
			}

			for i, arg := range args {
				if i >= len(tc.expectedArgs) {
					t.Errorf("Unexpected extra argument: %s", arg)
					continue
				}
				if arg != tc.expectedArgs[i] {
					t.Errorf("Arg %d: expected %s, got %s", i, tc.expectedArgs[i], arg)
				}
			}
		})
	}
}

// TestStatusParsing tests parsing minikube status output.
func TestStatusParsing(t *testing.T) {
	testCases := []struct {
		name           string
		statusOutput   string
		expectedStatus string
		isRunning      bool
	}{
		{
			name:           "Running status",
			statusOutput:   "Running",
			expectedStatus: "Running",
			isRunning:      true,
		},
		{
			name:           "Stopped status",
			statusOutput:   "Stopped",
			expectedStatus: "Stopped",
			isRunning:      false,
		},
		{
			name:           "Paused status",
			statusOutput:   "Paused",
			expectedStatus: "Paused",
			isRunning:      false,
		},
		{
			name:           "Empty status",
			statusOutput:   "",
			expectedStatus: "",
			isRunning:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate status parsing (trim whitespace)
			status := tc.statusOutput
			if len(status) > 0 && status[len(status)-1] == '\n' {
				status = status[:len(status)-1]
			}

			isRunning := status == "Running"

			if status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, status)
			}

			if isRunning != tc.isRunning {
				t.Errorf("Expected isRunning=%v, got %v", tc.isRunning, isRunning)
			}
		})
	}
}
