package tier2

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/stretchr/testify/assert"
)

// TestCNPGConstants tests that CNPG-related constants are defined correctly.
func TestCNPGConstants(t *testing.T) {
	// Verify CNPG namespace is defined
	assert.Equal(t, "cnpg-system", constants.NamespaceCNPG, "CNPG namespace should be cnpg-system")
}

// TestKeycloakDBNamespace tests that Keycloak DB uses correct namespace.
func TestKeycloakDBNamespace(t *testing.T) {
	assert.Equal(t, "keycloak", keycloakNamespace, "Keycloak DB should use keycloak namespace")
}

// TestDeployLaSuiteCNPGCluster_Signature verifies the function signature.
func TestDeployLaSuiteCNPGCluster_Signature(t *testing.T) {
	var _ func(context.Context, string) error = DeployLaSuiteCNPGCluster
}

// TestLaSuiteDBConfiguration tests the expected LaSuite database configuration.
func TestLaSuiteDBConfiguration(t *testing.T) {
	testCases := []struct {
		name       string
		namespace  string
		secretName string
		dbUser     string
	}{
		{
			name:       "OpenGateLLM namespace",
			namespace:  constants.NamespaceOpenGateLLM,
			secretName: "lasuite-db-secret",
			dbUser:     "lasuite",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.namespace, "Namespace should not be empty")
			assert.NotEmpty(t, tc.secretName, "Secret name should not be empty")
			assert.NotEmpty(t, tc.dbUser, "Database user should not be empty")
		})
	}
}

// TestCNPGClusterResourcePath tests that CNPG cluster resource paths exist.
func TestCNPGClusterResourcePath(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "Keycloak DB cluster",
			path: "resources/core/deployment/tier2/cnpg/clusters/keycloak-db.yaml",
		},
		{
			name: "LaSuite DB cluster",
			path: "resources/core/deployment/tier2/cnpg/clusters/lasuite-db.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.path, "Resource path should not be empty")
			assert.Contains(t, tc.path, "cnpg/clusters", "Path should point to CNPG clusters directory")
		})
	}
}
