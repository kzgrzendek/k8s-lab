#!/bin/bash

###############################################################################################################
# Name: 00-stop-k8s-lab-bind9-dns.sh                                                                                  #
# Version: 0.1                                                                                                #
# Author: @kzgrzendek                                                                                         #
# Description: Helper script to stop the bind9 instance to handle the domain names tied to the k8s local lab. #                                                                                    #
############################################################################################################### 

set -euo pipefail

echo -e "[INFO] Starting K8S Lab BIND9 DNS Server stopping script v1.0"

echo -e "[INFO] Stopping K8S Lab BIND9 DNS Server..."

if docker ps | grep -q "k8s-lab-bind9-dns"; then
    docker stop k8s-lab-bind9-dns
    echo "[INFO] K8S Lab BIND9 DNS Server stopped"
else
    echo "[INFO] No running K8S Lab BIND9 DNS Server container found."
fi

echo -e "[INFO] K8S Lab BIND9 DNS Server successfully stopped. \n"