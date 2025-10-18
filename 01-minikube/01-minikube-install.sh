#!/bin/bash

##############################################################################################################
# Name: 01-minikube-install.sh                                                                               #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a multi-node local Minikube cluster.                               #
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

# Bootstraping critical elements

## Minikube cluster creation
echo -e "[INFO] Stating Minikube cluster..."
minikube start \
    --install-addons=false \
    --driver docker \
    --docker-env TZ=Europe/Paris \
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

echo -e "\n[INFO] Script terminated successfully!"