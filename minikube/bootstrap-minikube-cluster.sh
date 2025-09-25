#!/bin/bash

##############################################################################################################
# Name: bootstrap-minikube-cluster.sh                                                                        #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a multi-node local Minikube cluster enabling IA and data workload, #
#              while maintaining production grade services (CNI, CSI, OIDC, Monitoring and Observability...)    #
############################################################################################################## 

set -eup

echo -e "[INFO] Stating Minkube provisioning script v1.0"

echo -e "\n[INFO] Checking if minikube is installed..."
if command -v minikube &>/dev/null; then
    echo -e "[INFO] ...minikube is installed."
else
    echo -e "[ERROR] ...minikube is not installed! Please follow these instructions and launch the script again : https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

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
helm repo add falcosecurity https://falcosecurity.github.io/charts --force-update
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm repo add coredns https://coredns.github.io/helm --force-update
helm repo update
echo -e "\n[INFO] ...done"

# Bootstraping critical elements

## Minikube cluster creation
echo -e "[INFO] Stating Minikube cluster..."
minikube start \
    --install-addons=false \
    --driver docker \
    --cpus 4 \
    --memory 4096 \
    --container-runtime docker \
    --gpus all \
    --kubernetes-version v1.33.5 \
    --network-plugin cni\
    --cni false \
    --nodes 3 \
    --extra-config kubelet.node-ip=0.0.0.0 \
    --extra-config=kube-proxy.skip-headers=true
echo -e "\n[INFO] ...done"

## CNI Cilium installation
echo -e "\n[INFO] Installing Cilium CNI..."

# Mounting bpffs
minikube ssh -n minikube "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m02 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m03 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"

helm upgrade cilium cilium/cilium \
    --install \
    --version 1.18.2 \
    --namespace kube-system \
    -f ./resources/cilium/helm/base.yaml \
    --set k8sServiceHost=$(minikube ip) \
    --set k8sServicePort=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -E 's|.*:(.*)|\1|') \
    --wait

kubectl apply -f ./resources/cilium/manifests/cilium-lb-ipam.yaml
kubectl apply -f ./resources/cilium/manifests/ciliuml2annoucementpolicy.yaml

kubectl rollout restart -n kube-system deployment/cilium-operator
kubectl rollout status -n kube-system deployment/cilium-operator --timeout=30s

kubectl rollout restart -n kube-system deployment/cilium-operator
kubectl rollout status -n kube-system deployment/cilium-operator --timeout=30s
echo -e "\n[INFO] ...done"

## Cilium Ingress installation
echo -e "\n[INFO] Installing Cilium Ingress Controller..."
helm upgrade cilium cilium/cilium \
   --version 1.18.2 \
   --namespace kube-system \
   --reuse-values \
   -f ./resources/cilium/helm/ingress.yaml \
   --wait
echo -e "\n[INFO] ...done"

## Cilium Hubble installation
echo -e "\n[INFO] Installing Cilium Hubble..."
helm upgrade cilium cilium/cilium \
   --version 1.18.2 \
   --namespace kube-system \
   --reuse-values \
   -f ./resources/cilium/helm/hubble.yaml \
   --wait
echo -e "\n[INFO] ...done"

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

# Installing security stack

## Kyverno
echo -e "\n[INFO] Installing Kyverno..."
helm upgrade kyverno kyverno/kyverno \
    --install \
    --namespace kyverno\
    --create-namespace \
    --wait
echo -e "\n[INFO] ...done"


## Falco
echo -e "\n[INFO] Installing Falco..."
helm upgrade falco falcosecurity/falco \
    --install \
    --namespace falco \
    --create-namespace \
    --set tty=true \
    --wait
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Script terminated successfully!"