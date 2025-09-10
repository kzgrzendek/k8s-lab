#!/bin/bash

#########################################################################################
# Name: 01-setup_openebs-storage.sh                                                     #
# Version: 0.1                                                                          #
# Author: Kévin ZGRZENDEK (@kzgrzendek)                                                 #
# Purpose: Setup the logical volume used for openebs storage, on worker nodes.          #
#          To be used as a Vagrant provisioning script.                                 #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S worker node OpenEBS storage volume setup version 0.1"

# Installing needed dependencies