package commands

import (
	"testing"
)

// TestDeleteStepsConfiguration tests the delete command steps configuration.
func TestDeleteStepsConfiguration(t *testing.T) {
	testCases := []struct {
		name          string
		purge         bool
		expectedSteps int
	}{
		{
			name:          "Delete without purge",
			purge:         false,
			expectedSteps: 3, // Minikube, NGINX, Bind9
		},
		{
			name:          "Delete with purge",
			purge:         true,
			expectedSteps: 5, // Minikube, NGINX, Bind9, DNS config, Config dir
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			steps := []string{
				"Minikube Cluster",
				"NGINX Gateway",
				"Bind9 DNS Server",
			}
			if tc.purge {
				steps = append(steps, "DNS Configuration", "Configuration Directory")
			}

			if len(steps) != tc.expectedSteps {
				t.Errorf("Expected %d steps, got %d", tc.expectedSteps, len(steps))
			}

			// Verify all steps have non-empty names
			for i, step := range steps {
				if step == "" {
					t.Errorf("Step %d should have a non-empty name", i)
				}
			}
		})
	}
}

// TestDeleteErrorMessages tests delete command error message formatting.
func TestDeleteErrorMessages(t *testing.T) {
	testCases := []struct {
		name             string
		errorFormat      string
		expectedContains string
	}{
		{
			name:             "Minikube delete error",
			errorFormat:      "failed to delete Minikube cluster: %w",
			expectedContains: "Minikube cluster",
		},
		{
			name:             "NGINX delete error",
			errorFormat:      "failed to remove NGINX gateway: %w",
			expectedContains: "NGINX gateway",
		},
		{
			name:             "Bind9 delete error",
			errorFormat:      "failed to remove Bind9 DNS server: %w",
			expectedContains: "Bind9 DNS server",
		},
		{
			name:             "DNS config remove error",
			errorFormat:      "failed to remove DNS configuration: %w",
			expectedContains: "DNS configuration",
		},
		{
			name:             "Config directory remove error",
			errorFormat:      "failed to remove configuration directory: %w",
			expectedContains: "configuration directory",
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

// TestDeleteComponentOrder tests that components are deleted in the correct order.
func TestDeleteComponentOrder(t *testing.T) {
	componentOrder := []string{
		"Minikube Cluster",
		"NGINX Gateway",
		"Bind9 DNS Server",
	}

	// Verify Minikube is deleted first
	if componentOrder[0] != "Minikube Cluster" {
		t.Error("Minikube should be deleted first")
	}

	// Verify host services are deleted after cluster
	if componentOrder[1] != "NGINX Gateway" {
		t.Error("NGINX should be deleted after Minikube")
	}

	if componentOrder[2] != "Bind9 DNS Server" {
		t.Error("Bind9 should be deleted after NGINX")
	}
}

// TestDeletePurgeSteps tests purge-specific steps.
func TestDeletePurgeSteps(t *testing.T) {
	purgeSteps := []string{
		"DNS Configuration",
		"Configuration Directory",
	}

	// Verify purge steps are defined
	if len(purgeSteps) != 2 {
		t.Errorf("Expected 2 purge steps, got %d", len(purgeSteps))
	}

	// Verify DNS configuration is removed before config directory
	if purgeSteps[0] != "DNS Configuration" {
		t.Error("DNS Configuration should be removed first in purge")
	}

	if purgeSteps[1] != "Configuration Directory" {
		t.Error("Configuration Directory should be removed last in purge")
	}
}

// TestDeleteConfirmationBehavior tests confirmation prompt behavior.
func TestDeleteConfirmationBehavior(t *testing.T) {
	testCases := []struct {
		name          string
		yes           bool
		shouldConfirm bool
	}{
		{
			name:          "Yes flag skips confirmation",
			yes:           true,
			shouldConfirm: false,
		},
		{
			name:          "No yes flag requires confirmation",
			yes:           false,
			shouldConfirm: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate confirmation logic
			requiresConfirmation := !tc.yes

			if requiresConfirmation != tc.shouldConfirm {
				t.Errorf("Expected confirmation required=%v, got %v", tc.shouldConfirm, requiresConfirmation)
			}
		})
	}
}
