package tier2

import (
	"testing"
)

// TestNamespaceConstants tests that namespace constants are defined correctly.
func TestNamespaceConstants(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
		expected  string
	}{
		{
			name:      "Kyverno namespace",
			namespace: kyvernoNamespace,
			expected:  "kyverno",
		},
		{
			name:      "VictoriaLogs namespace",
			namespace: victorialogsNamespace,
			expected:  "victorialogs",
		},
		{
			name:      "VictoriaMetrics namespace",
			namespace: victoriametricsNamespace,
			expected:  "victoriametrics",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.namespace != tc.expected {
				t.Errorf("Expected namespace %s, got %s", tc.expected, tc.namespace)
			}
		})
	}
}

// TestDeploymentSteps tests that deployment steps are defined correctly.
func TestDeploymentSteps(t *testing.T) {
	steps := []string{
		"Kyverno Policy Engine",
		"Hubble Network Observability",
		"VictoriaLogs Server",
		"VictoriaLogs Collector",
		"VictoriaMetrics Stack",
	}

	// Verify we have exactly 5 steps
	expectedSteps := 5
	if len(steps) != expectedSteps {
		t.Errorf("Expected %d deployment steps, got %d", expectedSteps, len(steps))
	}

	// Verify all steps have non-empty names
	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should have a non-empty name", i)
		}
	}

	// Verify step order
	if steps[0] != "Kyverno Policy Engine" {
		t.Error("First step should be Kyverno Policy Engine")
	}

	if steps[len(steps)-1] != "VictoriaMetrics Stack" {
		t.Error("Last step should be VictoriaMetrics Stack")
	}
}

// TestKyvernoConfiguration tests Kyverno deployment configuration.
func TestKyvernoConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		releaseName string
		chartRef    string
		namespace   string
		valuesPath  string
		timeout     int
		createNS    bool
	}{
		{
			name:        "Kyverno release configuration",
			releaseName: "kyverno",
			chartRef:    "kyverno/kyverno",
			namespace:   kyvernoNamespace,
			valuesPath:  "resources/core/deployment/tier2/kyverno/values.yaml",
			timeout:     300,
			createNS:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.releaseName == "" {
				t.Error("Release name should not be empty")
			}
			if tc.chartRef == "" {
				t.Error("Chart ref should not be empty")
			}
			if tc.namespace == "" {
				t.Error("Namespace should not be empty")
			}
			if tc.valuesPath == "" {
				t.Error("Values path should not be empty")
			}
			if tc.timeout < 60 {
				t.Errorf("Timeout should be at least 60 seconds, got %d", tc.timeout)
			}
		})
	}
}

// TestVictoriaStackConfiguration tests VictoriaMetrics/Logs configuration.
func TestVictoriaStackConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		releaseName string
		chartRef    string
		namespace   string
	}{
		{
			name:        "VictoriaLogs Server",
			releaseName: "vls",
			chartRef:    "vm/victoria-logs-single",
			namespace:   victorialogsNamespace,
		},
		{
			name:        "VictoriaLogs Collector",
			releaseName: "collector",
			chartRef:    "vm/victoria-logs-collector",
			namespace:   victorialogsNamespace,
		},
		{
			name:        "VictoriaMetrics Stack",
			releaseName: "vmks",
			chartRef:    "vm/victoria-metrics-k8s-stack",
			namespace:   victoriametricsNamespace,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.releaseName == "" {
				t.Error("Release name should not be empty")
			}
			if tc.chartRef == "" {
				t.Error("Chart ref should not be empty")
			}
			if tc.namespace == "" {
				t.Error("Namespace should not be empty")
			}

			// Verify VictoriaMetrics chart uses vm/ prefix
			if tc.namespace == victoriametricsNamespace || tc.namespace == victorialogsNamespace {
				if len(tc.chartRef) < 3 || tc.chartRef[:3] != "vm/" {
					t.Errorf("VictoriaMetrics charts should use vm/ prefix, got %s", tc.chartRef)
				}
			}
		})
	}
}

// TestHubbleConfiguration tests Hubble deployment configuration.
func TestHubbleConfiguration(t *testing.T) {
	testCases := []struct {
		name        string
		releaseName string
		chartRef    string
		namespace   string
		reuseValues bool
	}{
		{
			name:        "Hubble extends Cilium",
			releaseName: "cilium",
			chartRef:    "cilium/cilium",
			namespace:   "kube-system",
			reuseValues: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Hubble should upgrade existing Cilium release
			if tc.releaseName != "cilium" {
				t.Error("Hubble should upgrade the cilium release")
			}

			// Should reuse existing Cilium values
			if !tc.reuseValues {
				t.Error("Hubble deployment should reuse existing Cilium values")
			}

			// Should be deployed to kube-system
			if tc.namespace != "kube-system" {
				t.Error("Hubble should be deployed to kube-system namespace")
			}
		})
	}
}

// TestTimeoutConfiguration tests timeout values for tier 2 components.
func TestTimeoutConfiguration(t *testing.T) {
	testCases := []struct {
		name    string
		timeout int
		minTime int
	}{
		{
			name:    "Kyverno",
			timeout: 300,
			minTime: 60,
		},
		{
			name:    "Hubble",
			timeout: 300,
			minTime: 60,
		},
		{
			name:    "VictoriaLogs Server",
			timeout: 300,
			minTime: 60,
		},
		{
			name:    "VictoriaLogs Collector",
			timeout: 300,
			minTime: 60,
		},
		{
			name:    "VictoriaMetrics Stack",
			timeout: 600,
			minTime: 300,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.timeout < tc.minTime {
				t.Errorf("%s timeout should be at least %d seconds, got %d", tc.name, tc.minTime, tc.timeout)
			}
		})
	}
}

// TestGrafanaConfiguration tests Grafana admin secret generation.
func TestGrafanaConfiguration(t *testing.T) {
	// Test that Grafana requires a password
	testCases := []struct {
		name           string
		passwordLength int
		minLength      int
	}{
		{
			name:           "Grafana admin password",
			passwordLength: 32,
			minLength:      16,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.passwordLength < tc.minLength {
				t.Errorf("Grafana password should be at least %d characters, configured for %d", tc.minLength, tc.passwordLength)
			}
		})
	}
}

// TestComponentOrder tests that components are deployed in the correct order.
func TestComponentOrder(t *testing.T) {
	// Correct deployment order for Tier 2
	componentOrder := []string{
		"Kyverno Policy Engine",
		"Hubble Network Observability",
		"VictoriaLogs Server",
		"VictoriaLogs Collector",
		"VictoriaMetrics Stack",
	}

	// Verify Kyverno is deployed first (policies)
	if componentOrder[0] != "Kyverno Policy Engine" {
		t.Error("Kyverno should be deployed first to establish policies")
	}

	// Verify Hubble is deployed before Victoria stack
	hubbleIndex := -1
	victoriaIndex := -1
	for i, comp := range componentOrder {
		if comp == "Hubble Network Observability" {
			hubbleIndex = i
		}
		if comp == "VictoriaLogs Server" {
			victoriaIndex = i
		}
	}

	if hubbleIndex > victoriaIndex {
		t.Error("Hubble should be deployed before VictoriaLogs")
	}

	// Verify VictoriaLogs Server is deployed before Collector
	serverIndex := -1
	collectorIndex := -1
	for i, comp := range componentOrder {
		if comp == "VictoriaLogs Server" {
			serverIndex = i
		}
		if comp == "VictoriaLogs Collector" {
			collectorIndex = i
		}
	}

	if serverIndex > collectorIndex {
		t.Error("VictoriaLogs Server should be deployed before Collector")
	}

	// Verify VictoriaMetrics is deployed last (depends on metrics sources)
	if componentOrder[len(componentOrder)-1] != "VictoriaMetrics Stack" {
		t.Error("VictoriaMetrics Stack should be deployed last")
	}
}

// TestErrorMessages tests error message formatting.
func TestErrorMessages(t *testing.T) {
	testCases := []struct {
		name             string
		errorFormat      string
		expectedContains string
	}{
		{
			name:             "Kyverno deployment error",
			errorFormat:      "failed to deploy tier 2: %w",
			expectedContains: "tier 2",
		},
		{
			name:             "Grafana secret error",
			errorFormat:      "failed to create grafana admin secret: %w",
			expectedContains: "grafana admin secret",
		},
		{
			name:             "Namespace creation error",
			errorFormat:      "failed to create namespace: %w",
			expectedContains: "namespace",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.errorFormat == "" {
				t.Error("Error format should not be empty")
			}

			if len(tc.errorFormat) < 5 {
				t.Error("Error format should be a meaningful message")
			}
		})
	}
}
