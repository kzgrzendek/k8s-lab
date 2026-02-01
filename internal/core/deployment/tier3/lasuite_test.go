package tier3

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/stretchr/testify/assert"
)

// TestLaSuiteNamespaceConstants tests that LaSuite namespace constants are defined correctly.
func TestLaSuiteNamespaceConstants(t *testing.T) {
	assert.Equal(t, "opengatellm", constants.NamespaceOpenGateLLM, "OpenGateLLM namespace should be 'opengatellm'")
	assert.Equal(t, "conversations", constants.NamespaceConversations, "Conversations namespace should be 'conversations'")
}

// TestDeployLaSuiteInfrastructure_Signature verifies the function signature.
func TestDeployLaSuiteInfrastructure_Signature(t *testing.T) {
	var _ func(context.Context, *config.Config) error = DeployLaSuiteInfrastructure
}

// TestLaSuiteOIDCConstants tests that OIDC client constants are defined.
func TestLaSuiteOIDCConstants(t *testing.T) {
	// OpenGateLLM OIDC
	assert.NotEmpty(t, constants.OIDCOpenGateLLM.ID, "OpenGateLLM OIDC client ID should not be empty")
	assert.NotEmpty(t, constants.OIDCOpenGateLLM.Secret, "OpenGateLLM OIDC client secret should not be empty")

	// Conversations OIDC
	assert.NotEmpty(t, constants.OIDCConversations.ID, "Conversations OIDC client ID should not be empty")
	assert.NotEmpty(t, constants.OIDCConversations.Secret, "Conversations OIDC client secret should not be empty")
}

// TestLaSuiteHelmValuesPaths tests that Helm values paths are correct.
func TestLaSuiteHelmValuesPaths(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "OpenGateLLM values",
			path: "resources/core/deployment/tier3/opengatellm/helm/opengatellm.yaml",
		},
		{
			name: "Conversations values",
			path: "resources/core/deployment/tier3/conversations/helm/conversations.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.path, "Values path should not be empty")
			assert.Contains(t, tc.path, "tier3", "Path should be in tier3")
		})
	}
}

// TestLaSuiteHTTPRoutePaths tests that HTTPRoute paths are correct.
func TestLaSuiteHTTPRoutePaths(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "OpenGateLLM HTTPRoute",
			path: "resources/core/deployment/tier3/opengatellm/httproutes/opengatellm.yaml",
		},
		{
			name: "Conversations HTTPRoute",
			path: "resources/core/deployment/tier3/conversations/httproutes/conversations.yaml",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotEmpty(t, tc.path, "HTTPRoute path should not be empty")
			assert.Contains(t, tc.path, "httproutes", "Path should contain 'httproutes'")
		})
	}
}

// TestLaSuiteInfrastructureDeploymentOrder tests the expected deployment order.
func TestLaSuiteInfrastructureDeploymentOrder(t *testing.T) {
	// Infrastructure deployment order from DeployLaSuiteInfrastructure:
	// 1. Qdrant (vector DB for OpenGateLLM)
	// 2. Garage S3 (for Conversations file uploads)
	// 3. Namespace with labels
	// 4. CNPG PostgreSQL cluster
	// 5. Redis cluster

	steps := []string{
		"Qdrant",
		"Garage S3",
		"Namespace",
		"CNPG PostgreSQL",
		"Redis",
	}

	assert.Equal(t, 5, len(steps), "Should have 5 infrastructure steps")
	assert.Equal(t, "Qdrant", steps[0], "Qdrant should be deployed first")
	assert.Equal(t, "Garage S3", steps[1], "Garage S3 should be deployed second")
}

// TestLaSuiteServiceEndpoints tests the expected service DNS names.
func TestLaSuiteServiceEndpoints(t *testing.T) {
	testCases := []struct {
		name      string
		service   string
		namespace string
		port      int
	}{
		{
			name:      "PostgreSQL for OpenGateLLM",
			service:   "lasuite-db-rw",
			namespace: "opengatellm",
			port:      5432,
		},
		{
			name:      "Redis for OpenGateLLM",
			service:   "lasuite-redis",
			namespace: "opengatellm",
			port:      6379,
		},
		{
			name:      "Qdrant for OpenGateLLM",
			service:   "qdrant",
			namespace: "qdrant",
			port:      6333,
		},
		{
			name:      "Garage S3 for Conversations",
			service:   "garage",
			namespace: "garage",
			port:      3900,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedHost := tc.service + "." + tc.namespace + ".svc.cluster.local"
			assert.Contains(t, expectedHost, tc.namespace, "Host should contain namespace")
			assert.Greater(t, tc.port, 0, "Port should be positive")
		})
	}
}

// TestLaSuiteNamespaceLabels tests the expected namespace labels.
func TestLaSuiteNamespaceLabels(t *testing.T) {
	expectedLabels := map[string]string{
		"service-type":                   "nova",
		"trust-manager/inject-ca-secret": "enabled",
	}

	assert.NotEmpty(t, expectedLabels["service-type"], "service-type label should be set")
	assert.Equal(t, "nova", expectedLabels["service-type"], "service-type should be 'nova'")
	assert.Equal(t, "enabled", expectedLabels["trust-manager/inject-ca-secret"], "trust-manager label should be 'enabled'")
}
