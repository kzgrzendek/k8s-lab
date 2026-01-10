# Foundation Stack

The Foundation Stack is NOVA's host-level service infrastructure that must be running before the Kubernetes cluster starts. It provides essential networking, DNS, storage, and gateway services.

## Architecture

The Foundation Stack consists of Docker containers running on the host machine:

```
┌─────────────────────────────────────────────────────────────┐
│                     Foundation Stack                         │
│                  (Host Docker Containers)                    │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  NGINX   │  │  Bind9   │  │   NFS    │  │ Registry │   │
│  │ Gateway  │  │   DNS    │  │  Server  │  │  (opt)   │   │
│  │  .242    │  │   .243   │  │   .241   │  │   .240   │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│                                                               │
│                    Nova Docker Network                        │
│                      172.19.0.0/16                           │
└─────────────────────────────────────────────────────────────┘
                            ▲
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Minikube Cluster                            │
│         (Kubernetes running in Docker containers)            │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Control Plane: .2    Workers: .3, .4, .5, ...              │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## IP Allocation Strategy

All services use Docker's automatic IP allocation on the nova Docker network:

| Service           | IP Address         | Purpose                                    |
|-------------------|--------------------|--------------------------------------------|
| Minikube Control  | Auto-assigned      | Kubernetes control plane                   |
| Minikube Workers  | Auto-assigned      | Worker nodes (sequential allocation)       |
| Registry          | Auto-assigned      | Local container registry (tier 3 only)     |
| NFS Server        | Auto-assigned      | Persistent storage (tier 3 only)           |
| NGINX Gateway     | Auto-assigned      | Reverse proxy to cluster                   |
| Bind9 DNS         | Auto-assigned      | DNS resolution for *.nova.local            |

**Why automatic allocation?**

- **Predictable**: Docker assigns IPs sequentially as containers start
- **No conflicts**: Docker's IPAM ensures no IP overlap
- **Simpler**: No manual IP calculation or coordination needed
- **Reliable**: Minikube with `--profile=nova` gets consistent naming

## Service Startup Order

Services start in a specific order to handle dependencies:

```
1. Nova Docker Network
   └─→ Creates 172.19.0.0/16 network

2. Minikube Cluster
   ├─→ Starts with --profile=nova for consistent naming
   └─→ Gets automatic IPs from Docker (control plane + workers)

3. NGINX Gateway
   ├─→ Discovers minikube IP dynamically after cluster starts
   └─→ Proxies HTTPS traffic to Kubernetes ingresses

4. Bind9 DNS
   ├─→ Provides DNS for *.nova.local domains
   └─→ Needed by Registry for registry.local resolution

5. NFS Server [Tier 3 only]
   ├─→ Provides persistent storage for models
   └─→ Mounted by Kubernetes pods

6. Registry [Tier 3 only]
   ├─→ Local container registry
   └─→ Requires DNS for registry.local
```

**Key Design Decision**: Minikube starts FIRST in the foundation stack (immediately after network creation), getting automatic IPs from Docker. All other services start afterward and can discover minikube's IP dynamically. This approach avoids static IP conflicts and unpredictable minikube behavior with multi-node clusters.

## Usage

### Starting the Foundation Stack

```go
import (
    "context"
    "github.com/kzgrzendek/nova/internal/core/config"
    "github.com/kzgrzendek/nova/internal/host/foundation"
)

// Create foundation manager
cfg, _ := config.Load()
foundationStack := foundation.New(cfg)

// Start all foundation services (including minikube cluster)
// tier parameter determines which optional services start (tier >= 3 includes NFS + Registry)
if err := foundationStack.Start(context.Background(), 3); err != nil {
    log.Fatal(err)
}

// Minikube cluster is already started by foundation stack with --profile=nova
```

### Stopping the Foundation Stack

```go
// Stop all services (preserves network for faster restart)
if err := foundationStack.Stop(context.Background()); err != nil {
    log.Fatal(err)
}
```

### Deleting the Foundation Stack

```go
// Delete all services and remove network (complete cleanup)
if err := foundationStack.Delete(context.Background()); err != nil {
    log.Fatal(err)
}
```

## Service Details

### NGINX Gateway

- **Purpose**: Reverse proxy from host to Kubernetes ingresses
- **IP**: Auto-assigned by Docker
- **Ports**: `443` (HTTPS)
- **Configuration**: Discovers minikube IP dynamically using `minikube.GetIP()`
- **TLS**: Uses mkcert certificates for *.nova.local

### Bind9 DNS

- **Purpose**: DNS resolution for *.nova.local domains
- **IP**: Auto-assigned by Docker
- **Port**: `53` (UDP/TCP)
- **Records**: Wildcard A record pointing to NGINX gateway
- **Integration**: System DNS configured to use Bind9 via systemd-resolved

### NFS Server (Tier 3)

- **Purpose**: Persistent storage for LLM models
- **IP**: Auto-assigned by Docker
- **Exports**: `/exports/models` → `~/.nova/share/nfs/models`
- **Access**: Kubernetes pods mount via NFS StorageClass

### Registry (Tier 3)

- **Purpose**: Local container registry for image distribution
- **IP**: Auto-assigned by Docker
- **Port**: `5000`
- **DNS**: Accessible as `registry.local`
- **Usage**: Warmup images pushed here, then pulled to minikube nodes

## Error Handling

The Foundation Stack uses **fail-fast** error handling:

- **No automatic rollback**: If a service fails to start, the error is returned immediately
- **Manual cleanup**: Use `Stop()` or `Delete()` to clean up after errors
- **Clear errors**: Each error wraps the underlying cause with context

Example:
```go
if err := foundationStack.Start(ctx, tier); err != nil {
    // Error example: "failed to start foundation stack: failed to start NGINX: container already exists"
    log.Printf("Foundation startup failed: %v", err)

    // Clean up manually
    foundationStack.Delete(ctx)
    return err
}
```

## Integration with NOVA Deployment

The Foundation Stack is integrated into NOVA's deployment flow:

```go
// 1. Foundation Stack (includes minikube cluster)
foundationStack := foundation.New(cfg)
if err := foundationStack.Start(ctx, targetTier); err != nil {
    return err
}

// 2. Warmup operations (tier 3, background)
var warmupOrch *warmup.Orchestrator
if targetTier >= 3 {
    warmupOrch = warmup.New(ctx, cfg)
    warmupOrch.Start()
}

// 3. Tier 0: Cluster post-configuration
// Note: Minikube is already running (started by foundation)
if err := tier0.DeployTier0(ctx, cfg); err != nil {
    return err
}

// 4-6. Tiers 1-3...
```

## Benefits

### Before Foundation Stack

```text
Problems:
- Inconsistent startup order (host services at different times)
- Static IP conflicts with multi-node minikube clusters
- Scattered orchestration logic in start.go (~150 lines)
- Confusing phases: "Pre-warmup", "Post-deployment", etc.
```

### After Foundation Stack

```text
Benefits:
✓ Consistent startup: Foundation stack with clear ordering
✓ Automatic IP allocation: No conflicts, Docker manages IPs
✓ Clean abstraction: Single foundation.Start() call
✓ Clear separation: Host services separate from cluster services
✓ Easier maintenance: All host service logic in one package
✓ Simpler start.go: Reduced by ~150 lines
✓ Minikube profile: Consistent naming with --profile=nova
```

## Testing

```bash
# Run foundation package tests (when available)
go test ./internal/host/foundation/...

# Manual integration test
nova start --tier=3  # Full foundation + warmup + cluster
nova stop            # Stop cluster + foundation
nova start --tier=3  # Foundation already exists, faster restart
nova delete          # Complete cleanup
```

## Future Improvements

Potential enhancements:

1. **Health checks**: Monitor service health and auto-restart
2. **Graceful shutdown**: Coordinate service shutdown order
3. **Service discovery**: Dynamic service registration/discovery
4. **Metrics**: Export foundation service metrics
5. **Configuration validation**: Pre-flight checks before startup

## Related Files

- [internal/host/foundation/foundation.go](foundation.go) - Core implementation
- [internal/host/gateway/nginx/gateway.go](../gateway/nginx/gateway.go) - NGINX service
- [internal/host/dns/bind9/dns.go](../dns/bind9/dns.go) - Bind9 DNS service
- [internal/host/nfs/nfs.go](../nfs/nfs.go) - NFS server service
- [internal/host/registry/registry.go](../registry/registry.go) - Registry service
- [internal/utils/network.go](../../utils/network.go) - IP calculation utilities
