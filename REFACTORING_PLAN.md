# Package Restructuring Plan

## Current Problems

1. **Flat structure**: Everything thrown into `internal/` without organization
2. **No clear separation of concerns**: Hard to understand what each package does
3. **Mixed abstraction levels**: Low-level clients mixed with high-level orchestration
4. **Unclear dependencies**: Hard to see which packages depend on which

## Proposed Structure

Organize packages by **theme/domain** instead of alphabetically:

```
internal/
├── cli/                    # CLI layer
│   ├── commands/          # Cobra command implementations (setup, start, stop, delete, status)
│   └── ui/                # Terminal UI (progress bars, colors, formatting)
│
├── core/                   # Core business logic
│   ├── config/            # Configuration management
│   ├── deployment/        # Tier deployment orchestration
│   │   ├── tier0.go       # Minikube cluster deployment
│   │   ├── tier1.go       # Infrastructure deployment
│   │   ├── tier2.go       # Platform deployment (future)
│   │   └── tier3.go       # Applications deployment (future)
│   └── status/            # System status checking
│
├── platform/               # Platform management (Kubernetes cluster)
│   ├── cluster/           # Minikube cluster operations
│   ├── gpu/               # GPU detection and configuration
│   └── nodes/             # Node management (labeling, BPF mounts, etc.)
│
├── infrastructure/         # Infrastructure clients (wrappers around external tools)
│   ├── docker/            # Docker client
│   ├── helm/              # Helm client
│   ├── kubectl/           # Kubernetes client
│   └── minikube/          # Minikube CLI wrapper (low-level)
│
├── services/               # Host services (Docker containers on host)
│   ├── dns/               # Bind9 DNS server
│   └── gateway/           # NGINX reverse proxy
│
├── security/               # Security-related functionality
│   ├── certificates/      # mkcert CA management
│   └── policies/          # Security policies (future: Kyverno, Falco)
│
├── system/                 # System-level operations
│   ├── dns/               # DNS configuration (resolvconf)
│   ├── preflight/         # Dependency and system checks
│   └── validation/        # Input validation, compatibility checks
│
└── shared/                 # Shared utilities
    ├── errors/            # Error types and handling
    ├── exec/              # Command execution helpers
    └── paths/             # Path management and defaults
```

## Migration Strategy

### Phase 1: Create new structure (no code changes)
1. Create new directory structure
2. Keep old packages in place

### Phase 2: Move packages with minimal dependencies
1. Move `ui/` → `cli/ui/`
2. Move `config/` → `core/config/`
3. Move `preflight/` → `system/preflight/`
4. Move `pki/` → `security/certificates/`
5. Move `dns/` (resolvconf) → `system/dns/`

### Phase 3: Split and move complex packages
1. Split `cmd/` → `cli/commands/` (move command implementations)
2. Split `deployer/` → `core/deployment/` (tier deployment logic)
3. Move `status/` → `core/status/`
4. Move `gpu/` → `platform/gpu/`

### Phase 4: Reorganize infrastructure clients
1. Move `docker/` → `infrastructure/docker/`
2. Move `helm/` → `infrastructure/helm/`
3. Move `k8s/` → `infrastructure/kubectl/`
4. Split `minikube/` between:
   - `infrastructure/minikube/` (CLI wrapper)
   - `platform/cluster/` (cluster operations)

### Phase 5: Reorganize host services
1. Move `bind9/` → `services/dns/`
2. Move `nginx/` → `services/gateway/`

### Phase 6: Update all imports
1. Update imports across the codebase
2. Update tests
3. Verify build

### Phase 7: Cleanup
1. Remove old directories
2. Update documentation
3. Update README with new structure

## Benefits

1. **Clear organization**: Easy to find code by domain
2. **Better separation of concerns**: CLI, core logic, platform, infrastructure, services
3. **Easier onboarding**: New developers can understand the structure quickly
4. **Scalability**: Easy to add new tiers, services, or features
5. **Testability**: Clear boundaries make testing easier
6. **Dependency management**: Easier to see and manage package dependencies

## Package Descriptions

### cli/
Command-line interface layer. Everything related to user interaction.

### core/
Core business logic. Configuration, deployment orchestration, status checking.

### platform/
Kubernetes platform management. Cluster operations, GPU, nodes.

### infrastructure/
Low-level wrappers around external tools (docker, helm, kubectl, minikube CLI).

### services/
Host services that run as Docker containers (DNS, gateway).

### security/
Security-related functionality (certificates, policies).

### system/
System-level operations (DNS config, preflight checks, validation).

### shared/
Utilities shared across packages (errors, exec helpers, paths).

## Import Path Examples

```go
// Before
import "github.com/kzgrzendek/nova/internal/cmd"
import "github.com/kzgrzendek/nova/internal/deployer"
import "github.com/kzgrzendek/nova/internal/bind9"

// After
import "github.com/kzgrzendek/nova/internal/cli/commands"
import "github.com/kzgrzendek/nova/internal/core/deployment"
import "github.com/kzgrzendek/nova/internal/services/dns"
```

## Dependency Flow

```
CLI Layer (cli/*)
    ↓
Core Layer (core/*)
    ↓
Platform Layer (platform/*)
    ↓
Infrastructure Layer (infrastructure/*)
    ↓
Services Layer (services/*)
    ↓
System Layer (system/*)
    ↓
Shared Layer (shared/*)
```

Higher layers can depend on lower layers, but not vice versa.
