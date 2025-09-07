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

echo -e "[INFO] : The following nodes has been identified to join the cluster : ${CLUSTER_NODES_IPS}"

# Handling SSH keys installation (needed for k0sctl multi-nodes install)
#echo -e "\n[INFO] : Installing SSH keys on worker nodes..."

## Generating new key pair
# rm -f /root/.ssh/id_ed25519 
# ssh-keygen -t ed25519 -C "control-node-k0s" -f /root/.ssh/id_ed25519 -q -N ""

# ## Adding local control node & worker nodes to known hosts
# ssh-keyscan -H "192.168.56.10" "192.168.56.20" "192.168.56.21" "192.168.56.22" >> /root/.ssh/known_hosts

# ## Copying keys to vagrant user on local control node & worker nodes
# sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.11
# sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.21
# sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.22
# sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.23

# echo -e "\n[INFO] : ...done."

echo -e "\n[INFO] : Script terminated successfully"


