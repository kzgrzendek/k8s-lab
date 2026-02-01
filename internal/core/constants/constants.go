// Package constants defines commonly used constants across NOVA.
// This centralizes magic strings, URLs, ports, and other values that
// may need to be updated or referenced in multiple places.
package constants

import (
	"github.com/kzgrzendek/nova/internal/tools/crypto"
)

// --- Helm Repository URLs ---
const (
	HelmRepoCilium          = "https://helm.cilium.io/"
	HelmRepoNvidia          = "https://helm.ngc.nvidia.com/nvidia"
	HelmRepoIntel           = "https://intel.github.io/helm-charts/"
	HelmRepoNFD             = "https://kubernetes-sigs.github.io/node-feature-discovery/charts"
	HelmRepoFalco           = "https://falcosecurity.github.io/charts"
	HelmRepoJetstack        = "https://charts.jetstack.io"
	HelmRepoDandyDev        = "https://dandydeveloper.github.io/charts"
	HelmRepoKyverno         = "https://kyverno.github.io/kyverno/"
	HelmRepoVictoriaMetrics = "https://victoriametrics.github.io/helm-charts/"
	HelmRepoEnvoyGateway    = "oci://docker.io/envoyproxy/gateway-helm"
	HelmRepoEnvoyAIGateway  = "oci://docker.io/envoyproxy/ai-gateway-helm"
	HelmRepoAPHPHelix       = "https://aphp.github.io/HELIX"
	HelmRepoLLMD            = "https://llm-d-incubation.github.io/llm-d-modelservice"
	HelmRepoOpenWebUI       = "https://helm.openwebui.com/"

	// LaSuite repos (French government AI stack)
	HelmRepoOpenGateLLM   = "https://etalab-ia.github.io/opengatellm-helm"
	HelmRepoConversations = "https://suitenumerique.github.io/conversations"

	// Infrastructure operators repos
	HelmRepoOTRedis = "https://ot-container-kit.github.io/helm-charts/" // OT-CONTAINER-KIT Redis Operator
	HelmRepoGarage  = "https://garagehq.deuxfleurs.fr/helm-charts/"     // Garage S3-compatible storage
	HelmRepoQdrant  = "https://qdrant.github.io/qdrant-helm"            // Qdrant vector database
)

// --- Kubernetes Manifest URLs ---
const (
	// ManifestLocalPathProvisioner is the URL for the local-path-provisioner manifest
	ManifestLocalPathProvisioner = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.33/deploy/local-path-storage.yaml"

	// ManifestGatewayAPIInferenceExtension is the URL for the Gateway API Inference Extension CRDs
	ManifestGatewayAPIInferenceExtension = "https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v1.2.1/manifests.yaml"

	// Keycloak Operator manifests (version 26.4.7)
	ManifestKeycloakCRD            = "https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.7/kubernetes/keycloaks.k8s.keycloak.org-v1.yml"
	ManifestKeycloakRealmImportCRD = "https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.7/kubernetes/keycloakrealmimports.k8s.keycloak.org-v1.yml"
	ManifestKeycloakOperator       = "https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.7/kubernetes/kubernetes.yml"
)

// --- Network Ports ---
const (
	// HTTPPort is the standard HTTP port
	HTTPPort = 80

	// HTTPSPort is the standard HTTPS port
	HTTPSPort = 443

	// DNSPort is the standard DNS port
	DNSPort = 53

	// Bind9Port is the port where NOVA's Bind9 DNS server listens
	Bind9Port = 30053

	// MinikubeIngressHTTPSPort is the NodePort where Minikube ingress listens for HTTPS
	MinikubeIngressHTTPSPort = 30443

	// KubernetesAPIPort is the default Kubernetes API server port
	KubernetesAPIPort = 8443
)

// --- Container Images ---
const (
	ImageBind9    = "ubuntu/bind9:latest"
	ImageNginx    = "nginx:stable-alpine3.21-perl"
	ImageRegistry = "registry:2.8.3"
)

// --- Container Names ---
const (
	ContainerBind9    = "nova-bind9-dns"
	ContainerNginx    = "nova-nginx-gateway"
	ContainerRegistry = "nova-registry"
)

// --- Registry Configuration ---
const (
	RegistryPort   = 5000
	RegistryDomain = "registry.local"
	RegistryHost   = "registry.local:5000"
)

// --- Mount Configuration ---
const (
	// MountPointModels is the path where models are mounted inside minikube nodes.
	// This path is used by minikube mount to expose host models to the cluster.
	MountPointModels = "/mnt/nova/models"
)

// --- Namespaces ---
const (
	// Tier 1 namespaces
	NamespaceLocalPathStorage  = "local-path-storage"
	NamespaceFalco             = "falco"
	NamespaceCertManager       = "cert-manager"
	NamespaceEnvoyGateway      = "envoy-gateway-system"
	NamespaceEnvoyAIGateway    = "envoy-ai-gateway-system"
	NamespaceNvidiaGPUOperator = "nvidia-gpu-operator"

	// Tier 2 namespaces
	NamespaceKyverno         = "kyverno"
	NamespaceKeycloak        = "keycloak"
	NamespaceVictoriaLogs    = "victorialogs"
	NamespaceVictoriaMetrics = "victoriametrics"

	// Infrastructure operator namespaces
	NamespaceCNPG          = "cnpg-system"     // CloudNative PG operator
	NamespaceRedisOperator = "redis-operator"  // OT-CONTAINER-KIT Redis operator
	NamespaceGarage        = "garage"          // Garage S3-compatible storage

	// Tier 3 namespaces
	NamespaceLLMD      = "llmd"
	NamespaceOpenWebUI = "openwebui"
	NamespaceHelix     = "helix"

	// LaSuite namespaces (French government AI stack)
	NamespaceOpenGateLLM   = "opengatellm"
	NamespaceConversations = "conversations"

	// Vector database namespaces
	NamespaceQdrant = "qdrant" // Qdrant vector database
)

// --- Storage Classes ---
const (
	StorageClassLocalPath = "local-path"
)

// --- Installation Hints (URLs for documentation) ---
const (
	InstallHintDocker   = "https://docs.docker.com/get-docker/"
	InstallHintMinikube = "https://minikube.sigs.k8s.io/docs/start/"
	InstallHintMkcert   = "https://github.com/FiloSottile/mkcert#installation"
)

// --- Timeouts (in seconds) ---
const (
	// DefaultHelmTimeout is the default timeout for Helm operations
	DefaultHelmTimeout = 600

	// ExtendedHelmTimeout is used for longer operations like GPU operator
	ExtendedHelmTimeout = 900
)

// --- Kubernetes Labels ---
const (
	LabelGPU                 = "nvidia.com/gpu"
	LabelControlPlane        = "node-role.kubernetes.io/control-plane"
	LabelMaster              = "node-role.kubernetes.io/master" // Legacy label for older K8s versions
	LabelServiceType         = "service-type"
	LabelTrustManagerInject  = "trust.cert-manager.io/inject-ca-bundle"
	LabelNodeTypeGPU         = "nova.local/node-type"
	LabelGPUOperands         = "nvidia.com/gpu.deploy.operands"
	LabelPodSecurityStandard = "pod-security.kubernetes.io/enforce"
)

// --- Taints ---
const (
	TaintGPU              = "nvidia.com/gpu"
	TaintEffectNoSchedule = "NoSchedule"
)

// --- Wait Operation Timeouts (in seconds) ---
const (
	DefaultWaitTimeout      = 120
	PodReadyTimeout         = 180
	DeploymentReadyTimeout  = 300
	StatefulSetReadyTimeout = 300
	SecretReadyTimeout      = 120
	EndpointReadyTimeout    = 120
	ConditionCheckTimeout   = 600
	LongOperationTimeout    = 3600
)

// --- Retry Settings ---
const (
	DefaultMaxRetries        = 5
	DefaultRetryInitialDelay = 3  // seconds
	DefaultCheckInterval     = 5  // seconds
	FastCheckInterval        = 2  // seconds
	SlowCheckInterval        = 10 // seconds
)

// --- OIDC Clients ---
// OIDCClient represents an OIDC client configuration with ID and secret.
type OIDCClient struct {
	ID     string
	Secret string
}

// OIDC client configurations for lab applications.
// Secrets are generated once at program startup and remain constant for the session.
var (
	OIDCHubble        OIDCClient
	OIDCGrafana       OIDCClient
	OIDCHelix         OIDCClient
	OIDCOpenWebUI     OIDCClient
	OIDCOpenGateLLM   OIDCClient
	OIDCConversations OIDCClient
)

func init() {
	OIDCHubble = OIDCClient{ID: "hubble", Secret: mustGenerateSecret()}
	OIDCGrafana = OIDCClient{ID: "grafana", Secret: mustGenerateSecret()}
	OIDCHelix = OIDCClient{ID: "helix", Secret: mustGenerateSecret()}
	OIDCOpenWebUI = OIDCClient{ID: "open-webui", Secret: mustGenerateSecret()}
	OIDCOpenGateLLM = OIDCClient{ID: "opengatellm", Secret: mustGenerateSecret()}
	OIDCConversations = OIDCClient{ID: "conversations", Secret: mustGenerateSecret()}
}

// mustGenerateSecret generates a random secret or panics on failure.
func mustGenerateSecret() string {
	secret, err := crypto.GenerateRandomPassword(OIDCSecretLength)
	if err != nil {
		panic("failed to generate OIDC secret: " + err.Error())
	}
	return secret
}

// --- Password/Secret Lengths ---
const (
	DefaultPasswordLength = 32
	OIDCSecretLength      = 32
)

// --- Container Image Pull Settings ---
const (
	DefaultMaxConcurrentDownloads   = 3  // Docker default
	OptimizedMaxConcurrentDownloads = 10 // Optimized value for faster pulls
	SkopeoConcurrentLayers          = 10 // Concurrent layer downloads for skopeo
)
