#!/bin/bash

################################################################################################################
# Name: 00-delete-k8s-lab-bind9-dns.sh                                                                          #
# Version: 0.1                                                                                                  #
# Author: @kzgrzendek                                                                                           #
# Description: Helper script to delete the bind9 instance to handle the domain names tied to the k8s local lab. #                                                                                    #
#################################################################################################################

set -eup

echo -e "[INFO] Starting K8S Lab BIND9 DNS Server stopping script v1.0"

echo -e "[INFO] Deleting K8S Lab BIND9 DNS Server..."

if docker ps -a | grep -q "k8s-lab-bind9-dns"; then
    docker rm -f k8s-lab-bind9-dns
    echo "[INFO] K8S Lab BIND9 DNS Server deleted"
else
    echo "[INFO] No running K8S Lab BIND9 DNS Server container found."
fi

echo -e "[INFO] K8S Lab BIND9 DNS Server successfully deleted. \n"