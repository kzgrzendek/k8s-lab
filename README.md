# NOVA

**N**ative **O**perator for **V**ersatile **A**I

A GPU-powered Kubernetes lab for AI/ML development on your local machine.

## Overview

NOVA deploys a production-like, multi-tier Kubernetes environment optimized for AI/ML workloads. With a single command, you get:

- **3-node Minikube cluster** with GPU support
- **Infrastructure tier**: Cilium CNI, Cert-Manager, Envoy Gateway, NVIDIA GPU Operator
- **Platform tier**: Keycloak (IAM), Kyverno (policies), Victoria Metrics/Logs (observability)
- **Application tier**: llm-d (LLM serving), Open WebUI (chat interface), HELIX

## Installation

### Prerequisites

Install these tools before running NOVA:

| Tool | Version | Installation |
|------|---------|--------------|
| Docker | 24.0+ | [docs.docker.com](https://docs.docker.com/get-docker/) |
| Minikube | 1.32+ | [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/) |
| mkcert | latest | [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert#installation) |
| certutil | - | `apt install libnss3-tools` (Ubuntu/Debian) |

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

## Commands

| Command | Description |
|---------|-------------|
| `nova setup` | One-time setup: check dependencies, configure DNS, generate CA |
| `nova start [--tier=N]` | Start lab up to tier N (1, 2, or 3). Default: 3 |
| `nova stop` | Stop lab, preserve state |
| `nova delete [--purge]` | Delete lab. `--purge` removes config/certs |
| `nova kubectl ...` | Run kubectl against NOVA cluster |
| `nova helm ...` | Run helm commands against NOVA cluster |
| `nova version` | Show version information |

## Architecture

```
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

| Service | URL | Credentials |
|---------|-----|-------------|
| Keycloak | `https://keycloak.nova.local` | `admin` / (see secret) |
| Hubble UI | `https://hubble.nova.local` | - |
| Grafana | `https://grafana.nova.local` | `admin` / `admin` |
| Open WebUI | `https://chat.nova.local` | Create account |

### SSL/TLS Certificates

NOVA uses mkcert to generate a Root CA that's automatically trusted by your browser. No certificate warnings!

## Development

### Build

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Build
mage build

# Run tests
mage test

# Install to $GOPATH/bin
mage install

# Clean
mage clean
```

### Project Structure

```
nova/
├── cmd/nova/           # Entry point
├── internal/
│   ├── cmd/            # Cobra commands
│   ├── config/         # Configuration
│   ├── deployer/       # Tier deployment
│   ├── docker/         # Docker SDK wrapper
│   ├── helm/           # Helm SDK wrapper
│   ├── k8s/            # Kubernetes client
│   ├── minikube/       # Minikube wrapper
│   ├── preflight/      # Dependency checks
│   ├── pki/            # Certificate ops
│   ├── dns/            # DNS configuration
│   └── ui/             # Terminal UI
├── pkg/resources/      # Embedded resources
├── resources/          # Helm values, manifests
└── magefiles/          # Build targets
```

## License

Apache 2.0

---

Made with ❤️ by [@kzgrzendek](https://github.com/kzgrzendek)
