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
kubectl label namespace cert-manager trust-manager/inject-lab-ca-secret=enabled

kubectl -n cert-manager apply -R -f ./resources/cert-manager/secrets
helm upgrade cert-manager jetstack/cert-manager \
  --install \
  --version v1.19.1 \
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


### Gateway
echo -e "\n[INFO] Deploying cluster gateway..."
kubectl create namespace envoy-gateway-system --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace envoy-gateway-system  trust-manager/inject-lab-ca-secret=enabled

helm template envoy-gateway-crds oci://docker.io/envoyproxy/gateway-crds-helm \
  --server-side \
  --namespace envoy-gateway-system \
  -f ./resources/envoy-gateway/helm/crds.yaml \
| kubectl apply --server-side -f -

helm upgrade envoy-gateway oci://docker.io/envoyproxy/gateway-helm \
  --install \
  --namespace envoy-gateway-system \
  -f ./resources/envoy-gateway/helm/gateway.yaml \
  --wait

kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/certificates
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/envoyproxies
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/gatewayclasses
kubectl -n envoy-gateway-system apply -R -f ./resources/envoy-gateway/gateways
echo -e "[INFO] ...done\n"

echo -e "\n[INFO] Tier 1 layer sucessfully deployed.\n"