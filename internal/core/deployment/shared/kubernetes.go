package shared

import (
	"context"
	"fmt"

	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// EnsureNamespace ensures a namespace exists and has the specified labels.
// It is idempotent: checks existence first, then creates or updates.
func EnsureNamespace(ctx context.Context, name string, labels map[string]string) error {
	// 1. Create namespace if it doesn't exist
	if err := k8s.CreateNamespace(ctx, name); err != nil {
		return fmt.Errorf("failed to ensure namespace %s: %w", name, err)
	}

	// 2. Apply labels if provided
	if len(labels) > 0 {
		for key, value := range labels {
			if err := k8s.LabelNamespace(ctx, name, key, value); err != nil {
				return fmt.Errorf("failed to label namespace %s with %s=%s: %w", name, key, value, err)
			}
		}
	}

	return nil
}
