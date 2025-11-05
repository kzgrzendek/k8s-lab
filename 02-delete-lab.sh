#!/bin/bash

##############################################################################################################
# Name: 00-delete-lab.sh                                                                                     #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to delete the K8S Lab                                                           #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting K8S Lab delete script v1.0 \n"


# Lab Setup
## K8S setup
pushd 01-lab-setup/00-k8s-setup > /dev/null
. 02-delete-k8s.sh
popd > /dev/null


# NGINX Gateway setup
pushd 02-nginx-gateway-setup > /dev/null
. 02-delete-nginx-gateway.sh
popd > /dev/null


# BIND9 DNS setup
pushd 03-bind9-dns-setup > /dev/null
. 02-delete-bind9-dns.sh
popd > /dev/null

echo -e "[INFO] K8S Lab sucessfully deleted."