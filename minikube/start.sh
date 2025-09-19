#!/bin/bash

set -eup


minikube config set driver docker
minikube config set memory 4096
minikube config set cpus 4

minikube start \
    --container-runtime docker \
    --gpus all \
    --cni=cilium \
    --nodes 3

minikube addons enable volumesnapshots
minikube addons enable csi-hostpath-driver
minikube addons disable storage-provisioner
minikube addons disable default-storageclass

alias kubectl='minikube kubectl --'
kubectl patch storageclass csi-hostpath-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
