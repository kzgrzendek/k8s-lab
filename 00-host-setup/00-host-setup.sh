#!/bin/bash

################################################################################################################
# Name: 00-host-setup.sh                                                                                       #
# Version: 0.1                                                                                                 #
# Author: @kzgrzendek                                                                                          #
# Description: Helper script to generate the neccessaty resources on top of which the cluster will be deployed #                                                                               #
################################################################################################################

set -eup

echo -e "[INFO] hsot-setup script v0.1"

echo -e "\n[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
    exit 1
fi

echo -e "[INFO] Cleaning old CA Certificates if any..."
rm -f ./resources/certs/k8s-lab-ca.crt
rm -f ./resources/certs/k8s-lab-ca.key

echo -e "[INFO] Generating Root CA Certificate..."
openssl req \
    -new \
    -x509 \
    -config ./resources/certs/k8s-lab-ca.conf \
    -keyout ./resources/certs/k8s-lab-ca.key \
    -out ./resources/certs/k8s-lab-ca.crt \
    -days 3600 \
    -nodes

echo -e "[INFO] Outputting K8S Secret for next steps..."
kubectl create secret tls k8s-lab-cacert \
    --namespace cert-manager \
    --cert=./resources/certs/k8s-lab-ca.crt \
    --key=./resources/certs/k8s-lab-ca.key \
    --dry-run=client \
    -o yaml \
> ../01-lab-setup/01-tier1-setup/resources/cert-manager/secrets/k8s-lab-cacert.yaml

echo -e "\n[INFO] ... done."