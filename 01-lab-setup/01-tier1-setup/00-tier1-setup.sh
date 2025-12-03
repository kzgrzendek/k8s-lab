#!/bin/bash

##############################################################################################################
# Name: 00-tier1-setup.sh                                                                                    #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to install the technical components on which the cluster will operate.          #
############################################################################################################## 

echo -e "[INFO] Starting K8S base stack install script v1.0"

echo -e "[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
    exit 1
fi

echo -e "\n[INFO] Checking if helm is installed..."
if command -v helm &>/dev/null; then
    echo -e "[INFO] ...helm is installed."
else
    echo -e "[ERROR] ...helm is not installed! Please follow these instructions and launch the script again : https://helm.sh/docs/intro/install/"
    exit 1
fi

echo -e "\n[INFO] Adding Helm repositories..."
helm repo add cilium https://helm.cilium.io/ --force-update
helm repo add nvidia https://helm.ngc.nvidia.com/nvidia --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo add dandydev https://dandydeveloper.github.io/charts --force-update
helm repo update
echo -e "[INFO] ...done"

# Base Stack Install

## CNI Cilium installation
echo -e "\n[INFO] Installing Cilium CNI..."

### Installing Cilium
helm upgrade cilium cilium/cilium \
    --install \
    --namespace kube-system \
    -f ./resources/cilium/helm/cilium.yaml \
    --set k8sServiceHost=$(minikube ip) \
    --set k8sServicePort=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -E 's|.*:(.*)|\1|') \
    --wait
echo -e "[INFO] ...done."

## CoreDNS configuration
echo -e "\n[INFO] Updating Core DNS configuration..."
kubectl -n kube-system apply -R -f ./resources/coredns/configmaps
echo -e "[INFO] ...done."


## Local Path CSI Storage Provisioner support
echo -e "\n[INFO] Deploying Local Hostpath CSI Provisioner..."
kubectl create namespace local-path-storage --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f \
  https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml

kubectl apply \
  --namespace local-path-storage \
  -f ./resources/local-path-provisioner/storageclasses/standard.yaml

kubectl patch configmap local-path-config \
  --namespace local-path-storage \
  --patch-file ./resources/local-path-provisioner/patches/storage-dir.yaml
echo -e "[INFO] ...done."


## NVIDIA GPU support
echo -e "\n[INFO] Deploying NVidia GPU Operator..."
kubectl create namespace nvidia-gpu-operator --dry-run=client -o yaml | kubectl apply -f -
helm upgrade nvidia-gpu-operator nvidia/gpu-operator \
    --install \
    --namespace nvidia-gpu-operator \
    -f ./resources/nvidia-gpu-operator/helm/operator.yaml \
    --wait
echo -e "[INFO] ...done."


## Cert Manager
echo -e "\n[INFO] Installing Cert Manager..."
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace cert-manager trust-manager/inject-lab-ca-secret=enabled

kubectl -n cert-manager apply -R -f ./resources/cert-manager/secrets
helm upgrade cert-manager jetstack/cert-manager \
  --install \
  --namespace cert-manager \
  --create-namespace \
  -f ./resources/cert-manager/helm/cert-manager.yaml \
  --wait

kubectl -n cert-manager wait --for=condition=available deployment/cert-manager-webhook --timeout=300s
kubectl -n cert-manager apply -R -f ./resources/cert-manager/clusterissuers
echo -e "[INFO] ...done"

## Trust Manager
echo -e "\n[INFO] Installing Trust Manager..."
helm upgrade trust-manager jetstack/trust-manager \
  --install \
  --namespace cert-manager \
  -f ./resources/trust-manager/helm/trust-manager.yaml \
  --wait

kubectl -n cert-manager wait --for=condition=available deployment/trust-manager --timeout=300s
kubectl -n cert-manager apply -R -f ./resources/trust-manager/bundles

echo -e "\n[INFO] Enabling Cilium Envoy L7 feature with CA Injection..."
kubectl label namespace kube-system trust-manager/inject-lab-ca-secret=enabled
helm upgrade cilium cilium/cilium \
    --namespace kube-system \
    --reuse-values \
    -f ./resources/trust-manager/helm/cilium-envoy-mount-ca.yaml \
    --wait
echo -e "[INFO] ...done."


## Gateways
### Envoy AI Gateway
echo -e "\n[INFO] Deploying Envoy AI Gateway..."
kubectl create namespace envoy-ai-gateway-system --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace envoy-gateway-system  trust-manager/inject-lab-ca-secret=enabled

#### Inference Extension CRDs
kubectl apply -f \
  https://github.com/kubernetes-sigs/gateway-api-inference-extension/releases/download/v1.2.0/manifests.yaml

#### IA Gateway CRDs
helm upgrade aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm \
  --install \
  --namespace envoy-ai-gateway-system \
  --wait

#### Envoy IA Gaeway CRDs
helm upgrade aieg oci://docker.io/envoyproxy/ai-gateway-helm \
  --install \
  --namespace envoy-ai-gateway-system \
  -f ./resources/envoy-ai-gateway/helm/ai-gateway.yaml \
  --wait
echo -e "[INFO] ...done\n"

### Envoy Gateway
#### Envoy Redis backend
echo -e "\n[INFO] Deploying Redis Envoy Gateway Backend..."
kubectl create namespace envoy-gateway-system --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace envoy-gateway-system  trust-manager/inject-lab-ca-secret=enabled

#### Gateway CRDs
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/redis/secrets
helm upgrade redis dandydev/redis-ha \
  --install \
  --namespace envoy-gateway-system \
  -f ./resources/envoy-gateway/redis/helm/redis.yaml \
  --wait
echo -e "[INFO] ...done\n"

#### Envoy Gateway deployment
echo -e "\n[INFO] Deploying Envoy Gateway..."
helm template envoy-gateway-crds oci://docker.io/envoyproxy/gateway-crds-helm \
  --server-side \
  --namespace envoy-gateway-system \
  -f ./resources/envoy-gateway/helm/crds.yaml \
| kubectl apply --server-side -f -

helm upgrade envoy-gateway oci://docker.io/envoyproxy/gateway-helm \
  --install \
  --namespace envoy-gateway-system \
  -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/main/manifests/envoy-gateway-values.yaml \
  -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/main/examples/token_ratelimit/envoy-gateway-values-addon.yaml \
  -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/main/examples/inference-pool/envoy-gateway-values-addon.yaml \
  -f ./resources/envoy-gateway/helm/gateway.yaml \
  --wait

kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/certificates
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/envoyproxies
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/gatewayclasses
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/gateways
echo -e "[INFO] ...done\n"

echo -e "\n[INFO] Tier 1 layer sucessfully deployed.\n"