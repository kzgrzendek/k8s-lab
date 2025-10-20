#!/bin/bash

##############################################################################################################
# Name: 03-transversal-stack install.sh                                                                      #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to the applicative stack that will address cross-cutting concerns               #
############################################################################################################## 

set -eup

echo -e "[INFO] Stating K8S transversal stack install script v1.0"

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
helm repo add falcosecurity https://falcosecurity.github.io/charts --force-update
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm repo add vm https://victoriametrics.github.io/helm-charts --force-update
helm repo update
echo -e "\n[INFO] ...done"

# Installing applicative stack

## Kubeflow



echo -e "\n[INFO] Script terminated successfully!"