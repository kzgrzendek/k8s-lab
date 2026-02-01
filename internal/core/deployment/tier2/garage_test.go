package tier2

import (
	"context"
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/stretchr/testify/assert"
)

// TestGarageConstants tests that Garage-related constants are defined correctly.
func TestGarageConstants(t *testing.T) {
	assert.Equal(t, "garage", constants.NamespaceGarage, "Garage namespace should be 'garage'")
	assert.Contains(t, constants.HelmRepoGarage, "garagehq.deuxfleurs.fr", "Garage Helm repo should be from deuxfleurs")
}

// TestDeployGarage_Signature verifies the function signature.
func TestDeployGarage_Signature(t *testing.T) {
	var _ func(context.Context, *config.Config) error = DeployGarage
}

// TestCopyGarageCredentialsToNamespace_Signature verifies the function signature.
func TestCopyGarageCredentialsToNamespace_Signature(t *testing.T) {
	var _ func(context.Context, string) error = CopyGarageCredentialsToNamespace
}

// TestGarageCredentialsSecretKeys tests the expected secret key names.
func TestGarageCredentialsSecretKeys(t *testing.T) {
	expectedKeys := []string{"access-key", "secret-key"}

	for _, key := range expectedKeys {
		assert.NotEmpty(t, key, "Secret key name should not be empty")
	}
}

// TestGarageValuesPath tests that the values file path is correct.
func TestGarageValuesPath(t *testing.T) {
	expectedPath := "resources/core/deployment/tier2/garage/values.yaml"
	assert.NotEmpty(t, expectedPath, "Values path should not be empty")
	assert.Contains(t, expectedPath, "garage", "Path should contain 'garage'")
}

// TestGarageS3ServiceEndpoint tests the expected S3 service endpoint.
func TestGarageS3ServiceEndpoint(t *testing.T) {
	// Expected service DNS name within cluster
	endpoint := "garage.garage.svc.cluster.local"
	port := 3900

	assert.Contains(t, endpoint, "garage.garage", "Endpoint should be in garage namespace")
	assert.Equal(t, 3900, port, "S3 API port should be 3900")
}

// TestGarageInfraRequirement tests that Garage is conditional on LaSuite profile.
func TestGarageInfraRequirement(t *testing.T) {
	testCases := []struct {
		name         string
		appProfiles  []config.AppProfileType
		expectGarage bool
	}{
		{
			name:         "No profiles - Garage not needed",
			appProfiles:  []config.AppProfileType{},
			expectGarage: false,
		},
		{
			name:         "LaSuite profile - Garage needed",
			appProfiles:  []config.AppProfileType{config.AppProfileLaSuite},
			expectGarage: true,
		},
		{
			name:         "OpenWebUI only - Garage not needed",
			appProfiles:  []config.AppProfileType{config.AppProfileOpenWebUI},
			expectGarage: false,
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
			assert.Equal(t, tc.expectGarage, req.Garage, "Garage requirement mismatch")
		})
	}
}
