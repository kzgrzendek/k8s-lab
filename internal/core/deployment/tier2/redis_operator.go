// Package tier2 handles the deployment of NOVA Tier 2 (Platform Services).
package tier2

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// Note: Redis Operator is deployed in tier1 for infrastructure consistency.
// This file contains Redis cluster deployment functions for tier2/tier3.

// DeployLaSuiteRedisCluster deploys a Redis instance for LaSuite applications.
func DeployLaSuiteRedisCluster(ctx context.Context, namespace string) error {
	ui.Info("Deploying Redis cluster for LaSuite applications...")

	// Apply Redis cluster CR with namespace templating
	data := map[string]any{
		"Namespace": namespace,
	}
	if err := shared.ApplyTemplate(ctx, "resources/core/deployment/tier2/redis-operator/clusters/lasuite-redis.yaml", data); err != nil {
		return fmt.Errorf("failed to apply lasuite-redis cluster: %w", err)
	}

	// Wait for Redis to be ready
	ui.Info("Waiting for LaSuite Redis to be ready...")
	if err := k8s.WaitForCondition(ctx, namespace, "redis.redis.redis.opstreelabs.in/lasuite-redis", "Ready", 180); err != nil {
		return fmt.Errorf("LaSuite Redis not ready: %w", err)
	}

	return nil
}
