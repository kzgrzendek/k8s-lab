#!/bin/bash

#########################################################################################
# Name: 02-setup_ssh.sh                                                                 #
# Version: 0.1                                                                          #
# Author: Kévin ZGRZENDEK (@kzgrzendek)                                                 #
# Purpose: Generates an SSH keypair and registers the public key to all the nodes       #
#          that will join the k0s cluster.                                              #
#          To be used as a Vagrant provisioning script.                                 #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S control node SSH setup script v0.1"

# Generating new key pair
echo -e "[INFO] : Generating SSH keypair"
rm -f /root/.ssh/id_ed25519 
ssh-keygen -t ed25519 -C "control-node-k0s" -f /root/.ssh/id_ed25519 -q -N ""

echo -e "[INFO] : The following nodes has been identified to be reachable by SSH : ${CLUSTER_NODES_IPS}"

IFS=',' read -ra array <<< "$CLUSTER_NODES_IPS"
for ip in "${array[@]}"; do
    echo -e "[INFO] : Handling SSH access for the following IP : $ip ..."
    ssh-keyscan -H "$ip" >> /root/.ssh/known_hosts
    sshpass -p 'vagrant' ssh-copy-id -i "/root/.ssh/id_ed25519" "vagrant@$ip"
    echo -e "[INFO] : ... done."
done

echo -e "\n[INFO] : Script terminated successfully"