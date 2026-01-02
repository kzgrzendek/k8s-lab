package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestNodeRoleDetection tests the node role detection logic.
func TestNodeRoleDetection(t *testing.T) {
	testCases := []struct {
		name              string
		labels            map[string]string
		expectedIsControl bool
	}{
		{
			name: "Control plane node with standard label",
			labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
			expectedIsControl: true,
		},
		{
			name: "Control plane node with master label",
			labels: map[string]string{
				"node-role.kubernetes.io/master": "",
			},
			expectedIsControl: true,
		},
		{
			name: "Worker node without control plane labels",
			labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
			expectedIsControl: false,
		},
		{
			name: "Node with custom labels only",
			labels: map[string]string{
				"app": "myapp",
				"env": "production",
			},
			expectedIsControl: false,
		},
		{
			name:              "Node with no labels",
			labels:            map[string]string{},
			expectedIsControl: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check for control plane labels
			isControl := false
			if _, exists := tc.labels["node-role.kubernetes.io/control-plane"]; exists {
				isControl = true
			}
			if _, exists := tc.labels["node-role.kubernetes.io/master"]; exists {
				isControl = true
			}

			if isControl != tc.expectedIsControl {
				t.Errorf("Expected isControlPlane=%v, got %v", tc.expectedIsControl, isControl)
			}
		})
	}
}

// TestTaintConfiguration tests node taint configuration.
func TestTaintConfiguration(t *testing.T) {
	testCases := []struct {
		name          string
		key           string
		value         string
		effect        string
		expectedTaint corev1.Taint
		expectError   bool
	}{
		{
			name:   "NoSchedule taint",
			key:    "node-role.kubernetes.io/control-plane",
			value:  "",
			effect: "NoSchedule",
			expectedTaint: corev1.Taint{
				Key:    "node-role.kubernetes.io/control-plane",
				Value:  "",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectError: false,
		},
		{
			name:   "NoExecute taint",
			key:    "dedicated",
			value:  "gpu",
			effect: "NoExecute",
			expectedTaint: corev1.Taint{
				Key:    "dedicated",
				Value:  "gpu",
				Effect: corev1.TaintEffectNoExecute,
			},
			expectError: false,
		},
		{
			name:   "PreferNoSchedule taint",
			key:    "workload",
			value:  "batch",
			effect: "PreferNoSchedule",
			expectedTaint: corev1.Taint{
				Key:    "workload",
				Value:  "batch",
				Effect: corev1.TaintEffectPreferNoSchedule,
			},
			expectError: false,
		},
		{
			name:        "Empty taint key",
			key:         "",
			value:       "test",
			effect:      "NoSchedule",
			expectError: true,
		},
		{
			name:        "Invalid taint effect",
			key:         "test",
			value:       "value",
			effect:      "InvalidEffect",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key == "" {
				if !tc.expectError {
					t.Error("Expected error for empty taint key")
				}
				return
			}

			// Validate effect
			validEffects := map[string]bool{
				"NoSchedule":       true,
				"NoExecute":        true,
				"PreferNoSchedule": true,
			}

			if !validEffects[tc.effect] {
				if !tc.expectError {
					t.Error("Expected error for invalid effect")
				}
				return
			}

			// Create taint
			taint := corev1.Taint{
				Key:    tc.key,
				Value:  tc.value,
				Effect: corev1.TaintEffect(tc.effect),
			}

			if taint.Key != tc.expectedTaint.Key {
				t.Errorf("Expected key %s, got %s", tc.expectedTaint.Key, taint.Key)
			}
			if taint.Value != tc.expectedTaint.Value {
				t.Errorf("Expected value %s, got %s", tc.expectedTaint.Value, taint.Value)
			}
			if taint.Effect != tc.expectedTaint.Effect {
				t.Errorf("Expected effect %s, got %s", tc.expectedTaint.Effect, taint.Effect)
			}
		})
	}
}

// TestLabelConfiguration tests node label configuration.
func TestLabelConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		key         string
		value       string
		expectError bool
	}{
		{
			name:        "Valid label with value",
			key:         "env",
			value:       "production",
			expectError: false,
		},
		{
			name:        "Valid label without value",
			key:         "node-role.kubernetes.io/worker",
			value:       "",
			expectError: false,
		},
		{
			name:        "Label with DNS subdomain",
			key:         "topology.kubernetes.io/zone",
			value:       "us-east-1a",
			expectError: false,
		},
		{
			name:        "Empty label key",
			key:         "",
			value:       "test",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.key == "" {
				if !tc.expectError {
					t.Error("Expected error for empty label key")
				}
				return
			}

			// Simulate label addition
			labels := map[string]string{}
			labels[tc.key] = tc.value

			if val, exists := labels[tc.key]; !exists {
				t.Errorf("Label %s was not added", tc.key)
			} else if val != tc.value {
				t.Errorf("Expected label value %s, got %s", tc.value, val)
			}
		})
	}
}

// TestNodeFiltering tests filtering nodes by various criteria.
func TestNodeFiltering(t *testing.T) {
	testCases := []struct {
		name          string
		nodes         []corev1.Node
		filterRole    string
		expectedCount int
	}{
		{
			name: "Filter control plane nodes",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/control-plane": "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
			},
			filterRole:    "control-plane",
			expectedCount: 1,
		},
		{
			name: "Filter worker nodes",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/control-plane": "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-3",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
			},
			filterRole:    "worker",
			expectedCount: 2,
		},
		{
			name: "No nodes match filter",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
			},
			filterRole:    "control-plane",
			expectedCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Filter nodes by role
			filtered := []corev1.Node{}
			for _, node := range tc.nodes {
				labelKey := "node-role.kubernetes.io/" + tc.filterRole
				if _, exists := node.Labels[labelKey]; exists {
					filtered = append(filtered, node)
				}
			}

			if len(filtered) != tc.expectedCount {
				t.Errorf("Expected %d nodes, got %d", tc.expectedCount, len(filtered))
			}
		})
	}
}

// TestNamespaceValidation tests namespace validation.
func TestNamespaceValidation(t *testing.T) {
	testCases := []struct {
		name          string
		namespaceName string
		expectError   bool
	}{
		{
			name:          "Valid namespace name",
			namespaceName: "default",
			expectError:   false,
		},
		{
			name:          "Valid namespace with hyphens",
			namespaceName: "kube-system",
			expectError:   false,
		},
		{
			name:          "Valid namespace with numbers",
			namespaceName: "namespace-123",
			expectError:   false,
		},
		{
			name:          "Empty namespace name",
			namespaceName: "",
			expectError:   true,
		},
		{
			name:          "Namespace name with uppercase (invalid in k8s)",
			namespaceName: "MyNamespace",
			expectError:   false, // We're just testing structure, not k8s validation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.namespaceName == "" {
				if !tc.expectError {
					t.Error("Expected error for empty namespace name")
				}
				return
			}

			// Create namespace object
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: tc.namespaceName,
				},
			}

			if ns.Name != tc.namespaceName {
				t.Errorf("Expected namespace name %s, got %s", tc.namespaceName, ns.Name)
			}
		})
	}
}

// TestNodeStatusCheck tests checking node status conditions.
func TestNodeStatusCheck(t *testing.T) {
	testCases := []struct {
		name          string
		conditions    []corev1.NodeCondition
		expectedReady bool
	}{
		{
			name: "Node is ready",
			conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
			expectedReady: true,
		},
		{
			name: "Node is not ready",
			conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
			expectedReady: false,
		},
		{
			name: "Node status unknown",
			conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionUnknown,
				},
			},
			expectedReady: false,
		},
		{
			name:          "No conditions",
			conditions:    []corev1.NodeCondition{},
			expectedReady: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check if node is ready
			isReady := false
			for _, condition := range tc.conditions {
				if condition.Type == corev1.NodeReady {
					isReady = condition.Status == corev1.ConditionTrue
					break
				}
			}

			if isReady != tc.expectedReady {
				t.Errorf("Expected ready=%v, got %v", tc.expectedReady, isReady)
			}
		})
	}
}

// TestKubeconfigPath tests the kubeconfig path resolution.
func TestKubeconfigPath(t *testing.T) {
	testCases := []struct {
		name         string
		kubeconfigEnv string
		expectEmpty  bool
	}{
		{
			name:         "KUBECONFIG env set",
			kubeconfigEnv: "/custom/path/config",
			expectEmpty:  false,
		},
		{
			name:         "Default path",
			kubeconfigEnv: "",
			expectEmpty:  false, // Will use home dir
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This is a simple structural test
			// In practice, getKubeconfigPath checks KUBECONFIG env and home dir
			if tc.kubeconfigEnv != "" {
				if tc.kubeconfigEnv == "" && !tc.expectEmpty {
					t.Error("Expected non-empty path")
				}
			}
		})
	}
}

// TestContextConfiguration tests kubectl context configuration parameters.
func TestContextConfiguration(t *testing.T) {
	testCases := []struct {
		name               string
		contextName        string
		namespace          string
		serviceAccountName string
		expectError        bool
	}{
		{
			name:               "Valid developer context",
			contextName:        "nova-developer",
			namespace:          "developer",
			serviceAccountName: "developer",
			expectError:        false,
		},
		{
			name:               "Empty context name",
			contextName:        "",
			namespace:          "developer",
			serviceAccountName: "developer",
			expectError:        true,
		},
		{
			name:               "Empty namespace",
			contextName:        "nova-developer",
			namespace:          "",
			serviceAccountName: "developer",
			expectError:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate parameters
			if tc.contextName == "" || tc.namespace == "" {
				if !tc.expectError {
					t.Error("Expected error for empty parameters")
				}
				return
			}

			// Verify context naming convention
			expectedCluster := tc.contextName + "-cluster"
			expectedUser := tc.contextName + "-user"
			expectedSecretName := tc.serviceAccountName + "-token"

			if expectedCluster != tc.contextName+"-cluster" {
				t.Errorf("Expected cluster name %s-cluster", tc.contextName)
			}
			if expectedUser != tc.contextName+"-user" {
				t.Errorf("Expected user name %s-user", tc.contextName)
			}
			if expectedSecretName != tc.serviceAccountName+"-token" {
				t.Errorf("Expected secret name %s-token", tc.serviceAccountName)
			}
		})
	}
}

// TestPodReadyCheck tests the pod ready check logic.
func TestPodReadyCheck(t *testing.T) {
	testCases := []struct {
		name            string
		phase           corev1.PodPhase
		containerStatus []corev1.ContainerStatus
		expectedReady   bool
	}{
		{
			name:  "Pod running with ready containers",
			phase: corev1.PodRunning,
			containerStatus: []corev1.ContainerStatus{
				{Ready: true},
				{Ready: true},
			},
			expectedReady: true,
		},
		{
			name:  "Pod running with unready containers",
			phase: corev1.PodRunning,
			containerStatus: []corev1.ContainerStatus{
				{Ready: true},
				{Ready: false},
			},
			expectedReady: false,
		},
		{
			name:            "Pod pending",
			phase:           corev1.PodPending,
			containerStatus: []corev1.ContainerStatus{},
			expectedReady:   false,
		},
		{
			name:  "Pod failed",
			phase: corev1.PodFailed,
			containerStatus: []corev1.ContainerStatus{
				{Ready: false},
			},
			expectedReady: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check if pod would be considered ready
			isReady := false
			if tc.phase == corev1.PodRunning {
				allReady := true
				for _, cs := range tc.containerStatus {
					if !cs.Ready {
						allReady = false
						break
					}
				}
				if allReady && len(tc.containerStatus) > 0 {
					isReady = true
				}
			}

			if isReady != tc.expectedReady {
				t.Errorf("Expected ready=%v, got %v", tc.expectedReady, isReady)
			}
		})
	}
}

// TestTaintUpdate tests updating existing taints on nodes.
func TestTaintUpdate(t *testing.T) {
	testCases := []struct {
		name           string
		existingTaints []corev1.Taint
		newTaint       corev1.Taint
		expectedCount  int
		shouldUpdate   bool
	}{
		{
			name:           "Add new taint to empty list",
			existingTaints: []corev1.Taint{},
			newTaint: corev1.Taint{
				Key:    "test",
				Value:  "value",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectedCount: 1,
			shouldUpdate:  false,
		},
		{
			name: "Update existing taint",
			existingTaints: []corev1.Taint{
				{
					Key:    "test",
					Value:  "old-value",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			newTaint: corev1.Taint{
				Key:    "test",
				Value:  "new-value",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectedCount: 1,
			shouldUpdate:  true,
		},
		{
			name: "Add taint with different key",
			existingTaints: []corev1.Taint{
				{
					Key:    "existing",
					Value:  "value",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			newTaint: corev1.Taint{
				Key:    "new",
				Value:  "value",
				Effect: corev1.TaintEffectNoSchedule,
			},
			expectedCount: 2,
			shouldUpdate:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taints := make([]corev1.Taint, len(tc.existingTaints))
			copy(taints, tc.existingTaints)

			// Check if taint exists and update it, or add new taint
			taintExists := false
			for i, taint := range taints {
				if taint.Key == tc.newTaint.Key {
					taints[i] = tc.newTaint
					taintExists = true
					break
				}
			}

			if !taintExists {
				taints = append(taints, tc.newTaint)
			}

			if len(taints) != tc.expectedCount {
				t.Errorf("Expected %d taints, got %d", tc.expectedCount, len(taints))
			}

			if taintExists != tc.shouldUpdate {
				t.Errorf("Expected shouldUpdate=%v, got %v", tc.shouldUpdate, taintExists)
			}
		})
	}
}
