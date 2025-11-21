#!/bin/bash

##############################################################################################################
# Name: 00-start-k8s-lab-bind9-dns.sh                                                                                #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a bind9 instance to handle the domain names tied to the k8s        #
#              local lab.                                                                                    #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting K8S Lab BIND9 DNS Server provisioning script v1.0"
docker rm -f k8s-lab-bind9-dns > /dev/null 2>&1

docker run \
  --detach \
  --name k8s-lab-bind9-dns \
  --publish 30053:53/tcp \
  --publish 30053:53/udp \
  --env BIND9_USER=bind \
  --env TZ=Europe/Paris \
  --volume ./resources/named.conf:/etc/bind/named.conf \
  --volume ./resources/db.lab.k8s.local:/etc/bind/zones/db.lab.k8s.local \
  --volume ./resources/db.lab.k8s.local:/etc/bind/zones/db.auth.k8s.local \
  --cap-add=NET_ADMIN \
  ubuntu/bind9

echo -e "\n[INFO] K8S Lab BIND9 DNS Server successfully started. \n"