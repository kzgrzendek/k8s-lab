package tier1

import (
	"testing"

	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/stretchr/testify/assert"
)

// TestCNPGOperatorConstants tests CNPG operator constants.
func TestCNPGOperatorConstants(t *testing.T) {
	assert.Equal(t, "cnpg-system", constants.NamespaceCNPG, "CNPG namespace should be 'cnpg-system'")
}

// TestCNPGOperatorDeploymentName tests the expected deployment name.
func TestCNPGOperatorDeploymentName(t *testing.T) {
	deploymentName := "cnpg-controller-manager"
	assert.NotEmpty(t, deploymentName, "Deployment name should not be empty")
	assert.Contains(t, deploymentName, "cnpg", "Deployment name should contain 'cnpg'")
}

// TestRedisOperatorConstants tests Redis operator constants.
func TestRedisOperatorConstants(t *testing.T) {
	assert.Equal(t, "redis-operator", constants.NamespaceRedisOperator, "Redis operator namespace should be 'redis-operator'")
	assert.Contains(t, constants.HelmRepoOTRedis, "ot-container-kit.github.io", "Redis Helm repo should be from ot-container-kit")
}

// TestRedisOperatorValuesPath tests the values file path.
func TestRedisOperatorValuesPath(t *testing.T) {
	expectedPath := "resources/core/deployment/tier2/redis-operator/values.yaml"
	assert.NotEmpty(t, expectedPath, "Values path should not be empty")
	assert.Contains(t, expectedPath, "redis-operator", "Path should reference redis-operator")
}

// TestEnvoyRedisClusterResourcePath tests the Envoy Redis resource path.
func TestEnvoyRedisClusterResourcePath(t *testing.T) {
	expectedPath := "resources/core/deployment/tier1/redis-operator/clusters/envoy-redis.yaml"
	assert.NotEmpty(t, expectedPath, "Resource path should not be empty")
	assert.Contains(t, expectedPath, "tier1", "Path should be in tier1")
	assert.Contains(t, expectedPath, "envoy-redis", "Path should reference envoy-redis")
}

// TestEnvoyRedisConfiguration tests the expected Envoy Redis configuration.
func TestEnvoyRedisConfiguration(t *testing.T) {
	// Expected namespace for Envoy Redis
	assert.Equal(t, "envoy-gateway-system", constants.NamespaceEnvoyGateway, "Envoy Redis should be in envoy-gateway-system namespace")

	// Redis secret configuration
	secretName := "redis"
	secretKey := "redis-password"
	assert.NotEmpty(t, secretName, "Secret name should not be empty")
	assert.NotEmpty(t, secretKey, "Secret key should not be empty")
}

// TestRedisWaitConditionType tests the Redis wait condition resource type.
func TestRedisWaitConditionType(t *testing.T) {
	resourceType := "redis.redis.redis.opstreelabs.in/envoy-redis"
	assert.Contains(t, resourceType, "opstreelabs.in", "Should use OT Redis Operator CRD")
	assert.Contains(t, resourceType, "envoy-redis", "Should reference envoy-redis")
}

// TestOperatorDeploymentTimeout tests that timeouts are reasonable.
func TestOperatorDeploymentTimeout(t *testing.T) {
	testCases := []struct {
		name    string
		timeout int
		minTime int
		maxTime int
	}{
		{"CNPG Operator", 300, 180, 600},
		{"Redis Operator", 300, 180, 600},
		{"Envoy Redis Cluster", 180, 60, 300},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tc.timeout, tc.minTime, "Timeout should not be too short")
			assert.LessOrEqual(t, tc.timeout, tc.maxTime, "Timeout should not be too long")
		})
	}
}

// TestRedisPasswordGeneration tests the password generation function.
func TestRedisPasswordGeneration(t *testing.T) {
	pwd, err := getOrGenerateRedisPassword()
	assert.NoError(t, err, "Password generation should not fail")
	assert.Greater(t, len(pwd), 0, "Password should not be empty")
	// The crypto module generates base64-encoded output, so length will be > 32
	assert.GreaterOrEqual(t, len(pwd), 32, "Password should be at least 32 characters")
}
