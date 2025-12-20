#!/bin/bash

##############################################################################################################
# Name: 00-stop-lab.sh                                                                                       #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to stop the K8S Lab                                                             #
############################################################################################################## 

set -euo pipefail

echo -e "[INFO] Starting K8S Lab stopping script v1.0 \n"


# Lab Setup
## K8S setup
pushd 01-lab-setup/00-k8s-setup > /dev/null
. 01-stop-k8s.sh
popd > /dev/null


# NGINX Gateway setup
pushd 02-nginx-gateway-setup > /dev/null
. 01-stop-nginx-gateway.sh 
popd > /dev/null


# BIND9 DNS setup
pushd 03-bind9-dns-setup > /dev/null
. 01-stop-bind9-dns.sh
popd > /dev/null

echo -e "[INFO] K8S Lab successfully stopped."