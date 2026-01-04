# NOVA

**N**ative **O**perator for **V**ersatile **A**I

A GPU-powered Kubernetes lab for AI/ML development on your local machine.

## Overview

NOVA deploys a production-like, multi-tier Kubernetes environment optimized
for AI/ML workloads. With a single command, you get:

- **3-node Minikube cluster** with GPU support
- **Infrastructure tier**: Cilium CNI, Cert-Manager, Envoy Gateway, NVIDIA GPU Operator
- **Platform tier**: Keycloak (IAM), Kyverno (policies), Victoria Metrics/Logs (observability)
- **Application tier**: llm-d (LLM serving), Open WebUI (chat interface), HELIX

## Installation

### Prerequisites

Install these tools before running NOVA:

| Tool     | Version | Installation                                                                        |
|----------|---------|-------------------------------------------------------------------------------------|
| Docker   | 24.0+   | [docs.docker.com](https://docs.docker.com/get-docker/)                              |
| Minikube | 1.32+   | [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/)                    |
| mkcert   | latest  | [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert#installation) |
| certutil | -       | `apt install libnss3-tools` (Ubuntu/Debian)                                         |

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
nova start --tier=1  # Infrastructure only
nova start --tier=2  # Infrastructure + Platform

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

## Commands

| Command                      | Description                                                    |
|------------------------------|----------------------------------------------------------------|
| `nova setup`                 | One-time setup: check dependencies, configure DNS, generate CA |
| `nova start [--tier=N]`      | Start lab up to tier N (1, 2, or 3). Default: 3                |
| `nova stop`                  | Stop lab, preserve state                                       |
| `nova delete [--purge]`      | Delete lab. `--purge` removes config/certs                     |
| `nova status [-v]`           | Show status of all components. `-v` for verbose output         |
| `nova export-logs [-o DIR]`  | Export all logs to timestamped zip archive                     |
| `nova kubectl ...`           | Run kubectl against NOVA cluster                               |
| `nova version`               | Show version information                                       |

## Architecture

```txt
┌─────────────────────────────────────────────────────────────────┐
│                          HOST LAYER                             │
│     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐       │
│     │    NGINX    │    │   Bind9     │    │   mkcert    │       │
│     │   Gateway   │    │    DNS      │    │   Root CA   │       │
│     │ (Docker)    │    │  (Docker)   │    │             │       │
│     └──────┬──────┘    └─────────────┘    └─────────────┘       │
│            │ :443, :80                                          │
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

### Tier 1: Infrastructure

- **Cilium** - eBPF-based CNI with network policies
- **Falco** - Runtime security monitoring
- **NVIDIA GPU Operator** - GPU resource management
- **Cert-Manager + Trust-Manager** - Certificate lifecycle
- **Envoy Gateway** - Gateway API implementation
- **Envoy AI Gateway** - LLM-aware routing

### Tier 2: Platform

- **Kyverno** - Policy engine
- **Keycloak** - Identity and access management
- **Hubble** - Network observability
- **Victoria Metrics** - Prometheus-compatible metrics
- **Victoria Logs** - Centralized logging

### Tier 3: Applications

- **llm-d** - LLM model serving with vLLM
- **Open WebUI** - Web-based LLM chat interface
- **HELIX** - Enterprise AI platform

### Host Services

- **NGINX** - Reverse proxy (ports 80/443)
- **Bind9** - DNS server for `*.nova.local`

## Configuration

Configuration is stored at `~/.nova/config.yaml`:

```yaml
minikube:
  cpus: 4
  memory: 4096
  nodes: 3
  kubernetesVersion: v1.33.5
dns:
  domain: nova.local
  bind9Port: 30053
```

## Accessing Services

### Default Endpoints

| Service    | URL                               | Credentials                       |
|------------|-----------------------------------|-----------------------------------|
| Dashboard  | `https://dashboard.nova.local`    | -                                 |
| Keycloak   | `https://auth.nova.local`         | `admin` / (shown after deploy)    |
| Hubble UI  | `https://hubble.nova.local`       | -                                 |
| Grafana    | `https://grafana.nova.local`      | `admin` / `admin`                 |
| Open WebUI | `https://webui.nova.local`        | Create account                    |

After deployment, NOVA displays all available URLs and Keycloak credentials:

```txt
Cluster deployed. You can now access the following applications:
  Kubernetes Dashboard: https://dashboard.nova.local
  Keycloak: https://auth.nova.local
  ...

Log in via Keycloak:
  As administrator: admin / <random_password>
  As user: user / user
  As developer: developer / developer
```

### SSL/TLS Certificates

NOVA uses mkcert to generate a Root CA that's automatically trusted by your browser. No certificate warnings!

### Developer kubectl Context

NOVA automatically creates a restricted kubectl context (`nova-developer`) during Tier 0 deployment. This context provides full read/write access to namespace-scoped resources, but only within the `developer` namespace.

**Permissions include:**

- Pods, Deployments, Services, ConfigMaps, Secrets
- Jobs, CronJobs, StatefulSets, DaemonSets
- PersistentVolumeClaims, ServiceAccounts
- Ingresses, NetworkPolicies
- Events, Endpoints, ResourceQuotas, LimitRanges

**Restrictions:**

- No access to cluster-scoped resources (nodes, namespaces, etc.)
- No access to other namespaces (kube-system, etc.)

**Switch contexts:**

```bash
# Use the developer context (restricted to 'developer' namespace)
kubectl config use-context nova-developer

# Switch back to admin context (full cluster access)
kubectl config use-context minikube
```

**Example workflow:**

```bash
# As developer: deploy an application in the developer namespace
kubectl config use-context nova-developer
kubectl run nginx --image=nginx
kubectl get pods  # Shows pods in 'developer' namespace only

# Cannot access other namespaces
kubectl get pods -n kube-system  # Error: forbidden
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

# Run tests
mage test

# Run tests in short mode
mage testShort

# Run tests with coverage report (generates coverage/coverage.html)
mage testCoverage

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
│   │   ├── deployer/             # Deployment coordination
│   │   └── status/               # System status checking
│   ├── setup/                     # One-time initialization
│   │   ├── preflight/            # Dependency checks
│   │   ├── certificates/         # mkcert CA management
│   │   └── system/               # System configuration (DNS)
│   ├── deployment/                # Runtime tier deployments
│   │   ├── tier0/                # Minikube cluster + GPU detection
│   │   ├── tier1/                # Infrastructure (ready for components)
│   │   ├── tier2/                # Platform (future)
│   │   └── tier3/                # Applications (future)
│   ├── host/                      # Host services (outside k8s)
│   │   ├── dns/bind9/            # Bind9 DNS server
│   │   └── gateway/nginx/        # NGINX reverse proxy
│   ├── tools/                     # External tool integrations
│   │   ├── docker/               # Docker SDK wrapper
│   │   ├── helm/                 # Helm SDK wrapper
│   │   ├── kubectl/              # Kubernetes client-go wrapper
│   │   └── minikube/             # Minikube CLI wrapper
│   └── shared/                    # Common utilities (future)
├── pkg/resources/                 # Embedded resources
├── resources/                     # Helm values, manifests
└── magefiles/                     # Build targets
```

## License

Apache 2.0

---

Made with ❤️ by [@kzgrzendek](https://github.com/kzgrzendek)
