#!/bin/bash

#########################################################################################
# Name: provision-control-node.sh                                                       #
# Version: 0.1                                                                          #
# Kévin ZGRZENDEK                                                                       #
# Purpose : Setup necessary tools and binaries to bootstrap a multi-node K0S cluster,   #
#           from the control node. To be used as a Vagrant provisioning script.         #
#########################################################################################

set -euo pipefail


echo -e "[INFO] : Starting K0S control node bootstraping script v0.1"

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

### Retriveing latest version of the installation script from Helm's GitHub repo
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


# Handling SSH keys installation (needed for k0sctl multi-nodes install)
echo -e "\n[INFO] : Installing SSH keys on worker nodes..."

## Generating new key pair
rm -f /root/.ssh/id_ed25519 
ssh-keygen -t ed25519 -C "control-node-k0s" -f /root/.ssh/id_ed25519 -q -N ""

## Adding local control node & worker nodes to known hosts
ssh-keyscan -H "192.168.56.10" "192.168.56.20" "192.168.56.21" "192.168.56.22" >> /root/.ssh/known_hosts

## Copying keys to vagrant user on local control node & worker nodes
sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.10
sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.20
sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.21
sshpass -p 'vagrant' ssh-copy-id -i /root/.ssh/id_ed25519 vagrant@192.168.56.22

echo -e "\n[INFO] : ...done."

# Bootstrapping k0s cluster
echo -e "\n[INFO] : Installing k0s cluster..."

## Cluster setup
mkdir -p /root/.config
k0sctl apply --config /vagrant/k0sctl/cluster-config.yaml --no-wait
k0sctl kubeconfig --config=/vagrant/k0sctl/cluster-config.yaml > /root/.kube/config

echo -e "\n[INFO] : ...done."


## Cilum install
echo -e "\n[INFO] : Deploying Cilium with Hubble..."

# cilium install \
#     --set kubeProxyReplacement=true \
#     --set ingressController.enabled=true \
#     --set ingressController.loadbalancerMode=dedicated \
#     --wait
# cilium hubble enable

echo -e "\n[INFO] : ...done."

## Cilum install
echo -e "\n[INFO] : Deploying Tetragon Chart..."

# helm repo add cilium https://helm.cilium.io
# helm repo update
# helm install tetragon cilium/tetragon -n kube-system --wait

echo -e "\n[INFO] : ...done."


echo -e "\n[INFO] : Script terminated successfully"


