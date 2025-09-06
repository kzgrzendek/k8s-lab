#!/bin/bash

#########################################################################################
# Name: provision-worker-node.sh                                                        #
# Version: 0.1                                                                          #
# Kévin ZGRZENDEK                                                                       #
# Purpose : Setup necessary tools and binaries to bootstrap K0S cluster worker nodes,   #
#           To be used as a Vagrant provisioning script.                                #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S worker node bootstrapping script v0.1"

echo -e "[INFO] : Mounting OpenEBS Storage disk..."
mkdir -p /var/openebs/local
mkfs.ext4 /dev/sdb
mount /dev/sdb /var/openebs/local
echo -e "[INFO] : ...done."

echo -e "\n[INFO] : Script terminated successfully"