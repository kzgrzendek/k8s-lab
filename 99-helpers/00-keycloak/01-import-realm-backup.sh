#!/bin/bash

##############################################################################################################
# Name: 00-import-realm-backup.sh                                                                            #
# Version: 1.0                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to import a realm-backup CRD that can be mounted during keycloak setup          #
############################################################################################################## 

echo -e "[INFO] Starting keycloak backup import helper script v1.0..."

echo -e "\n[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
    exit 1
fi

echo -e "[INFO] Applying backup as CRD..."
kubectl -n keycloak apply -R -f ../../01-lab-setup/02-tier2-setup/resources/keycloak/keycloakrealmimports
kubectl -n keycloak wait --for=condition=Done keycloakrealmimports/k8s-lab --timeout=600s
echo -e "[INFO] ...done."

echo -e "\n[INFO] ... done."