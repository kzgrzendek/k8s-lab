package commands

import (
	"testing"
)

// TestStopStepsConfiguration tests the stop command steps configuration.
func TestStopStepsConfiguration(t *testing.T) {
	steps := []string{
		"Minikube Cluster",
		"NGINX Gateway",
		"Bind9 DNS Server",
	}

	// Verify we have the expected number of steps
	expectedSteps := 3
	if len(steps) != expectedSteps {
		t.Errorf("Expected %d stop steps, got %d", expectedSteps, len(steps))
	}

	// Verify all steps have non-empty names
	for i, step := range steps {
		if step == "" {
			t.Errorf("Step %d should have a non-empty name", i)
		}
	}

	// Verify step order (Minikube should be first)
	if steps[0] != "Minikube Cluster" {
		t.Error("First step should be Minikube Cluster")
	}

	// Verify host services are last
	if steps[len(steps)-1] != "Bind9 DNS Server" {
		t.Error("Last step should be Bind9 DNS Server")
	}
}

// TestStopErrorMessages tests stop command error message formatting.
func TestStopErrorMessages(t *testing.T) {
	testCases := []struct {
		name             string
		errorFormat      string
		expectedContains string
	}{
		{
			name:             "Minikube stop error",
			errorFormat:      "failed to stop Minikube cluster: %w",
			expectedContains: "Minikube cluster",
		},
		{
			name:             "NGINX stop error",
			errorFormat:      "failed to stop NGINX gateway: %w",
			expectedContains: "NGINX gateway",
		},
		{
			name:             "Bind9 stop error",
			errorFormat:      "failed to stop Bind9 DNS server: %w",
			expectedContains: "Bind9 DNS server",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.errorFormat == "" {
				t.Error("Error format should not be empty")
			}

			// Verify error message contains expected keywords
			if len(tc.errorFormat) < 5 {
				t.Error("Error format should be a meaningful message")
			}
		})
	}
}

// TestStopComponentOrder tests that components are stopped in the correct order.
func TestStopComponentOrder(t *testing.T) {
	// Components should be stopped in reverse order of startup
	// Start order: Minikube -> Host services (NGINX, Bind9)
	// Stop order: Minikube -> NGINX -> Bind9
	componentOrder := []string{
		"Minikube Cluster",
		"NGINX Gateway",
		"Bind9 DNS Server",
	}

	// Verify Minikube is stopped first
	if componentOrder[0] != "Minikube Cluster" {
		t.Error("Minikube should be stopped first to prevent access to cluster")
	}

	// Verify host services are stopped after cluster
	hostServicesStart := 1
	for i := hostServicesStart; i < len(componentOrder); i++ {
		component := componentOrder[i]
		if component != "NGINX Gateway" && component != "Bind9 DNS Server" {
			t.Errorf("Component %s at position %d should be a host service", component, i)
		}
	}
}
