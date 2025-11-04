#!/bin/bash

##############################################################################################################
# Name: 00-tier1-setup.sh                                                                                    #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to install the technical components on which the cluster will operate.          #
############################################################################################################## 

echo -e "[INFO] Stating K8S base stack install script v1.0"

echo -e "\n[INFO] Checking if kubectl is installed..."
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
helm repo add k8tz https://k8tz.github.io/k8tz/ --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo add istio https://istio-release.storage.googleapis.com/charts --force-update
helm repo add oauth2-proxy https://oauth2-proxy.github.io/manifests
helm repo update
echo -e "\n[INFO] ...done"

# Base Stack Install

## CNI Cilium installation
### Mounting bpffs
minikube ssh -n minikube "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m02 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m03 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"

### Installing Cilium
helm upgrade cilium cilium/cilium \
    --install \
    --namespace kube-system \
    -f ./resources/cilium/helm/cilium.yaml \
    --set k8sServiceHost=$(minikube ip) \
    --set k8sServicePort=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -E 's|.*:(.*)|\1|') \
    --wait

## CoreDNS configuration
kubectl -n kube-system apply -R -f ./resources/coredns/configmaps


## CSI Hostpath installation
echo -e "[INFO] Enabling csi-hostpath-driver storage class as default..."
minikube addons enable volumesnapshots
minikube addons enable csi-hostpath-driver
kubectl patch storageclass csi-hostpath-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
echo -e "\n[INFO] ...done"


## NVIDIA GPU support
echo -e "[INFO] Enabling NVidia Device Plugin Support..."
minikube addons enable nvidia-device-plugin
echo -e "\n[INFO] ...done"


## K8tz
echo -e "\n[INFO] Installing K8tz Timezone Controller..."

kubectl create namespace k8tz --dry-run=client -o yaml | kubectl apply -f -
helm upgrade k8tz k8tz/k8tz  \
    --install \
    --namespace k8tz \
    -f ./resources/k8tz/helm/k8tz.yaml \
    --wait

echo -e "\n[INFO] ...done"


## Cert Manager
echo -e "\n[INFO] Installing Cert Manager..."
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
kubectl -n cert-manager apply -R -f ./resources/cert-manager/secrets

helm upgrade cert-manager jetstack/cert-manager \
  --install \
  --version v1.19.1 \
  --namespace cert-manager \
  --create-namespace \
  -f ./resources/cert-manager/helm/cert-manager.yaml \
  --wait

kubectl -n cert-manager apply -R -f ./resources/cert-manager/clusterissuers
echo -e "\n[INFO] ...done"

## Trust Manager
echo -e "\n[INFO] Installing Trust Manager..."
helm upgrade trust-manager jetstack/trust-manager \
  --install \
  --namespace cert-manager \
  -f ./resources/trust-manager/helm/trust-manager.yaml \
  --wait

kubectl -n cert-manager apply -R -f ./resources/trust-manager/bundles
echo -e "\n[INFO] ...done"

## Istio CSR

#### Injecting trust bundle as generic secret for it to be used by Istio CSR
kubectl label namespace cert-manager trust-manager/inject=enabled

#### Initialising Istio Namespace
kubectl create namespace istio-system --dry-run=client -o yaml | kubectl apply -f -

#### Deploying the chart
echo -e "\n[INFO] Installing Cert Manager Istio CSR..."

helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  -f ./resources/cert-manager/istio-csr/helm/istio-csr.yaml  \
  --wait
echo -e "\n[INFO] ...done"


## Istio
echo -e "\n[INFO] Installing Istio ..."

### Base components
echo -e "\n[INFO] Installing Istio base components..."
helm upgrade istio-base istio/base \
  --install \
  --namespace istio-system \
  --wait

### K8S Gateway CRDs
echo -e "\n[INFO] Installing K8S Gateway CRDs..."
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0/standard-install.yaml

### Control plane
echo -e "\n[INFO] Installing Istio Control Plane..."
helm upgrade istiod istio/istiod \
  --install \
  --namespace istio-system \
  -f ./resources/istio/helm/control-plane.yaml \
  --wait

### CNI Node Agent
echo -e "\n[INFO] Installing Istio CNI Node Agent..."
helm upgrade istio-cni istio/cni \
  --install \
  --namespace istio-system \
  -f ./resources/istio/helm/cni-node-agent.yaml \
  --wait
  echo -e "\n[INFO] ...done"

### ZTunnel
echo -e "\n[INFO] Installing Istio ZTunnel..."
helm upgrade ztunnel istio/ztunnel \
  --install \
  --namespace istio-system \
  --wait
echo -e "\n[INFO] ...done"


# ## OAuth2-Proxy
# kubectl create namespace oauth2-proxy --dry-run=client -o yaml | kubectl apply -f -

# echo -e "\n[INFO] Installing OAuth2-Proxy..."
# helm upgrade oauth2-proxy oauth2-proxy/oauth2-proxy \
#   --install \
#   --namespace oauth2-proxy \
#   --wait

echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Script terminated successfully!"