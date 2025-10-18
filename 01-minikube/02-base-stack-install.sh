#!/bin/bash

##############################################################################################################
# Name: 02-base-stack-install.sh                                                                             #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to install the technical components on which the cluster will operate.          #
############################################################################################################## 

set -eup

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
helm repo update
echo -e "\n[INFO] ...done"

# Base Stack Install

## CNI Cilium installation
echo -e "\n[INFO] Installing Cilium CNI..."

# Mounting bpffs
minikube ssh -n minikube "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m02 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m03 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"

helm upgrade cilium cilium/cilium \
    --install \
    --version 1.15.2 \
    --namespace kube-system \
    -f ./resources/cilium/helm/cilium.yaml \
    --set k8sServiceHost=$(minikube ip) \
    --set k8sServicePort=$(kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed -E 's|.*:(.*)|\1|') \
    --wait

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

echo -e "\n[INFO] Script terminated successfully!"