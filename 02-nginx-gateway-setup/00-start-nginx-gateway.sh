#!/bin/bash

##############################################################################################################
# Name: 00-start-nginx-gateway.sh                                                                            #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to start a nginx instance to handle the traffic routing to the k8s local lab.   #                                                                                          #
############################################################################################################## 

set -eup

echo -e "[INFO] Starting K8S Lab NGINX Gateway provisioning script v1.0"
docker rm -f k8s-lab-nginx-gateway > /dev/null 2>&1

echo -e "[INFO] Templating configuration files"
export MINIKUBE_IP
MINIKUBE_IP=$(minikube ip)
# shellcheck disable=SC2016
envsubst '$MINIKUBE_IP' < resources/templates/nginx.conf.template > resources/config/nginx.conf

echo -e "[INFO] Starting K8S Lab NGINX Gateway container"

docker run \
    --detach \
    --name k8s-lab-nginx-gateway \
    --network minikube \
    --publish 80:80 \
    --publish 443:443 \
    --volume ./resources/config/nginx.conf:/etc/nginx/nginx.conf \
    --volume ./resources/config/conf.d:/etc/nginx/conf.d \
    nginx:stable-alpine3.21-perl

echo -e "[INFO] K8S Lab NGINX Gateway successfully started. \n"