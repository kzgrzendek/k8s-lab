#!/bin/bash

#########################################################################################
# Name: 03-setup_k0s_cluster.sh.sh                                                      #
# Version: 0.1                                                                          #
# Author: Kévin ZGRZENDEK (@kzgrzendek)                                                 #
# Purpose: Setup k0s cluster using k0sctl with SSH.                                     #
#          To be used as a Vagrant provisioning script.                                 #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S control node cluster setup script version 0.1"

# Bootstrapping k0s cluster
echo -e "\n[INFO] : Installing k0s cluster..."

echo -e "\n[INFO] : Control Node IPs: $CONTROL_NODES_IPS"
echo -e "\n[INFO] : Woker Nodes IPs: $WORKER_NODES_IPS"

IFS=',' read -ra array <<< "$CONTROL_NODES_IPS"
for ip in "${array[@]}"; do
    echo -e "[INFO] : Handling K0S Cluster config for Control Node $ip..."
    # yq logic here
    echo -e "\n[INFO] : ...done."
done

IFS=',' read -ra array <<< "$WORKER_NODES_IPS"
for ip in "${array[@]}"; do
    echo -e "[INFO] : Handling K0S Cluster config for Worker Node $ip..."
    # yq logic here
    echo -e "\n[INFO] : ...done."
done

echo -e "\n[INFO] : All set! Now applying K0S Cluster configuration..."
# ## Cluster setup
# mkdir -p /root/.config
# k0sctl apply --config /vagrant/k0sctl/cluster-config.yaml --no-wait
# k0sctl kubeconfig --config=/vagrant/k0sctl/cluster-config.yaml > /root/.kube/config

echo -e "\n[INFO] : ...done."

echo -e "\n[INFO] : Script terminated successfully"


