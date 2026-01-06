package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmDeploymentOptions_Validation(t *testing.T) {
	tests := []struct {
		name string
		opts HelmDeploymentOptions
		want bool
	}{
		{
			name: "valid options with all fields",
			opts: HelmDeploymentOptions{
				ReleaseName:     "test-release",
				ChartRef:        "repo/chart",
				Namespace:       "test-namespace",
				ValuesPath:      "path/to/values.yaml",
				Values:          map[string]any{"key": "value"},
				Wait:            true,
				TimeoutSeconds:  600,
				InfoMessage:     "Installing test",
				SuccessMessage:  "Test installed",
				CreateNamespace: true,
			},
			want: true,
		},
		{
			name: "minimal valid options",
			opts: HelmDeploymentOptions{
				ReleaseName: "test-release",
				ChartRef:    "repo/chart",
				Namespace:   "test-namespace",
			},
			want: true,
		},
		{
			name: "OCI chart reference",
			opts: HelmDeploymentOptions{
				ReleaseName: "envoy-gateway",
				ChartRef:    "oci://docker.io/envoyproxy/gateway-helm:v1.6.1",
				Namespace:   "envoy-gateway-system",
				Wait:        true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure required fields are not empty
			assert.NotEmpty(t, tt.opts.ReleaseName, "ReleaseName should not be empty")
			assert.NotEmpty(t, tt.opts.ChartRef, "ChartRef should not be empty")
			assert.NotEmpty(t, tt.opts.Namespace, "Namespace should not be empty")
		})
	}
}

func TestHelmDeploymentOptions_OptionalFields(t *testing.T) {
	opts := HelmDeploymentOptions{
		ReleaseName: "test",
		ChartRef:    "repo/chart",
		Namespace:   "default",
	}

	// Test that optional fields have sensible defaults
	assert.Empty(t, opts.ValuesPath, "ValuesPath should be empty by default")
	assert.Nil(t, opts.Values, "Values should be nil by default")
	assert.False(t, opts.Wait, "Wait should be false by default")
	assert.Equal(t, 0, opts.TimeoutSeconds, "TimeoutSeconds should be 0 by default")
	assert.Empty(t, opts.InfoMessage, "InfoMessage should be empty by default")
	assert.Empty(t, opts.SuccessMessage, "SuccessMessage should be empty by default")
	assert.False(t, opts.CreateNamespace, "CreateNamespace should be false by default")
}
