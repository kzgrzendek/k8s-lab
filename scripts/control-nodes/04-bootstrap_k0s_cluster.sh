#!/bin/bash

#########################################################################################
# Name: 04-bootstrap_k0s_cluster.sh                                                     #
# Version: 0.1                                                                          #
# Author: Kévin ZGRZENDEK (@kzgrzendek)                                                 #
# Purpose: Setup necessary charts and applications to provide with a functionnal        #
#          k0s cluster.                                                                 #
#          To be used as a Vagrant provisioning script.                                 #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S control node cluster bootstraping script version 0.1"

## Cilium install
echo -e "\n[INFO] : Deploying Cilium with Hubble..."

cilium install \
    --set kubeProxyReplacement=true \
    --set ingressController.enabled=true \
    --set ingressController.loadbalancerMode=dedicated \
    --wait
cilium hubble enable

echo -e "\n[INFO] : ...done."

## Tetragon install
echo -e "\n[INFO] : Deploying Tetragon Chart..."

helm repo add cilium https://helm.cilium.io
helm repo update
helm install tetragon cilium/tetragon -n kube-system --wait

echo -e "\n[INFO] : ...done."

# ## Victoria Metrics K8S Stack install
# echo -e "\n[INFO] : Installing Victoria Metrics K8S Stack..."

# echo -e "\n[INFO] : ...done."

## Victoria Logs install
# echo -e "\n[INFO] : Installing Victoria Logs..."
# kubectl create namespace vmks
# helm install vmks oci://ghcr.io/victoriametrics/helm-charts/victoria-metrics-k8s-stack -n vmks --wait
# echo -e "\n[INFO] : ...done."


echo -e "\n[INFO] : Script terminated successfully"


