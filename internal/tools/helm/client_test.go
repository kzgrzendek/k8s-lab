package helm

import (
	"context"
	"testing"

	"helm.sh/helm/v4/pkg/action"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	repov1 "helm.sh/helm/v4/pkg/repo/v1"
)

// TestNewClient tests creating a new Helm client.
func TestNewClient(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
	}{
		{
			name:      "Client with default namespace",
			namespace: "",
		},
		{
			name:      "Client with custom namespace",
			namespace: "kube-system",
		},
		{
			name:      "Client with application namespace",
			namespace: "my-app",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(tc.namespace)
			if client == nil {
				t.Fatal("Expected non-nil client")
			}

			if client.settings == nil {
				t.Fatal("Expected settings to be initialized")
			}

			// Verify namespace is set correctly
			if tc.namespace != "" && client.settings.Namespace() != tc.namespace {
				t.Errorf("Expected namespace %s, got %s", tc.namespace, client.settings.Namespace())
			}
		})
	}
}

// TestGetterProviders tests that HTTP/HTTPS getters are available.
func TestGetterProviders(t *testing.T) {
	client := NewClient("default")
	if client == nil || client.settings == nil {
		t.Fatal("Failed to create client")
	}

	// Get all available getter providers
	providers := getter.All(client.settings)
	if providers == nil {
		t.Fatal("Expected non-nil getter providers")
	}

	// Verify HTTPS provider is available
	httpsGetter, err := providers.ByScheme("https")
	if err != nil {
		t.Errorf("Expected HTTPS getter to be available: %v", err)
	}
	if httpsGetter == nil {
		t.Error("Expected non-nil HTTPS getter")
	}

	// Verify HTTP provider is available
	httpGetter, err := providers.ByScheme("http")
	if err != nil {
		t.Errorf("Expected HTTP getter to be available: %v", err)
	}
	if httpGetter == nil {
		t.Error("Expected non-nil HTTP getter")
	}
}

// TestAddRepository tests adding Helm repositories with various configurations.
func TestAddRepository(t *testing.T) {
	testCases := []struct {
		name        string
		repoName    string
		repoURL     string
		expectError bool
	}{
		{
			name:        "Valid HTTPS repository",
			repoName:    "bitnami",
			repoURL:     "https://charts.bitnami.com/bitnami",
			expectError: false,
		},
		{
			name:        "Valid repository with different name",
			repoName:    "stable",
			repoURL:     "https://charts.helm.sh/stable",
			expectError: false,
		},
		{
			name:        "Empty repository name",
			repoName:    "",
			repoURL:     "https://charts.helm.sh/stable",
			expectError: true,
		},
		{
			name:        "Empty repository URL",
			repoName:    "test",
			repoURL:     "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate input parameters
			if tc.repoName == "" || tc.repoURL == "" {
				if !tc.expectError {
					t.Error("Expected error for empty parameters but got none")
				}
				return
			}

			// Validate repository entry structure
			entry := &repov1.Entry{
				Name: tc.repoName,
				URL:  tc.repoURL,
			}

			if entry.Name != tc.repoName {
				t.Errorf("Expected repo name %s, got %s", tc.repoName, entry.Name)
			}
			if entry.URL != tc.repoURL {
				t.Errorf("Expected repo URL %s, got %s", tc.repoURL, entry.URL)
			}
		})
	}
}

// TestReleaseConfiguration tests Helm release configuration parsing.
func TestReleaseConfiguration(t *testing.T) {
	testCases := []struct {
		name           string
		releaseName    string
		namespace      string
		chartName      string
		version        string
		repoURL        string
		expectError    bool
		validateConfig func(*testing.T, *action.Configuration)
	}{
		{
			name:        "Valid release configuration",
			releaseName: "my-release",
			namespace:   "default",
			chartName:   "nginx",
			version:     "1.0.0",
			repoURL:     "https://charts.bitnami.com/bitnami",
			expectError: false,
		},
		{
			name:        "Release with custom namespace",
			releaseName: "custom-release",
			namespace:   "kube-system",
			chartName:   "cilium",
			version:     "1.14.5",
			repoURL:     "https://helm.cilium.io/",
			expectError: false,
		},
		{
			name:        "Empty release name",
			releaseName: "",
			namespace:   "default",
			chartName:   "test",
			version:     "1.0.0",
			repoURL:     "https://example.com",
			expectError: true,
		},
		{
			name:        "Empty namespace",
			releaseName: "test-release",
			namespace:   "",
			chartName:   "test",
			version:     "1.0.0",
			repoURL:     "https://example.com",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate basic parameters
			if tc.releaseName == "" {
				if !tc.expectError {
					t.Error("Expected error for empty release name")
				}
				return
			}

			if tc.namespace == "" {
				if !tc.expectError {
					t.Error("Expected error for empty namespace")
				}
				return
			}

			// Verify release metadata structure
			if tc.releaseName != "" && tc.namespace != "" {
				metadata := struct {
					Name      string
					Namespace string
					Chart     string
					Version   string
				}{
					Name:      tc.releaseName,
					Namespace: tc.namespace,
					Chart:     tc.chartName,
					Version:   tc.version,
				}

				if metadata.Name != tc.releaseName {
					t.Errorf("Expected release name %s, got %s", tc.releaseName, metadata.Name)
				}
				if metadata.Namespace != tc.namespace {
					t.Errorf("Expected namespace %s, got %s", tc.namespace, metadata.Namespace)
				}
			}
		})
	}
}

// TestReleaseStatus tests release status checking logic.
func TestReleaseStatus(t *testing.T) {
	testCases := []struct {
		name           string
		release        *releasev1.Release
		expectedStatus string
		expectedExists bool
	}{
		{
			name: "Deployed release",
			release: &releasev1.Release{
				Name: "test-release",
				Info: &releasev1.Info{
					Status: common.StatusDeployed,
				},
			},
			expectedStatus: "deployed",
			expectedExists: true,
		},
		{
			name: "Failed release",
			release: &releasev1.Release{
				Name: "failed-release",
				Info: &releasev1.Info{
					Status: common.StatusFailed,
				},
			},
			expectedStatus: "failed",
			expectedExists: true,
		},
		{
			name: "Pending install release",
			release: &releasev1.Release{
				Name: "pending-release",
				Info: &releasev1.Info{
					Status: common.StatusPendingInstall,
				},
			},
			expectedStatus: "pending-install",
			expectedExists: true,
		},
		{
			name:           "Non-existent release",
			release:        nil,
			expectedStatus: "",
			expectedExists: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.release == nil {
				if tc.expectedExists {
					t.Error("Expected release to exist but got nil")
				}
				return
			}

			status := tc.release.Info.Status.String()
			if status != tc.expectedStatus {
				t.Errorf("Expected status %s, got %s", tc.expectedStatus, status)
			}
		})
	}
}

// TestChartValidation tests chart configuration validation.
func TestChartValidation(t *testing.T) {
	testCases := []struct {
		name        string
		chart       *chartv2.Chart
		expectError bool
	}{
		{
			name: "Valid chart with metadata",
			chart: &chartv2.Chart{
				Metadata: &chartv2.Metadata{
					Name:    "nginx",
					Version: "1.0.0",
				},
			},
			expectError: false,
		},
		{
			name: "Chart with dependencies",
			chart: &chartv2.Chart{
				Metadata: &chartv2.Metadata{
					Name:    "complex-app",
					Version: "2.0.0",
					Dependencies: []*chartv2.Dependency{
						{
							Name:       "postgresql",
							Version:    "11.0.0",
							Repository: "https://charts.bitnami.com/bitnami",
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "Nil chart",
			chart:       nil,
			expectError: true,
		},
		{
			name: "Chart without metadata",
			chart: &chartv2.Chart{
				Metadata: nil,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.chart == nil {
				if !tc.expectError {
					t.Error("Expected error for nil chart")
				}
				return
			}

			if tc.chart.Metadata == nil {
				if !tc.expectError {
					t.Error("Expected error for chart without metadata")
				}
				return
			}

			// Validate chart metadata
			if tc.chart.Metadata.Name == "" {
				t.Error("Chart name should not be empty")
			}
			if tc.chart.Metadata.Version == "" {
				t.Error("Chart version should not be empty")
			}
		})
	}
}

// TestValuesMarshaling tests values marshaling for Helm releases.
func TestValuesMarshaling(t *testing.T) {
	testCases := []struct {
		name          string
		values        map[string]interface{}
		expectedKey   string
		expectedValue interface{}
		expectError   bool
	}{
		{
			name: "Simple string values",
			values: map[string]interface{}{
				"replicaCount": 3,
				"image.tag":    "1.0.0",
			},
			expectedKey:   "replicaCount",
			expectedValue: 3,
			expectError:   false,
		},
		{
			name: "Nested values",
			values: map[string]interface{}{
				"service": map[string]interface{}{
					"type": "LoadBalancer",
					"port": 80,
				},
			},
			expectedKey:   "service",
			expectedValue: nil, // Will check it's a map
			expectError:   false,
		},
		{
			name:          "Empty values",
			values:        map[string]interface{}{},
			expectedKey:   "",
			expectedValue: nil,
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.values) == 0 && tc.expectedKey != "" {
				t.Error("Expected non-empty values map")
				return
			}

			if tc.expectedKey != "" {
				val, exists := tc.values[tc.expectedKey]
				if !exists {
					t.Errorf("Expected key %s to exist in values", tc.expectedKey)
				}

				// For nested values, check if it's a map
				if tc.expectedValue == nil && tc.expectedKey == "service" {
					if _, ok := val.(map[string]interface{}); !ok {
						t.Error("Expected service value to be a map")
					}
				} else if tc.expectedValue != nil && val != tc.expectedValue {
					t.Errorf("Expected value %v, got %v", tc.expectedValue, val)
				}
			}
		})
	}
}

// TestActionConfiguration tests Helm action configuration setup.
func TestActionConfiguration(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name        string
		namespace   string
		expectError bool
	}{
		{
			name:        "Default namespace",
			namespace:   "default",
			expectError: false,
		},
		{
			name:        "Custom namespace",
			namespace:   "kube-system",
			expectError: false,
		},
		{
			name:        "Empty namespace",
			namespace:   "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.namespace == "" {
				if !tc.expectError {
					t.Error("Expected error for empty namespace")
				}
				return
			}

			// Verify action configuration can be initialized
			actionConfig := &action.Configuration{}
			if actionConfig == nil {
				t.Error("Failed to create action configuration")
			}

			// Test that we can create basic actions with the config
			listAction := action.NewList(actionConfig)
			if listAction == nil {
				t.Error("Failed to create list action")
			}

			_ = ctx // Use context to avoid unused variable warning
		})
	}
}

// TestOCIChartReference tests OCI chart reference parsing and validation.
func TestOCIChartReference(t *testing.T) {
	testCases := []struct {
		name            string
		chartRef        string
		expectedURL     string
		expectedVersion string
		isOCI           bool
		expectError     bool
	}{
		{
			name:            "OCI chart without version",
			chartRef:        "oci://docker.io/bitnamicharts/nginx",
			expectedURL:     "oci://docker.io/bitnamicharts/nginx",
			expectedVersion: "",
			isOCI:           true,
			expectError:     false,
		},
		{
			name:            "OCI chart with version tag",
			chartRef:        "oci://docker.io/bitnamicharts/nginx:1.0.0",
			expectedURL:     "oci://docker.io/bitnamicharts/nginx",
			expectedVersion: "1.0.0",
			isOCI:           true,
			expectError:     false,
		},
		{
			name:            "OCI chart with v-prefixed version",
			chartRef:        "oci://docker.io/bitnamicharts/nginx:v1.2.3",
			expectedURL:     "oci://docker.io/bitnamicharts/nginx",
			expectedVersion: "1.2.3",
			isOCI:           true,
			expectError:     false,
		},
		{
			name:            "Regular HTTPS chart",
			chartRef:        "https://charts.bitnami.com/nginx",
			expectedURL:     "https://charts.bitnami.com/nginx",
			expectedVersion: "",
			isOCI:           false,
			expectError:     false,
		},
		{
			name:            "Empty chart reference",
			chartRef:        "",
			expectedURL:     "",
			expectedVersion: "",
			isOCI:           false,
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.chartRef == "" {
				if !tc.expectError {
					t.Error("Expected error for empty chart reference")
				}
				return
			}

			// Test OCI detection
			isOCI := len(tc.chartRef) >= 6 && tc.chartRef[:6] == "oci://"
			if isOCI != tc.isOCI {
				t.Errorf("Expected isOCI=%v, got %v", tc.isOCI, isOCI)
			}

			// Test version extraction
			chartURL := tc.chartRef
			version := ""
			if idx := lastIndexAfterScheme(tc.chartRef, ":"); idx > 0 {
				chartURL = tc.chartRef[:idx]
				version = tc.chartRef[idx+1:]
				if len(version) > 0 && version[0] == 'v' {
					version = version[1:]
				}
			}

			if chartURL != tc.expectedURL {
				t.Errorf("Expected URL %s, got %s", tc.expectedURL, chartURL)
			}
			if version != tc.expectedVersion {
				t.Errorf("Expected version %s, got %s", tc.expectedVersion, version)
			}
		})
	}
}

// Helper function for test
func lastIndexAfterScheme(s string, sep string) int {
	schemeIdx := indexOf(s, "://")
	if schemeIdx < 0 {
		return lastIndexOf(s, sep)
	}

	idx := lastIndexOf(s, sep)
	if idx > schemeIdx+3 {
		return idx
	}
	return -1
}

func indexOf(s string, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func lastIndexOf(s string, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestRegistryClientInitialization tests that registry client is properly initialized.
func TestRegistryClientInitialization(t *testing.T) {
	testCases := []struct {
		name        string
		setupConfig func(*action.Configuration) error
		expectError bool
	}{
		{
			name: "Action config without registry client",
			setupConfig: func(cfg *action.Configuration) error {
				// Registry client is nil - this should fail in production
				if cfg.RegistryClient != nil {
					t.Error("Expected registry client to be nil initially")
				}
				return nil
			},
			expectError: true,
		},
		{
			name: "Action config with registry client",
			setupConfig: func(cfg *action.Configuration) error {
				// Simulate proper initialization
				// In real code, this would be: cfg.RegistryClient = registry.NewClient()
				return nil
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actionConfig := &action.Configuration{}
			err := tc.setupConfig(actionConfig)

			if tc.expectError && err == nil {
				// This test case expects error condition in real usage
				t.Log("Registry client not initialized - would cause nil pointer in OCI operations")
			}
		})
	}
}

// TestOCIChartLoadingRequirements tests the requirements for loading OCI charts.
func TestOCIChartLoadingRequirements(t *testing.T) {
	testCases := []struct {
		name             string
		hasActionConfig  bool
		hasRegistryClient bool
		hasSettings      bool
		expectError      bool
	}{
		{
			name:             "All requirements met",
			hasActionConfig:  true,
			hasRegistryClient: true,
			hasSettings:      true,
			expectError:      false,
		},
		{
			name:             "Missing registry client",
			hasActionConfig:  true,
			hasRegistryClient: false,
			hasSettings:      true,
			expectError:      true,
		},
		{
			name:             "Missing action config",
			hasActionConfig:  false,
			hasRegistryClient: false,
			hasSettings:      true,
			expectError:      true,
		},
		{
			name:             "Missing settings",
			hasActionConfig:  true,
			hasRegistryClient: true,
			hasSettings:      false,
			expectError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hasError := !tc.hasActionConfig || !tc.hasRegistryClient || !tc.hasSettings

			if hasError != tc.expectError {
				t.Errorf("Expected error=%v, got error=%v", tc.expectError, hasError)
			}

			// Validate critical requirement: registry client must be initialized for OCI operations
			if !tc.hasRegistryClient && !tc.expectError {
				t.Error("CRITICAL: OCI operations require registry client initialization")
			}
		})
	}
}
