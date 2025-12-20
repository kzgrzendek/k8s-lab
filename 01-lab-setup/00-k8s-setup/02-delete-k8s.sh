#!/bin/bash

##############################################################################################################
# Name: 00-delete-k8s.sh                                                                                       #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to delete a multi-node local Minikube cluster.                                   #
############################################################################################################## 

set -euo pipefail

echo -e "[INFO] Starting Minikube deleting script v1.0"

echo -e "[INFO] Checking if minikube is installed..."
if command -v minikube &>/dev/null; then
    echo -e "[INFO] ...minikube is installed."
else
    echo -e "[ERROR] ...minikube is not installed! Please follow these instructions and launch the script again : https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

echo -e "[INFO] Deleting Minikube instance..."

if minikube status | grep -q -e "Running" -e "Stopped"; then
    minikube delete --all --purge
    echo "[INFO] Minikube cluster deleted."
else
    echo "[INFO] No running minkube cluster has been found."
fi

echo -e "[INFO] Minikube cluster deleted. \n"