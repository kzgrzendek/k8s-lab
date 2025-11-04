#!/bin/bash

##############################################################################################################
# Name: 00-export-realm-backup.sh                                                                          #
# Version: 1.0                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to generate a realm-backup CRD that can be mounted during keycloak setup        #
############################################################################################################## 

echo -e "[INFO] Stating keycloak backup export helper script v1.0..."

echo -e "\n[INFO] Checking if yq is installed..."
if command -v yq &>/dev/null; then
    echo -e "[INFO] ...yq is installed."
else
    echo -e "[ERROR] ...yq is not installed! Please follow these instructions and launch the script again : https://github.com/mikefarah/yq?tab=readme-ov-file#install"
    exit 1
fi

echo -e "\n[INFO] Generating backup..."
kubectl exec \
  -n keycloak \
  -c keycloak keycloak-0 \
  -- sh -c \
    "/opt/keycloak/bin/kc.sh export \
      --realm k8s-lab \
      --file /tmp/kc-backup.json \
     > /dev/null 2>&1;  \
     cat /tmp/kc-backup.json" \
     | yq eval '
      {
        "apiVersion": "k8s.keycloak.org/v2alpha1",
        "kind": "KeycloakRealmImport",
        "metadata": {
          "name": "k8s-lab-import",
          "namespace": "keycloak"
        },
        "spec": {
          "keycloakCRName": "keycloak",
          "realm": .
        }
      }
     ' \
> ../../01-lab-setup/02-tier2-setup/resources/keycloak/keycloakrealmimports/k8s-lab.yaml

echo -e "\n[INFO] ...done."

echo -e "\n[INFO] Script terminated successfully!"