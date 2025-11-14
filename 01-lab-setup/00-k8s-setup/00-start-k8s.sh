#!/bin/bash

##############################################################################################################
# Name: 00-start-k8s.sh                                                                                      #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to start a multi-node local Minikube cluster.                                   #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting Minkube provisioning script v1.0"

echo -e "[INFO] Checking if minikube is installed..."
if command -v minikube &>/dev/null; then
    echo -e "[INFO] ...minikube is installed."
else
    echo -e "[ERROR] ...minikube is not installed! Please follow these instructions and launch the script again : https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

# Bootstraping K8S Cluster - Minikube flavour

## Minikube cluster creation
echo -e "\n[INFO] Starting Minikube cluster..."
minikube start \
    --install-addons=false \
    --driver docker \
    --docker-env TZ=Europe/Paris \
    --cpus 4 \
    --memory 4096 \
    --container-runtime docker \
    --gpus all \
    --kubernetes-version v1.33.5 \
    --network-plugin cni \
    --cni false \
    --nodes 3 \
    --extra-config kubelet.node-ip=0.0.0.0 \
    --extra-config=kube-proxy.skip-headers=true
echo -e "[INFO] ...done"

## Mounting bpffs
echo -e "\n[INFO] Mounting BPFS filesystem into the containers..."
minikube ssh -n minikube "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m02 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
minikube ssh -n minikube-m03 "sudo /bin/bash -c 'grep \"bpffs /sys/fs/bpf\" /proc/mounts || sudo mount bpffs -t bpf /sys/fs/bpf'"
echo -e "[INFO] ...done"

echo -e "[INFO] Minikube cluster deployed. \n"