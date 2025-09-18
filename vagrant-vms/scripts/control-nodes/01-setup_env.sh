#!/bin/bash

#########################################################################################
# Name: 01-setup_env.sh                                                                 #
# Version: 0.1                                                                          #
# Author: Kévin ZGRZENDEK (@kzgrzendek)                                                 #
# Purpose: Setup necessary tools and binaries to bootstrap a multi-node K0S cluster,    #
#          from the control node.                                                       #
#          To be used as a Vagrant provisioning script.                                 #
#########################################################################################

set -euo pipefail

echo -e "[INFO] : Starting K0S control node environment setup version 0.1"

# Installing needed dependencies

## Classic apt tools
echo -e "\n[INFO] : Installing apt dependencies..."

apt-get update --yes
apt-get install --yes \
    bash \
    curl\
    openssl \
    sshpass \
    golang \
    git \
    jq \
    findutils \
    coreutils \
    tar

echo -e "\n[INFO] : ...done."


## Helm
echo -e "\n[INFO] : Installing Helm..."

### Retrieving latest version of the installation script from Helm's GitHub repo
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

echo -e "\n[INFO] : ...done."


## k0sctl
echo -e "\n[INFO] : Installing k0sctl..."

### Retrieving latest version from GitHub's API
curl -s "https://api.github.com/repos/k0sproject/k0sctl/releases/latest" | \
    jq '.assets[] | select(.name=="k0sctl-linux-amd64")'.browser_download_url | \
    xargs curl -s -L -o /usr/local/bin/k0sctl
chmod 755 /usr/local/bin/k0sctl

echo -e "\n[INFO] : ...done."

## kubectl
echo -e "\n[INFO] : Installing k0sctl..."

curl -L "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" -o /usr/local/bin/kubectl
chmod 755 /usr/local/bin/kubectl

echo -e "\n[INFO] : ...done."


## Cilium CLI
echo -e "\n[INFO] : Installing Cilium CLI..."

### Retrieving latest version from GitHub's API
curl -s "https://api.github.com/repos/cilium/cilium-cli/releases/latest" | \
    jq '.assets[] | select(.name=="cilium-linux-amd64.tar.gz")'.browser_download_url | \
    xargs curl -s -L | \
    tar xvz -C /usr/local/bin cilium
chmod 755 /usr/local/bin/cilium

echo -e "\n[INFO] : ...done."

echo -e "\n[INFO] : Script terminated successfully"


