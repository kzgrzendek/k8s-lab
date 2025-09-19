#!/bin/bash

##############################################################################################################
# Name: bootstrap-minikube-cluster.sh                                                                        #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a multi-node local Minikube cluster enabling IA and data workload, #
#              while mainting production grade services (CNI, CSI, OIDC, Monitoring and Observability...)    #
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
helm repo update
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Configuring Minikube cluster..."
minikube config set driver docker
minikube config set memory 4096
minikube config set cpus 4
echo -e "\n[INFO] ...done"

echo -e "[INFO] Stating Minkube cluster..."
minikube start \
    --container-runtime docker \
    --gpus all \
    --network-plugin=cni\
     --cni=false \
    --nodes 3
echo -e "\n[INFO] ...done"

echo -e "[INFO] Enabling addons..."
minikube addons enable volumesnapshots
minikube addons enable csi-hostpath-driver
minikube addons disable storage-provisioner
minikube addons disable default-storageclass
kubectl patch storageclass csi-hostpath-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Installing Cilium..."
helm upgrade cilium cilium/cilium \
    --install \
    --version 1.18.2 \
    --namespace kube-system \
    -f ./resources/cilium/values.yaml
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Installing Tetragon..."
helm upgrade tetragon cilium/tetragon \
    --install \
    --namespace kube-system 
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Script terminated successfully!"