#!/bin/bash

##############################################################################################################
# Name: 00-start-lab.sh                                                                                      #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to start the K8S Lab                                                            #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting K8S Lab starting script v1.0 \n"

# Host Setup
pushd 00-host-setup > /dev/null
. 00-host-setup.sh
popd > /dev/null

# Lab Setup
## K8S setup
pushd 01-lab-setup/00-k8s-setup > /dev/null
. 00-start-k8s.sh
popd > /dev/null

## Tier 1 setup
pushd 01-lab-setup/01-tier1-setup > /dev/null
. 00-tier1-setup.sh
popd > /dev/null

## Tier 2 setup
pushd 01-lab-setup/02-tier2-setup > /dev/null
. 00-tier2-setup.sh
popd > /dev/null

# Tier 3 setup
pushd 01-lab-setup/03-tier3-setup.sh > /dev/null
. 00-tier3-setup.sh
popd > /dev/null


# NGINX Gateway setup
pushd 02-nginx-gateway-setup > /dev/null
. 00-start-nginx-gateway.sh 
popd > /dev/null


# BIND9 DNS setup
pushd 03-bind9-dns-setup > /dev/null
. 00-start-bind9-dns.sh
popd > /dev/null

echo -e "\n[INFO] K8S Lab sucessfully started."