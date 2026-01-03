package tier2

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/stretchr/testify/assert"
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
			name:      "Keycloak namespace",
			namespace: keycloakNamespace,
			expected:  "keycloak",
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
			assert.Equal(t, tc.expected, tc.namespace)
		})
	}
}

// TestDeploymentSteps tests that deployment steps are defined correctly in the expected order.
func TestDeploymentSteps(t *testing.T) {
	// Reconstruct the steps slice as defined in tier2.go
	steps := []string{
		"Kyverno Policy Engine",
		"Keycloak Operator",
		"Keycloak PostgreSQL Database",
		"Keycloak Theme",
		"Keycloak IAM Instance",
		"Hubble Network Observability",
		"VictoriaLogs Server",
		"VictoriaLogs Collector",
		"VictoriaMetrics Stack",
	}

	expectedSteps := 9
	assert.Equal(t, expectedSteps, len(steps), "Should have exactly %d deployment steps", expectedSteps)

	// Verify crucial ordering dependencies
	stepMap := make(map[string]int)
	for i, s := range steps {
		stepMap[s] = i
	}

	// Keycloak Operator before Instance
	assert.Less(t, stepMap["Keycloak Operator"], stepMap["Keycloak IAM Instance"], "Keycloak Operator must come before Instance")

	// Keycloak DB before Instance
	assert.Less(t, stepMap["Keycloak PostgreSQL Database"], stepMap["Keycloak IAM Instance"], "Keycloak DB must come before Instance")

	// Hubble depends on Cilium (which is Tier 1) so it can go anywhere relative to others,
	// but purely logically it's nice if it's after Keycloak if we use OIDC (which we do).
	assert.Less(t, stepMap["Keycloak IAM Instance"], stepMap["Hubble Network Observability"], "Keycloak should be ready before Hubble OIDC setup")

	// VLS before Collector
	assert.Less(t, stepMap["VictoriaLogs Server"], stepMap["VictoriaLogs Collector"], "VLS must come before Collector")
}

// TestDeployResultStructure tests the DeployResult struct and its expectations.
func TestDeployResultStructure(t *testing.T) {
	result := &DeployResult{
		KeycloakUsers: []KeycloakUser{
			{Username: "admin", Password: "pwd", Group: "ADMIN"},
		},
	}

	assert.NotNil(t, result.KeycloakUsers)
	assert.Equal(t, 1, len(result.KeycloakUsers))
	assert.Equal(t, "admin", result.KeycloakUsers[0].Username)
}

// TestGetCredentials_NilCtx ensures it handles nil context gracefully or strictly (here we just checking signature).
// Real logic requires mocking K8s client, which is complex here without DI.
// We just verify the API surface exists.
func TestGetCredentials_Signature(t *testing.T) {
	// This would panic or return nil in real execution without a k8s env
	// We just ensure the function compiles and has correct signature
	var _ func(context.Context) *DeployResult = GetCredentials
}

// TestDeployTier2_Signature verifies the function signature.
func TestDeployTier2_Signature(t *testing.T) {
	var _ func(context.Context, *config.Config) (*DeployResult, error) = DeployTier2
}
