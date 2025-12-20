#!/bin/bash

##############################################################################################################
# Name: 00-stop-k8s.sh                                                                                       #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to stop a multi-node local Minikube cluster.                                    #
############################################################################################################## 

set -euo pipefail

echo -e "[INFO] Starting Minikube stopping script v1.0"

echo -e "\n[INFO] Checking if minikube is installed..."
if command -v minikube &>/dev/null; then
    echo -e "[INFO] ...minikube is installed."
else
    echo -e "[ERROR] ...minikube is not installed! Please follow these instructions and launch the script again : https://minikube.sigs.k8s.io/docs/start/?arch=%2Flinux%2Fx86-64%2Fstable%2Fbinary+download"
    exit 1
fi

echo -e "[INFO] Stopping Minikube instance..."

if minikube status | grep -q "Running"; then
    minikube stop
    echo "[INFO] Minikube cluster stopped."
else
    echo "[INFO] No running minkube cluster has been found."
fi

echo -e "[INFO] Minikube cluster stopped. \n"