# 🧪 k8s-lab

A production-like Kubernetes lab environment for AI/ML workloads on your laptop.

Deploy a fully-featured, GPU-enabled Minikube cluster with enterprise-grade components in minutes.

## ✨ Features

- **🚀 One-command deployment** - Single script deploys the entire stack
- **🎮 GPU support** - NVIDIA GPU sharing via Docker runtime (no complex passthrough)
- **🔐 Full PKI** - mkcert Root CA + Cert-Manager + Trust-Manager with automatic CA injection
- **🌐 Modern networking** - Cilium CNI with eBPF + Hubble observability
- **🤖 AI/ML ready** - Envoy AI Gateway + llm-d + Gateway API Inference Extension
- **🔒 Security stack** - Falco (runtime) + Kyverno (policies) + Keycloak (IAM)
- **📊 Observability** - Victoria Metrics + Victoria Logs
- **🌍 Local DNS** - Bind9 for `*.lab.local` resolution
- **🚪 External Gateway** - NGINX reverse proxy for host access

## 🏗️ Architecture

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

## 📋 Prerequisites

### Required tools

| Tool | Version | Installation |
|------|---------|--------------|
| Docker | 24.0+ | [docs.docker.com](https://docs.docker.com/engine/install/) |
| Minikube | 1.32+ | [minikube.sigs.k8s.io](https://minikube.sigs.k8s.io/docs/start/) |
| kubectl | 1.30+ | [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |
| Helm | 3.14+ | [helm.sh](https://helm.sh/docs/intro/install/) |
| mkcert | latest | [github.com/FiloSottile/mkcert](https://github.com/FiloSottile/mkcert#installation) |
| certutil | - | `apt install libnss3-tools` (Debian/Ubuntu) |

### GPU support (optional)

For NVIDIA GPU acceleration:

```bash
# 1. NVIDIA Driver (should already be installed)
nvidia-smi

# 2. NVIDIA Container Toolkit
# Ubuntu/Debian/PopOS:
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

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/kzgrzendek/k8s-lab.git
cd k8s-lab

# Deploy everything with a single command
./00-start-lab.sh

# ☕ Grab a coffee, this will take ~10-15 minutes
```

That's it! The script will:
1. Generate a local Root CA with mkcert (trusted by your browser)
2. Start a 3-node Minikube cluster with GPU support
3. Deploy all infrastructure components (Tier 1)
4. Deploy platform services (Tier 2)
5. Deploy AI/ML applications (Tier 3)
6. Start NGINX gateway for external access
7. Start Bind9 DNS for `*.lab.local` resolution

### Step-by-step deployment (optional)

If you prefer incremental deployment:

```bash
# 1. Host setup (mkcert CA generation)
cd 00-host-setup && ./00-host-setup.sh && cd ..

# 2. Start Minikube cluster
cd 01-lab-setup/00-k8s-setup && ./00-start-k8s.sh && cd ../..

# 3. Deploy infrastructure
cd 01-lab-setup/01-tier1-setup && ./00-tier1-setup.sh && cd ../..

# 4. Deploy platform services
cd 01-lab-setup/02-tier2-setup && ./00-tier2-setup.sh && cd ../..

# 5. Deploy applications
cd 01-lab-setup/03-tier3-setup && ./00-tier3-setup.sh && cd ../..

# 6. Start external gateway
cd 02-nginx-gateway-setup && ./00-start-nginx-gateway.sh && cd ..

# 7. Start DNS server
cd 03-bind9-dns-setup && ./00-start-bind9-dns.sh && cd ..
```

## 📦 Components

### Tier 1 - Infrastructure

| Component | Purpose | Namespace |
|-----------|---------|-----------|
| [Cilium](https://cilium.io/) | CNI with eBPF networking | `kube-system` |
| [Falco](https://falco.org/) | Runtime security | `falco` |
| [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/) | GPU scheduling & drivers | `nvidia-gpu-operator` |
| [Cert-Manager](https://cert-manager.io/) | Certificate management | `cert-manager` |
| [Trust-Manager](https://cert-manager.io/docs/trust/trust-manager/) | CA bundle distribution | `cert-manager` |
| [Envoy Gateway](https://gateway.envoyproxy.io/) | Gateway API implementation | `envoy-gateway-system` |
| [Envoy AI Gateway](https://aigateway.envoyproxy.io/) | LLM-aware routing | `envoy-ai-gateway-system` |
| [Local Path Provisioner](https://github.com/rancher/local-path-provisioner) | Dynamic PV provisioning | `local-path-storage` |

### Tier 2 - Platform

| Component | Purpose | Namespace |
|-----------|---------|-----------|
| [Kyverno](https://kyverno.io/) | Policy engine | `kyverno` |
| [Keycloak](https://www.keycloak.org/) | Identity & Access Management | `keycloak` |
| [Hubble](https://docs.cilium.io/en/stable/observability/hubble/) | Network observability | `kube-system` |
| [Victoria Metrics](https://victoriametrics.com/) | Metrics storage & Grafana | `victoriametrics` |
| [Victoria Logs](https://docs.victoriametrics.com/victorialogs/) | Log aggregation | `victorialogs` |

### Tier 3 - Applications

| Component | Purpose | Namespace |
|-----------|---------|-----------|
| [llm-d](https://github.com/llm-d-incubation/llm-d) | LLM serving with vLLM | `llmd` |
| [Open WebUI](https://openwebui.com/) | Chat interface | `openwebui` |
| [HELIX](https://github.com/aphp/HELIX) | APHP AI platform | `helix` |

### Host Layer (Docker containers)

| Component | Purpose | Port |
|-----------|---------|------|
| NGINX Gateway | Reverse proxy to Envoy Gateway | `80`, `443` |
| Bind9 DNS | Local DNS for `*.lab.local` | `53` |
| mkcert CA | Root CA trusted by browsers | - |

## 🎮 GPU Configuration

The lab uses Docker's native GPU support (no VFIO/passthrough required):

```bash
minikube start --driver docker --gpus all
```

**Benefits:**
- ✅ GPU shared between host and containers
- ✅ No session interruption
- ✅ Works on laptops without IOMMU
- ✅ Near-native performance

**GPU node selection:**
The startup script automatically labels one worker node for GPU operands:

```bash
# All nodes get GPU disabled by default
kubectl label nodes --all nvidia.com/gpu.deploy.operands=false

# One worker (or master if no workers) gets GPU enabled
kubectl label <selected-node> nvidia.com/gpu.deploy.operands-
```

## 🔐 Accessing Services

### Automatic DNS resolution

The lab includes a Bind9 DNS server. Configure your system to use it:

```bash
# Option 1: Add to /etc/resolv.conf (temporary)
echo "nameserver 127.0.0.1" | sudo tee -a /etc/resolv.conf

# Option 2: Configure NetworkManager (permanent)
# Add DNS=127.0.0.1 to your connection settings
```

### SSL/TLS Certificates

The lab uses mkcert to generate a Root CA that's automatically trusted by your browser. No certificate warnings! 🎉

The Root CA is generated during `00-host-setup.sh` and injected into the cluster via Cert-Manager.

### Default endpoints

| Service | URL | Credentials |
|---------|-----|-------------|
| Keycloak | `https://keycloak.lab.local` | `admin` / (see secret) |
| Hubble UI | `https://hubble.lab.local` | - |
| Grafana | `https://grafana.lab.local` | `admin` / `admin` |
| Open WebUI | `https://chat.lab.local` | Create account |

### Manual /etc/hosts (if not using Bind9)

```bash
# Add to /etc/hosts
127.0.0.1 keycloak.lab.local hubble.lab.local grafana.lab.local chat.lab.local
```

## 🛠️ Management

```bash
# Stop everything (cluster + containers)
./01-stop-lab.sh

# Start again
./00-start-lab.sh

# Delete everything
./02-delete-lab.sh

# Check status
minikube status
docker ps  # NGINX + Bind9 containers
kubectl get pods -A
```

### Individual component control

```bash
# Stop only Minikube (keep NGINX/Bind9 running)
minikube stop

# Restart NGINX gateway
cd 02-nginx-gateway-setup && ./01-stop-nginx-gateway.sh && ./00-start-nginx-gateway.sh

# Restart Bind9 DNS
cd 03-bind9-dns-setup && ./01-stop-bind9-dns.sh && ./00-start-bind9-dns.sh
```

## 📁 Project Structure

```
k8s-lab/
├── 00-start-lab.sh              # 🚀 Main entry point - deploys everything
├── 01-stop-lab.sh               # Stop cluster & containers
├── 02-delete-lab.sh             # Delete everything
│
├── 00-host-setup/               # Host-level setup
│   └── 00-host-setup.sh         # mkcert CA generation
│
├── 01-lab-setup/                # Kubernetes components
│   ├── 00-k8s-setup/
│   │   └── 00-start-k8s.sh      # Minikube cluster bootstrap
│   ├── 01-tier1-setup/
│   │   ├── 00-tier1-setup.sh    # Infrastructure layer
│   │   └── resources/           # Helm values & manifests
│   │       ├── cilium/
│   │       ├── cert-manager/
│   │       ├── envoy-gateway/
│   │       ├── nvidia-gpu-operator/
│   │       └── ...
│   ├── 02-tier2-setup/
│   │   ├── 00-tier2-setup.sh    # Platform layer
│   │   └── resources/
│   │       ├── keycloak/
│   │       ├── hubble/
│   │       ├── victoriametrics/
│   │       └── ...
│   └── 03-tier3-setup/
│       ├── 00-tier3-setup.sh    # Application layer
│       └── resources/
│           ├── llmd/
│           ├── openwebui/
│           └── helix/
│
├── 02-nginx-gateway-setup/      # External NGINX reverse proxy (Docker)
│   └── 00-start-nginx-gateway.sh
│
├── 03-bind9-dns-setup/          # Local DNS server (Docker)
│   └── 00-start-bind9-dns.sh
│
└── 99-helpers/                  # Utility scripts
    └── 00-keycloak/
```

## 🌐 Network Flow

```
Browser (https://app.lab.local)
    │
    ▼ :53
┌─────────────┐
│   Bind9     │ ──► Resolves *.lab.local → 127.0.0.1
│   (Docker)  │
└─────────────┘
    │
    ▼ :443
┌─────────────┐
│   NGINX     │ ──► Terminates external TLS, proxies to Envoy
│   (Docker)  │
└─────────────┘
    │
    ▼
┌─────────────┐
│   Envoy     │ ──► Gateway API routing, rate limiting
│   Gateway   │
└─────────────┘
    │
    ▼
┌─────────────┐
│   Service   │ ──► Keycloak, Grafana, Open WebUI, etc.
└─────────────┘
```

## ⚠️ Known Limitations

- **Single GPU node**: Only one node receives GPU workloads (by design for laptops)
- **Local storage**: Uses hostPath, not suitable for multi-node persistence
- **Port 53 conflict**: Bind9 may conflict with systemd-resolved (disable it or change port)
- **Port 80/443**: NGINX gateway needs these ports free on localhost

## 🤝 Contributing

Contributions welcome! Please open an issue or PR.

## 📄 License

Apache License 2.0 - See [LICENSE](LICENSE)

---

Made with ❤️ by [@kzgrzendek](https://github.com/kzgrzendek)
