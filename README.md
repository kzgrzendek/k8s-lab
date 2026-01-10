# NOVA

**N**ative **O**perator for **V**ersatile **A**I

A GPU-powered Kubernetes lab for AI/ML development on your local machine.

## Overview

NOVA deploys a production-like, multi-tier Kubernetes environment optimized
for AI/ML workloads. With a single command, you get:

- **Tier 0 - Cluster Basics**: Minikube cluster with Cilium CNI, CoreDNS, and Local Path Storage
- **Warmup Module**: Background model and image pre-loading for faster application startup
- **Tier 1 - Infrastructure**: Security (Falco), GPU (NVIDIA Operator), Certificates (Cert-Manager), and Gateways (Envoy)
- **Tier 2 - Platform**: Keycloak (IAM), Kyverno (policies), Victoria Metrics/Logs (observability)
- **Tier 3 - Applications**: llm-d (LLM serving), Open WebUI (chat interface), HELIX (JupyterHub)

## Architecture

NOVA uses a **Foundation Stack** architecture where host-level Docker services start before the Kubernetes cluster. This design ensures all prerequisites are in place for seamless cluster operation.

### Deployment Flow

```text
┌─────────────────────────────────────────────────────────────────┐
│  1. Foundation Stack (Host Docker Containers)                   │
│     ├── Nova Network (172.19.0.0/16)                            │
│     ├── NGINX Gateway (.242) - Reverse proxy to cluster         │
│     ├── Bind9 DNS (.243) - DNS for *.nova.local domains         │
│     ├── NFS Server (.241) - Model storage (tier 3 only)         │
│     └── Registry (.240) - Image distribution (tier 3 only)      │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│  2. Warmup Phase (Tier 3 only, runs in background)              │
│     ├── Model Download - Downloads LLMs to NFS storage          │
│     └── Image Warmup - Pre-pulls CUDA images (GPU mode)         │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│  3. Tier 0: Minikube Cluster                                    │
│     ├── Control Plane (.2) - Uses static IP                     │
│     ├── Workers (.3, .4, .5...) - Sequential allocation         │
│     └── Kubernetes Dashboard                                    │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│  4. Tier 1: Infrastructure                                      │
│     ├── Cilium CNI, Falco, NVIDIA GPU Operator                  │
│     ├── Cert-Manager, Trust-Manager                             │
│     └── Envoy Gateway, Envoy AI Gateway                         │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│  5. Tier 2: Platform                                            │
│     ├── Kyverno (Policy Engine)                                 │
│     ├── Keycloak (Identity & Access Management)                 │
│     └── Victoria Metrics/Logs, Hubble UI                        │
└─────────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────────┐
│  6. Tier 3: Applications                                        │
│     ├── llm-d (LLM Serving with vLLM)                           │
│     ├── Open WebUI (Chat Interface)                             │
│     └── HELIX (JupyterHub for ML)                               │
└─────────────────────────────────────────────────────────────────┘
```

### IP Allocation Strategy

All services use **static IP allocation** on the nova Docker network to prevent conflicts:

| Service           | IP Address    | Purpose                                    |
|-------------------|---------------|--------------------------------------------|
| Minikube Control  | `.2`          | Kubernetes control plane (static IP)      |
| Minikube Workers  | `.3-.239`     | Worker nodes (sequential allocation)      |
| Registry          | `.240`        | Local container registry (tier 3 only)    |
| NFS Server        | `.241`        | Persistent storage (tier 3 only)          |
| NGINX Gateway     | `.242`        | Reverse proxy to cluster                  |
| Bind9 DNS         | `.243`        | DNS resolution for *.nova.local           |

**Key Design**: Minikube uses `.2` (first usable IP) so workers follow naturally. Host services use the high range (.240-.243) to avoid conflicts.

### Foundation Stack Benefits

- **Consistent startup**: All host services ready before cluster starts
- **Static IP strategy**: NGINX pre-configured with known minikube IP
- **Parallel warmup**: Models and images download while cluster deploys
- **Clean abstraction**: Simple API for complex orchestration
- **Fail-fast**: Errors stop deployment immediately

For detailed foundation architecture, see [internal/host/foundation/README.md](internal/host/foundation/README.md).

## Installation

### Prerequisites

Install these tools before running NOVA:

| Tool     | Version | Installation                                                                                        |
|----------|---------|-----------------------------------------------------------------------------------------------------|
| Docker   | 24.0+   | [docs.docker.com](https://docs.docker.com/get-docker/)                                              |
| Minikube | 1.32+   | [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/)                                    |
| mkcert   | latest  | [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert#installation)                     |
| certutil | -       | `apt install libnss3-tools` (Ubuntu/Debian)                                                         |

**Image Distribution**: NOVA automatically manages large container images (5-8GB LLM models) using a local Docker registry and skopeo (Docker container). This memory-efficient approach uses ~300MB RAM instead of 5-10GB, preventing system instability during image transfers to multi-node clusters.

### GPU Support (Optional)

For NVIDIA GPU acceleration:

```bash
# 1. NVIDIA Driver (should already be installed)
nvidia-smi

# 2. NVIDIA Container Toolkit
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
sudo apt update && sudo apt install -y nvidia-container-toolkit

# 3. Configure Docker runtime
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker

# 4. Verify
docker run --rm --gpus all nvidia/cuda:12.2.0-base-ubuntu22.04 nvidia-smi
```

### Install NOVA

```bash
# From source
go install github.com/kzgrzendek/nova/cmd/nova@latest

# Or build locally
git clone https://github.com/kzgrzendek/nova.git
cd nova
mage install
```

## Quick Start

```bash
# One-time setup (checks dependencies, configures DNS, generates certificates)
nova setup

# Start the full lab (all 3 tiers)
nova start

# Or start with specific tier
nova start --tier=1  # Cluster basics + warmup + infrastructure
nova start --tier=2  # + platform services

# Optional: Customize Kubernetes version
nova start --kubernetes-version=v1.32.0

# Optional: Force CPU mode even if GPU is available
nova start --cpu-mode

# Optional: Provide Hugging Face token for faster model downloads (tier 3)
nova start --hf-token=YOUR_HF_TOKEN

# Check cluster status
nova kubectl get nodes
nova kubectl get pods -A

# Stop (preserves state)
nova stop

# Delete (removes everything)
nova delete

# Delete with purge (removes config and certificates too)
nova delete --purge
```

## Commands

### Core Commands

| Command                      | Description                                                    |
|------------------------------|----------------------------------------------------------------|
| `nova setup`                 | One-time setup: check dependencies, configure DNS, generate CA |
| `nova start [flags]`         | Start lab up to specified tier (default: tier 3)               |
| `nova stop`                  | Stop lab, preserve state                                       |
| `nova delete [--purge]`      | Delete lab. `--purge` removes config/certs                     |
| `nova status [-v]`           | Show status of all components. `-v` for verbose output         |
| `nova export-logs [-o DIR]`  | Export all logs to timestamped zip archive                     |
| `nova kubectl ...`           | Run kubectl against NOVA cluster                               |
| `nova version`               | Show version information                                       |

### Start Command Flags

| Flag                      | Type   | Default         | Description                                    |
|---------------------------|--------|-----------------|------------------------------------------------|
| `--tier`                  | int    | 3               | Deploy up to tier N (0, 1, 2, or 3)            |
| `--k8s-version`           | string | v1.33.5         | Kubernetes version for Minikube cluster        |
| `--nodes`                 | int    | (from config)   | Total number of nodes in the cluster           |
| `--cpu-mode`              | bool   | false           | Force CPU mode even if GPU is available        |
| `--hf-token`              | string | -               | Hugging Face token for model downloads         |
| `--model`                 | string | Qwen/Qwen3-0.6B | Hugging Face model to serve                    |

### Setup Command Flags

| Flag          | Type | Default | Description                                   |
|---------------|------|---------|-----------------------------------------------|
| `--skip-dns`  | bool | false   | Skip DNS configuration                        |
| `--rootless`  | bool | false   | Rootless mode (skip DNS, warn instead of fail)|

### Delete Command Flags

| Flag          | Type | Default | Description                                   |
|---------------|------|---------|-----------------------------------------------|
| `--purge`     | bool | false   | Remove config and certificates too            |
| `-y, --yes`   | bool | false   | Skip confirmation prompt                      |

### Model Configuration

NOVA supports serving any Hugging Face model compatible with vLLM. By default, NOVA deploys **Qwen/Qwen3-0.6B**, a lightweight open-source model that requires no authentication.

#### Gated Models Warning

Some Hugging Face models are **gated behind license agreements** (e.g., Meta Llama, Google Gemma). To use these models:

1. **Create a Hugging Face account** at [huggingface.co](https://huggingface.co)
2. **Accept the model license** on the model's page (e.g., `https://huggingface.co/google/gemma-3-4b-it`)
3. **Generate an API token** at [Settings > Access Tokens](https://huggingface.co/settings/tokens)
4. **Provide the token** when starting NOVA:

```bash
nova start --model=google/gemma-3-4b-it --hf-token=YOUR_HF_TOKEN
```

The default model (Qwen/Qwen3-0.6B) does **not** require a Hugging Face account or token.

## Configuration

NOVA uses a configuration file at `~/.nova/config.yaml` that manages all aspects of your lab environment.

### Configuration Structure

```yaml
minikube:
  cpus: 4                          # CPU cores per node
  memory: 4096                      # Memory in MB per node
  nodes: 3                          # Total nodes (1 control plane + 2 workers)
  kubernetesVersion: v1.33.5        # Kubernetes version
  driver: docker                    # Minikube driver (docker or kvm2)
  gpus: all                         # GPU passthrough: "all", "none", or "disabled"
  cpuModeForced: false              # Force CPU mode even if GPU available

dns:
  domain: nova.local                # Primary domain for services
  authDomain: auth.local            # Keycloak domain
  bind9Port: 30053                  # DNS server port

llm:
  hfToken: ""                       # Optional: Hugging Face token

state:
  initialized: false                # Setup completion flag
  lastDeployedTier: 0               # Last successfully deployed tier

# Component versions (all version-pinned for reproducibility)
versions:
  kubernetes: v1.33.5               # Minikube Kubernetes version

  # Tier 1: Infrastructure
  tier1:
    cilium:
      chart: cilium/cilium
      version: 1.18.5
    falco:
      chart: falcosecurity/falco
      version: 7.0.2
    gpuOperator:
      chart: nvidia/gpu-operator
      version: v25.10.1
    certManager:
      chart: jetstack/cert-manager
      version: v1.19.2
    trustManager:
      chart: jetstack/trust-manager
      version: v0.20.3
    envoyGateway:
      chart: oci://docker.io/envoyproxy/gateway-helm
      version: v1.6.1
    envoyAiGateway:
      chart: oci://docker.io/envoyproxy/ai-gateway-helm
      version: v0.4.0
    redis:
      chart: dandydev/redis-ha
      version: 4.35.5
    # Manifest-based components (non-Helm)
    localPathProvisioner: v0.0.33
    gatewayApiInferenceExtension: v1.2.1

  # Tier 2: Platform Services
  tier2:
    kyverno:
      chart: kyverno/kyverno
      version: 3.6.1
    hubble:
      chart: cilium/cilium
      version: 1.18.5
    victoriaLogsSingle:
      chart: vm/victoria-logs-single
      version: 0.11.23
    victoriaLogsCollector:
      chart: vm/victoria-logs-collector
      version: 0.2.4
    victoriaMetricsStack:
      chart: vm/victoria-metrics-k8s-stack
      version: 0.66.1
    # Manifest-based components
    keycloakOperator: "26.4.7"

  # Tier 3: Application Layer
  tier3:
    llmd:
      chart: llm-d-modelservice/llm-d-modelservice
      version: v0.3.16
      customValuesPath: ""          # Optional: path to custom Helm values
    inferencePool:
      chart: oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool
      version: v1.2.1
      customValuesPath: ""          # Optional: path to custom Helm values
    openWebui:
      chart: open-webui/open-webui
      version: "9.0.0"
      customValuesPath: ""          # Optional: path to custom Helm values
    helix:
      chart: aphp-helix/helix
      version: 1.0.11
      customValuesPath: ""          # Optional: path to custom Helm values
```

### Version Management

NOVA pins all component versions for **reproducible deployments**:

- **Helm Charts**: All charts have explicit versions
- **Kubernetes Manifests**: Operator CRDs and other manifests use versioned URLs
- **Automatic Migration**: Config file auto-updates when new fields are added

This ensures your lab environment is:

- ✅ **Reproducible** - Same versions every time
- ✅ **Upgradable** - Update versions explicitly when ready
- ✅ **Documented** - All versions tracked in one place

### Custom Values for Tier 3 Apps

**Tier 3 applications support optional custom Helm values** that merge on top of default configurations:

```yaml
versions:
  tier3:
    openWebui:
      chart: open-webui/open-webui
      version: "9.0.0"
      customValuesPath: /home/user/my-openwebui-values.yaml  # Add custom values
```

**How it works:**

1. Default values are always loaded from NOVA's built-in configurations
2. If `customValuesPath` is specified, that file is loaded and **merged on top**
3. Custom values override/extend defaults (Helm merge semantics)
4. Supports templating for OpenWebUI and Helix (e.g., `{{.Domain}}`, `{{.AuthDomain}}`)

**Example custom values file** (`my-openwebui-values.yaml`):

```yaml
# Override resource limits
resources:
  limits:
    memory: 2Gi
    cpu: 1000m

# Add extra environment variables
extraEnvVars:
  - name: MY_CUSTOM_VAR
    value: "custom-value"

# Change replica count
replicaCount: 2
```

This gives you **full flexibility** to customize Tier 3 applications without modifying NOVA's default files!

## Debugging and Troubleshooting

### Export Logs

When you encounter issues, export all cluster logs for analysis:

```bash
# Export to current directory
nova export-logs

# Export to specific directory
nova export-logs -o /path/to/output
```

This creates a timestamped zip archive (e.g., `nova-logs-20250129-143022.zip`)
containing:

- **System Information** - Docker, Minikube, Kubectl versions
- **Minikube Logs** - Cluster status and logs
- **Kubernetes Cluster Logs** - Nodes, namespaces, resources, events
- **Node Kubelet Logs** - Kubelet logs from all cluster nodes
- **Pod Logs** - Current and previous logs from all pods (all namespaces)
- **Docker Container Logs** - Bind9 DNS and NGINX gateway logs
- **Configuration Files** - NOVA config and kubeconfig

The archive is perfect for sharing when reporting issues or debugging complex
problems.

## Architecture

```txt
┌─────────────────────────────────────────────────────────────────┐
│                          HOST LAYER                             │
│     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│     │    NGINX    │    │   Bind9     │    │   mkcert    │       │
│     │   Gateway   │    │    DNS      │    │   Root CA   │       │
│     │ (Docker)    │    │  (Docker)   │    │             │       │
│     └──────┬──────┘    └─────────────┘    └─────────────┘       │
│            │ :443                                               │
├────────────┼────────────────────────────────────────────────────┤
│            ▼                                                    │
│                        TIER 3 - Applications                    │
│     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│     │    llm-d    │    │  Open WebUI │    │    HELIX    │       │
│     │ (LLM serve) │    │   (Chat UI) │    │   (APHP)    │       │
│     └─────────────┘    └─────────────┘    └─────────────┘       │
├─────────────────────────────────────────────────────────────────┤
│                       TIER 2 - Platform                         │
│  ┌─────────┐  ┌──────────┐  ┌────────┐  ┌───────────────────┐   │
│  │ Kyverno │  │ Keycloak │  │ Hubble │  │ Victoria Metrics  │   │
│  │(Policy) │  │  (IAM)   │  │ (Net)  │  │     & Logs        │   │
│  └─────────┘  └──────────┘  └────────┘  └───────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│                     TIER 1 - Infrastructure                     │
│  ┌────────┐  ┌───────┐  ┌────────────┐  ┌───────────────────┐   │
│  │ Cilium │  │ Falco │  │  NVIDIA    │  │   Cert-Manager    │   │
│  │  CNI   │  │(Secur)│  │ GPU Oper.  │  │  Trust-Manager    │   │
│  └────────┘  └───────┘  └────────────┘  └───────────────────┘   │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │          Envoy Gateway + AI Gateway + Redis              │   │
│  │            (Gateway API + Inference Extension)           │   │
│  └──────────────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────────────┤
│                     TIER 0 - Minikube Cluster                   │
│              3 nodes │ Docker driver │ GPU enabled              │
└─────────────────────────────────────────────────────────────────┘
```

### Tier 0: Cluster Basics

NOVA's foundation - the Kubernetes cluster with essential networking, DNS, and storage:

- **Minikube** (latest version) - Multi-node Kubernetes cluster with Docker driver
  - Kubernetes v1.33.5 (configurable via `--kubernetes-version`)
  - 3 nodes (1 control plane + 2 workers)
  - GPU passthrough support via NVIDIA Container Toolkit
  - CPU mode fallback for systems without GPU
- **Cilium CNI** (v1.18.5) - eBPF-based networking with network policies
- **CoreDNS** - DNS configuration with domain rewrites for NOVA services
- **Local Path Provisioner** (v0.0.33) - Dynamic local storage provisioner

**Deployment includes:**

- Automatic GPU detection and node configuration
- BPF filesystem mounting for Cilium/Falco eBPF support
- Control-plane node tainting and labeling
- Network policies and storage classes

### Warmup Module

Optimizes application deployment by pre-loading models and images in the background:

- **Node Election**: Selects the optimal node for llm-d workloads based on GPU/CPU mode
  - GPU mode: Elects first GPU worker node
  - CPU mode: Elects first CPU worker node
  - Single-node: Uses control-plane node (removes NoSchedule taint)
- **Model Warmup** (async): Downloads Hugging Face models to PVC during tier1/tier2 deployment
  - Reduces llm-d startup time from 10-20 minutes to seconds
  - Runs as Kubernetes Job in `llmd` namespace
  - Supports gated models with HF_TOKEN authentication
- **Image Warmup** (async, GPU only): Pre-pulls heavy container images (5-8GB) using local registry
  - Uses memory-efficient skopeo + local registry (~300MB RAM vs 5-10GB)
  - Runs in background during tier1/tier2 deployment
  - Completes before tier3 to ensure fast application startup

**Graceful Degradation**: All warmup failures are non-blocking - deployment continues with slower cold starts

### Tier 1: Infrastructure

Security, GPU management, certificates, and gateway infrastructure:

| Component               | Version  | Description                                  |
| ----------------------- | -------- | -------------------------------------------- |
| **Falco**               | v7.0.2   | Runtime security monitoring with modern eBPF |
| **NVIDIA GPU Operator** | v25.10.1 | GPU resource management (GPU mode only)      |
| **Cert-Manager**        | v1.19.2  | TLS certificate management                   |
| **Trust-Manager**       | v0.20.3  | Certificate bundle distribution              |
| **Envoy Gateway**       | v1.6.1   | Gateway API implementation                   |
| **Envoy AI Gateway**    | v0.4.0   | LLM-aware routing with inference extensions  |
| **Redis**               | v4.35.5  | High-availability key-value store            |

### Tier 2: Platform

| Component            | Version  | Description                                          |
| -------------------- | -------- | ---------------------------------------------------- |
| **Kyverno**          | v3.6.1   | Policy engine for security and compliance            |
| **Keycloak**         | v26.4.7  | Identity and access management (IAM)                 |
| **Hubble**           | v1.18.5  | Network observability powered by Cilium              |
| **Victoria Metrics** | v0.66.1  | Prometheus-compatible metrics with long-term storage |
| **Victoria Logs**    | v0.11.23 | High-performance centralized logging                 |

### Tier 3: Applications

| Component          | Version | Description                                        |
| ------------------ | ------- | -------------------------------------------------- |
| **llm-d**          | v0.3.16 | LLM model serving with vLLM backend                |
| **Inference Pool** | v1.2.1  | Gateway API Inference Extension for load balancing |
| **Open WebUI**     | v9.0.0  | Modern web-based LLM chat interface with OIDC      |
| **HELIX**          | v1.3.1  | Enterprise AI platform from APHP                   |

### Host Services

| Component    | Description                                              |
| ------------ | -------------------------------------------------------- |
| **Registry** | Local Docker registry for image distribution            |
| **NGINX**    | Reverse proxy (ports 443) for cluster ingress            |
| **Bind9**    | DNS server for `*.nova.local` and `*.auth.local` domains |

## Accessing Services

### Default Endpoints

All services are protected by Keycloak SSO authentication:

| Service              | URL                               | Authentication                           |
|----------------------|-----------------------------------|------------------------------------------|
| Keycloak (Auth)      | `https://auth.local`              | `cluster-admin` / (shown after deploy)   |
| Hubble UI            | `https://hubble.nova.local`       | Login via Keycloak                       |
| Grafana              | `https://grafana.nova.local`      | Login via Keycloak                       |
| Open WebUI           | `https://chat.nova.local`         | Login via Keycloak                       |
| HELIX                | `https://helix.nova.local`        | Login via Keycloak                       |

After deployment, NOVA displays all available URLs and Keycloak credentials:

```txt
Cluster deployed. You can now access the following applications:
  Keycloak: https://auth.local
  Hubble UI: https://hubble.nova.local
  Grafana: https://grafana.nova.local
  Open WebUI: https://chat.nova.local
  HELIX: https://helix.nova.local

Log in via Keycloak:
  As administrator: admin / <random_password>
  As user: user / user
  As developer: developer / developer
```

### SSL/TLS Certificates

NOVA uses mkcert to generate a Root CA that's automatically trusted by your browser. No certificate warnings!

### Kubectl Context

NOVA automatically creates a `cluster-admin` kubectl context during Tier 0 deployment. This context provides full cluster access for administrative operations.

**Switch contexts:**

```bash
# Use the cluster-admin context (full cluster access)
kubectl config use-context cluster-admin

# Or use minikube's default context
kubectl config use-context minikube
```

**Example workflow:**

```bash
# Switch to cluster-admin context
kubectl config use-context cluster-admin
kubectl get nodes
kubectl get pods -A  # Shows pods in all namespaces
kubectl get pods -n kube-system  # Full access to all namespaces
```

## Development

### Build

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Available Mage targets
mage -l

# Build for current platform
mage build

# Build for all platforms
mage buildAll

# Format code
mage fmt

# Run linter
mage lint

# Run all CI checks (format, lint, test)
mage ci

# Install to $GOPATH/bin
mage install

# Clean build artifacts
mage clean
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run specific package
go test ./internal/core/config/...

# Generate coverage report
mage testCoverage
# Opens coverage/coverage.html
```

### Code Quality Standards

- All new features must include tests
- Use table-driven tests for multiple test cases
- Mock external dependencies (filesystem, network, kubectl)
- Follow DRY, KISS, YAGNI principles
- Minimum 60% coverage for new code

### Project Structure

```txt
nova/
├── cmd/nova/                      # Entry point
├── internal/
│   ├── cli/                       # CLI layer
│   │   ├── commands/             # Cobra command implementations
│   │   └── ui/                   # Terminal UI utilities
│   ├── core/                      # Core business logic
│   │   ├── config/               # Configuration management
│   │   ├── constants/            # Shared constants
│   │   └── deployment/           # Deployment coordination
│   │       ├── tier0/            # Minikube cluster + GPU detection
│   │       ├── tier1/            # Infrastructure tier
│   │       ├── tier2/            # Platform tier
│   │       ├── tier3/            # Application tier
│   │       └── shared/           # Shared deployment utilities
│   ├── setup/                     # One-time initialization
│   │   ├── preflight/            # Dependency checks
│   │   ├── certificates/         # mkcert CA management
│   │   └── system/               # System configuration (DNS)
│   ├── host/                      # Host services (outside k8s)
│   │   ├── dns/bind9/            # Bind9 DNS server
│   │   └── gateway/nginx/        # NGINX reverse proxy
│   └── tools/                     # External tool integrations
│       ├── docker/               # Docker SDK wrapper
│       ├── helm/                 # Helm SDK wrapper
│       ├── kubectl/              # Kubernetes client-go wrapper
│       ├── minikube/             # Minikube CLI wrapper
│       └── crypto/               # Cryptographic utilities
├── resources/                     # Helm values, manifests, templates
│   └── core/deployment/
│       ├── tier0/                # Minikube and RBAC configs
│       ├── tier1/                # Infrastructure Helm values
│       ├── tier2/                # Platform Helm values
│       └── tier3/                # Application Helm values
└── magefiles/                     # Build targets
```

## License

Apache 2.0

---

Made with ❤️ by [@kzgrzendek](https://github.com/kzgrzendek)
