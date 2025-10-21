#!/bin/bash

##############################################################################################################
# Name: nginx-install.sh                                                                                     #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to bootstrap a nginx instance to handle the traffic routing to the k8s local    #
#              lab.                                                                                          #
############################################################################################################## 

# set -eup

echo -e "[INFO] Stating NGINX provisioning script v1.0"
docker rm -f nginx-minikube > /dev/null 2>&1

docker run \
    --detach \
    --name nginx-minikube \
    --network minikube \
    --env TZ=Europe/Paris \
    --publish 80:80 \
    --publish 443:443 \
    --volume ./resources/nginx.conf:/etc/nginx/nginx.conf \
    --volume ./resources/conf.d:/etc/nginx/conf.d \
    nginx:stable-alpine3.21-perl

echo -e "\n[INFO] Script terminated successfully!"