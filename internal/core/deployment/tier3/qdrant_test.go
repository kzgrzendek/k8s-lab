package tier3

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/stretchr/testify/assert"
)

// TestQdrantConstants tests that Qdrant-related constants are defined correctly.
func TestQdrantConstants(t *testing.T) {
	assert.Equal(t, "qdrant", constants.NamespaceQdrant, "Qdrant namespace should be 'qdrant'")
	assert.Contains(t, constants.HelmRepoQdrant, "qdrant.github.io", "Qdrant Helm repo should be from qdrant.github.io")
}

// TestDeployQdrant_Signature verifies the function signature.
func TestDeployQdrant_Signature(t *testing.T) {
	var _ func(context.Context, *config.Config) error = DeployQdrant
}

// TestQdrantValuesPath tests that the values file path is correct.
func TestQdrantValuesPath(t *testing.T) {
	expectedPath := "resources/core/deployment/tier3/qdrant/helm/qdrant.yaml"
	assert.NotEmpty(t, expectedPath, "Values path should not be empty")
	assert.Contains(t, expectedPath, "qdrant", "Path should contain 'qdrant'")
}

// TestQdrantServiceEndpoint tests the expected Qdrant service endpoint.
func TestQdrantServiceEndpoint(t *testing.T) {
	// Expected service DNS name within cluster
	endpoint := "qdrant.qdrant.svc.cluster.local"
	grpcPort := 6334
	httpPort := 6333

	assert.Contains(t, endpoint, "qdrant.qdrant", "Endpoint should be in qdrant namespace")
	assert.Equal(t, 6333, httpPort, "HTTP API port should be 6333")
	assert.Equal(t, 6334, grpcPort, "gRPC port should be 6334")
}

// TestQdrantInfraRequirement tests that Qdrant is conditional on LaSuite profile.
func TestQdrantInfraRequirement(t *testing.T) {
	testCases := []struct {
		name         string
		appProfiles  []config.AppProfileType
		expectQdrant bool
	}{
		{
			name:         "No profiles - Qdrant not needed",
			appProfiles:  []config.AppProfileType{},
			expectQdrant: false,
		},
		{
			name:         "LaSuite profile - Qdrant needed",
			appProfiles:  []config.AppProfileType{config.AppProfileLaSuite},
			expectQdrant: true,
		},
		{
			name:         "OpenWebUI only - Qdrant not needed",
			appProfiles:  []config.AppProfileType{config.AppProfileOpenWebUI},
			expectQdrant: false,
		},
		{
			name:         "Multiple profiles with LaSuite - Qdrant needed",
			appProfiles:  []config.AppProfileType{config.AppProfileOpenWebUI, config.AppProfileLaSuite},
			expectQdrant: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				AppProfiles: config.AppProfilesConfig{
					ActiveProfiles: tc.appProfiles,
				},
			}

			req := cfg.GetInfraRequirements()
			assert.Equal(t, tc.expectQdrant, req.Qdrant, "Qdrant requirement mismatch")
		})
	}
}
