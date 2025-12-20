#!/bin/bash

##############################################################################################################
# Name: 00-stop-nginx-gateway.sh                                                                             #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to stop a nginx instance to handle the traffic routing to the k8s local lab.    #                                                                                          #
############################################################################################################## 

set -euo pipefail

echo -e "[INFO] Starting K8S Lab NGINX Gateway stopping script v1.0"

echo -e "[INFO] Stopping K8S Lab NGINX Gateway..."

if docker ps | grep -q "k8s-lab-nginx-gateway"; then
    docker stop k8s-lab-nginx-gateway
    echo "[INFO] K8S Lab NGINX Gateway stopped"
else
    echo "[INFO] No running K8S Lab NGINX Gateway container found."
fi

echo -e "[INFO] K8S Lab NGINX Gateway successfully stopped. \n"