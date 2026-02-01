package shared

import (
	"testing"

	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/stretchr/testify/assert"
)

func TestResourcesAsHelmValues(t *testing.T) {
	tests := []struct {
		name     string
		input    config.ResourceRequirements
		expected map[string]any
	}{
		{
			name:     "empty resources",
			input:    config.ResourceRequirements{},
			expected: map[string]any{},
		},
		{
			name: "requests only",
			input: config.ResourceRequirements{
				Requests: config.ResourceSpec{CPU: "100m", Memory: "256Mi"},
			},
			expected: map[string]any{
				"requests": map[string]any{
					"cpu":    "100m",
					"memory": "256Mi",
				},
			},
		},
		{
			name: "limits only",
			input: config.ResourceRequirements{
				Limits: config.ResourceSpec{CPU: "500m", Memory: "1Gi"},
			},
			expected: map[string]any{
				"limits": map[string]any{
					"cpu":    "500m",
					"memory": "1Gi",
				},
			},
		},
		{
			name: "full resources",
			input: config.ResourceRequirements{
				Requests: config.ResourceSpec{CPU: "100m", Memory: "256Mi"},
				Limits:   config.ResourceSpec{CPU: "500m", Memory: "1Gi"},
			},
			expected: map[string]any{
				"requests": map[string]any{
					"cpu":    "100m",
					"memory": "256Mi",
				},
				"limits": map[string]any{
					"cpu":    "500m",
					"memory": "1Gi",
				},
			},
		},
		{
			name: "partial CPU only",
			input: config.ResourceRequirements{
				Requests: config.ResourceSpec{CPU: "100m"},
				Limits:   config.ResourceSpec{CPU: "500m"},
			},
			expected: map[string]any{
				"requests": map[string]any{
					"cpu": "100m",
				},
				"limits": map[string]any{
					"cpu": "500m",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResourcesAsHelmValues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMergeResourcesIntoValues(t *testing.T) {
	resources := config.ResourceRequirements{
		Requests: config.ResourceSpec{CPU: "100m", Memory: "256Mi"},
		Limits:   config.ResourceSpec{CPU: "500m", Memory: "1Gi"},
	}

	tests := []struct {
		name     string
		values   map[string]any
		path     string
		expected map[string]any
	}{
		{
			name:   "empty path adds to resources key",
			values: map[string]any{},
			path:   "",
			expected: map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
					"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
				},
			},
		},
		{
			name:   "simple path",
			values: map[string]any{},
			path:   "controller.resources",
			expected: map[string]any{
				"controller": map[string]any{
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
						"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
					},
				},
			},
		},
		{
			name: "merge with existing values",
			values: map[string]any{
				"controller": map[string]any{
					"replicas": 2,
				},
			},
			path: "controller.resources",
			expected: map[string]any{
				"controller": map[string]any{
					"replicas": 2,
					"resources": map[string]any{
						"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
						"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
					},
				},
			},
		},
		{
			name:   "nil values creates new map",
			values: nil,
			path:   "resources",
			expected: map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{"cpu": "100m", "memory": "256Mi"},
					"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeResourcesIntoValues(tt.values, resources, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"", nil},
		{"resources", []string{"resources"}},
		{"controller.resources", []string{"controller", "resources"}},
		{"a.b.c.d", []string{"a", "b", "c", "d"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := splitPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
