#!/bin/bash

##############################################################################################################
# Name: bind9-install.sh                                                                                     #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a bind9 instance to handle the domain names tied to the k8s        #
#              local lab.                                                                                    #
############################################################################################################## 

# set -eup

echo -e "[INFO] Starting bind9 provisioning script v1.0"
docker rm -f bind9-minikube > /dev/null 2>&1

docker run \
  --detach \
  --name bind9-minikube \
  --publish 30053:53/tcp \
  --publish 30053:53/udp \
  --env BIND9_USER=bind \
  --env TZ=Europe/Paris \
  --volume ./resources/named.conf:/etc/bind/named.conf \
  --volume ./resources/db.k8s-lab.local:/etc/bind/zones/db.k8s-lab.local \
  --cap-add=NET_ADMIN \
  ubuntu/bind9

echo -e "\n[INFO] Script terminated successfully!"