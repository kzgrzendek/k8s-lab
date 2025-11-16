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
helm repo add k8tz https://k8tz.github.io/k8tz/ --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo add istio https://istio-release.storage.googleapis.com/charts --force-update
helm repo update
echo -e "[INFO] ...done"

# Base Stack Install

## CNI Cilium installation
echo -e "\n[INFO] Installing Cilium CNI with Hubble..."

### Installing Cilium
helm upgrade cilium cilium/cilium \
    --install \
    --namespace kube-system \
    -f ./resources/cilium/helm/cilium.yaml \
    --set k8sServiceHost=$(minikube ip) \
    --set k8sServicePort=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -E 's|.*:(.*)|\1|') \
    --wait
echo -e "[INFO] ...done."

echo -e "\n[INFO] Updating Core DNS configuration..."
## CoreDNS configuration
kubectl -n kube-system apply -R -f ./resources/coredns/configmaps
echo -e "[INFO] ...done."

## CSI Hostpath installation
echo -e "\n[INFO] Enabling csi-hostpath-driver storage class as default..."
minikube addons enable volumesnapshots
minikube addons enable csi-hostpath-driver
kubectl patch storageclass csi-hostpath-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
echo -e "[INFO] ...done."


## NVIDIA GPU support
echo -e "\n[INFO] Enabling NVidia Device Plugin Support..."
minikube addons enable nvidia-device-plugin
echo -e "[INFO] ...done."


## K8tz
echo -e "\n[INFO] Installing K8tz Timezone Controller..."
kubectl create namespace k8tz --dry-run=client -o yaml | kubectl apply -f -
helm upgrade k8tz k8tz/k8tz  \
    --install \
    --namespace k8tz \
    -f ./resources/k8tz/helm/k8tz.yaml \
    --wait
echo -e "[INFO] ...done."


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
echo -e "[INFO] ...done"

## Trust Manager
echo -e "\n[INFO] Installing Trust Manager..."
helm upgrade trust-manager jetstack/trust-manager \
  --install \
  --namespace cert-manager \
  -f ./resources/trust-manager/helm/trust-manager.yaml \
  --wait

kubectl -n cert-manager apply -R -f ./resources/trust-manager/bundles
kubectl label namespace cert-manager trust-manager/inject-secret=enabled
echo -e "[INFO] ...done."

## Istio CSR
echo -e "\n[INFO] Deploying Istio CSR component..."

#### Initialising Istio Namespace and setuping the CAs Certs and Issuers
kubectl create namespace istio-system --dry-run=client -o yaml | kubectl apply -f -
kubectl -n istio-system apply -R -f ./resources/cert-manager/istio-csr/certificates
kubectl -n istio-system apply -R -f ./resources/cert-manager/istio-csr/issuers
echo -e "[INFO] ...done."

#### Deploying the chart
echo -e "\n[INFO] Installing Cert Manager Istio CSR..."

kubectl -n cert-manager create secret generic istio-root-ca \
    --from-literal=ca.pem="$(kubectl -n istio-system get secret istio-ca -ogo-template='{{index .data "tls.crt"}}' | base64 -d)"

helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  -f ./resources/cert-manager/istio-csr/helm/istio-csr.yaml \
  --wait
echo -e "[INFO] ...done."


## Istio
echo -e "\n[INFO] Installing Istio ..."


### Base components setup
echo -e "\n[INFO] Installing Istio base components..."
helm upgrade istio-base istio/base \
  --install \
  --namespace istio-system \
  --wait

### K8S Gateway CRDs
echo -e "\n[INFO] Installing K8S Gateway CRDs..."
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.4.0/experimental-install.yaml

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
echo -e "[INFO] ...done\n"

### ZTunnel
echo -e "\n[INFO] Installing Istio ZTunnel..."
helm upgrade ztunnel istio/ztunnel \
  --install \
  --namespace istio-system \
  --wait
echo -e "[INFO] ...done\n"

### Gateway
echo -e "\n[INFO] Deploying cluster gateway..."
kubectl create namespace istio-gateway --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace istio-gateway trust-manager/inject-secret=enabled
kubectl -n istio-gateway apply -R -f ./resources/istio/certificates
kubectl -n istio-gateway apply -R -f ./resources/istio/configmaps
kubectl -n istio-gateway apply -R -f ./resources/istio/gateways
echo -e "[INFO] ...done\n"


## Hubble
echo -e "\n[INFO] Installing Hubble..."
helm upgrade cilium cilium/cilium \
    --install \
    --namespace kube-system \
    -f ./resources/hubble/helm/hubble.yaml \
    --wait
kubectl -n kube-system apply -R -f ./resources/hubble/httproutes   
echo -e "[INFO] ...done."

echo -e "\n[INFO] Tier 1 layer sucessfully deployed.\n"