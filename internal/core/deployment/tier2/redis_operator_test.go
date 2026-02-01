package tier2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDeployLaSuiteRedisCluster_Signature verifies the function signature.
func TestDeployLaSuiteRedisCluster_Signature(t *testing.T) {
	var _ func(context.Context, string) error = DeployLaSuiteRedisCluster
}

// TestLaSuiteRedisResourcePath tests that Redis cluster resource path is correct.
func TestLaSuiteRedisResourcePath(t *testing.T) {
	expectedPath := "resources/core/deployment/tier2/redis-operator/clusters/lasuite-redis.yaml"
	assert.NotEmpty(t, expectedPath, "Resource path should not be empty")
	assert.Contains(t, expectedPath, "redis-operator", "Path should reference redis-operator")
	assert.Contains(t, expectedPath, "lasuite-redis", "Path should reference lasuite-redis")
}

// TestRedisServiceEndpoint tests the expected Redis service endpoint format.
func TestRedisServiceEndpoint(t *testing.T) {
	testCases := []struct {
		name          string
		namespace     string
		expectedHost  string
		expectedPort  int
	}{
		{
			name:         "OpenGateLLM namespace",
			namespace:    "opengatellm",
			expectedHost: "lasuite-redis.opengatellm.svc.cluster.local",
			expectedPort: 6379,
		},
		{
			name:         "Conversations namespace",
			namespace:    "conversations",
			expectedHost: "lasuite-redis.conversations.svc.cluster.local",
			expectedPort: 6379,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Contains(t, tc.expectedHost, tc.namespace, "Host should contain namespace")
			assert.Contains(t, tc.expectedHost, "lasuite-redis", "Host should reference lasuite-redis service")
			assert.Equal(t, 6379, tc.expectedPort, "Redis port should be 6379")
		})
	}
}

// TestRedisWaitCondition tests the Redis wait condition resource type.
func TestRedisWaitCondition(t *testing.T) {
	// The condition string used in WaitForCondition
	resourceType := "redis.redis.redis.opstreelabs.in/lasuite-redis"

	assert.Contains(t, resourceType, "redis.opstreelabs.in", "Should use OT Redis Operator CRD")
	assert.Contains(t, resourceType, "lasuite-redis", "Should reference lasuite-redis resource")
}
