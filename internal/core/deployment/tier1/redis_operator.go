// Package tier1 handles the deployment of NOVA Tier 1 (Cluster Infrastructure).
package tier1

import (
	"context"
	"fmt"

	"github.com/kzgrzendek/nova/internal/cli/ui"
	"github.com/kzgrzendek/nova/internal/core/config"
	"github.com/kzgrzendek/nova/internal/core/constants"
	"github.com/kzgrzendek/nova/internal/core/deployment/shared"
	"github.com/kzgrzendek/nova/internal/tools/crypto"
	k8s "github.com/kzgrzendek/nova/internal/tools/kubectl"
)

// deployRedisOperator deploys OT-CONTAINER-KIT Redis Operator via Helm.
// The operator is deployed in tier1 to be available for both Envoy Gateway
// and LaSuite applications (mutualized infrastructure).
func deployRedisOperator(ctx context.Context, cfg *config.Config) error {
	// Add Redis Operator Helm repo
	repos := map[string]string{
		"ot-helm": constants.HelmRepoOTRedis,
	}
	if err := shared.AddHelmRepositories(ctx, repos); err != nil {
		return fmt.Errorf("failed to add Redis Operator Helm repository: %w", err)
	}

	return shared.DeployHelmChart(ctx, shared.HelmDeploymentOptions{
		ReleaseName:     "redis-operator",
		ChartRef:        cfg.Versions.Tier2.RedisOperator.ChartRef(),
		Version:         cfg.Versions.Tier2.RedisOperator.GetVersion(),
		Namespace:       constants.NamespaceRedisOperator,
		ValuesPath:      "resources/core/deployment/tier2/redis-operator/values.yaml",
		Wait:            true,
		TimeoutSeconds:  300,
		CreateNamespace: true,
	})
}

// deployEnvoyRedisCluster deploys a Redis instance for Envoy Gateway rate limiting.
// This uses the OT-CONTAINER-KIT Redis Operator CRDs.
func deployEnvoyRedisCluster(ctx context.Context) error {
	envoyNamespace := constants.NamespaceEnvoyGateway

	// Generate Redis password (or reuse existing)
	redisPassword, err := shared.GetOrGenerateSecret(ctx, envoyNamespace, "redis", "redis-password", 32)
	if err != nil {
		return fmt.Errorf("failed to get or generate Redis password: %w", err)
	}

	// Create Redis secret
	ui.Info("Creating Redis authentication secret...")
	if err := k8s.CreateSecret(ctx, envoyNamespace, "redis", map[string]string{
		"redis-password": redisPassword,
	}); err != nil {
		return fmt.Errorf("failed to create Redis secret: %w", err)
	}

	// Apply Redis CR
	ui.Info("Deploying Redis cluster for Envoy Gateway...")
	if err := k8s.ApplyYAML(ctx, "resources/core/deployment/tier1/redis-operator/clusters/envoy-redis.yaml"); err != nil {
		return fmt.Errorf("failed to apply envoy-redis cluster: %w", err)
	}

	// Wait for Redis to be ready
	ui.Info("Waiting for Envoy Redis to be ready...")
	if err := k8s.WaitForCondition(ctx, envoyNamespace, "redis.redis.redis.opstreelabs.in/envoy-redis", "Ready", 180); err != nil {
		// If condition wait fails, try waiting for the statefulset as fallback
		ui.Warn("Condition wait failed, trying StatefulSet wait...")
		if err := k8s.WaitForStatefulSetReady(ctx, envoyNamespace, "envoy-redis", 180); err != nil {
			return fmt.Errorf("Envoy Redis not ready: %w", err)
		}
	}

	return nil
}

// getOrGenerateRedisPassword generates a random password for Redis.
func getOrGenerateRedisPassword() (string, error) {
	return crypto.GenerateRandomPassword(32)
}
