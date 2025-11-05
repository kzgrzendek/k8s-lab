#!/bin/bash

##############################################################################################################
# Name: 00-delete-nginx-gateway.sh                                                                           #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to delete a nginx instance to handle the traffic routing to the k8s local lab.  #                                                                                          #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting NGINX delete script v1.0"

echo -e "[INFO] Deleting NGINX Gateway..."

if docker ps -a | grep -q "k8s-lab-nginx-gateway"; then
    docker rm -f k8s-lab-nginx-gateway
    echo "[INFO] NGINX Gateway deleted"
else
    echo "[INFO] No running NGINX Gateway container found."
fi

echo -e "[INFO] K8S Lab NGINX Gateway successfully deleted. \n"