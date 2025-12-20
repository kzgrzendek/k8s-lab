#!/bin/bash

################################################################################################################
# Name: 00-host-setup.sh                                                                                       #
# Version: 0.1                                                                                                 #
# Author: @kzgrzendek                                                                                          #
# Description: Helper script to generate the neccessaty resources on top of which the cluster will be deployed #                                                                               #
################################################################################################################

set -euo pipefail

echo -e "[INFO] hsot-setup script v0.1"

echo -e "\n[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux"
    exit 1
fi

echo -e "\n[INFO] Checking if mkcert is installed..."
if command -v mkcert &>/dev/null; then
    echo -e "[INFO] ...mkcert is installed."
else
    echo -e "[ERROR] ...mkcert is not installed! Please follow these instructions and launch the script again : https://github.com/FiloSottile/mkcert?tab=readme-ov-file#installation"
    exit 1
fi

echo -e "\n[INFO] Checking if certutil is installed..."
if command -v certutil &>/dev/null; then
    echo -e "[INFO] ...certutil is installed."
else
    echo -e "[ERROR] ...certutil is not installed! Please install it using OS PAckage Manager"
    exit 1
fi

echo -e "[INFO] Generating Root CA Certificate..."
mkcert -install 

echo -e "[INFO] Generating K8S Secret for next steps..."
kubectl create secret tls k8s-lab-cacert \
    --namespace cert-manager \
    --cert=$(mkcert -CAROOT)/rootCA.pem \
    --key=$(mkcert -CAROOT)/rootCA-key.pem \
    --dry-run=client \
    -o yaml \
> ../01-lab-setup/01-tier1-setup/resources/cert-manager/secrets/k8s-lab-cacert.yaml

echo -e "\n[INFO] ... done."