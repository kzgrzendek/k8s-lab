#!/bin/bash

##############################################################################################################
# Name: 00-tier3-setup.sh                                                                                    #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to the applicative stack that will address cross-cutting concerns               #
############################################################################################################## 

echo -e "[INFO] Starting K8S transversal stack install script v1.0 \n"

echo -e "[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
    exit 1
fi

echo -e "[INFO] Checking if helm is installed..."
if command -v helm &>/dev/null; then
    echo -e "[INFO] ...helm is installed."
else
    echo -e "[ERROR] ...helm is not installed! Please follow these instructions and launch the script again : https://helm.sh/docs/intro/install/"
    exit 1
fi

echo -e "\n[INFO] Adding Helm repositories..."
helm repo add aphp-helix https://aphp.github.io/HELIX --force-update
helm repo add llm-d-modelservice https://llm-d-incubation.github.io/llm-d-modelservice --force-update
helm repo add open-webui https://helm.openwebui.com/ --force-update
helm repo update
echo -e "\n[INFO] ...done"

# Installing applicative stack

## llm-d stack
echo -e "\n[INFO] Installing llm-d Stack..."
kubectl create namespace llmd --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace llmd trust-manager/inject-lab-ca-secret=enabled
kubectl label namespace llmd service-type=llm

### llm-d
helm upgrade llmd llm-d-modelservice/llm-d-modelservice \
    --install \
    --namespace llmd \
    -f ./resources/llmd/helm/llmd.yaml \
    --wait

### Inference Pool
helm upgrade llmd-qwen3-pool oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool \
  --install \
  --version v1.2.1 \
  --namespace llmd \
  -f ./resources/llmd/inferencepools/helm/ip-llmd.yaml

kubectl -n llmd apply -f ./resources/llmd/referencegrants
kubectl -n llmd apply -f ./resources/llmd/inferenceobjectives
kubectl -n llmd apply -f ./resources/llmd/certificates
kubectl -n llmd apply -f ./resources/llmd/gateways
kubectl -n llmd apply -f ./resources/llmd/aigateawayroutes

  
echo -e "[INFO] ...done."

## Open WebUI
echo -e "\n[INFO] Installing Open WebUI..."
kubectl create namespace openwebui --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace openwebui service-type=lab
kubectl label namespace openwebui trust-manager/inject-lab-ca-secret=enabled

kubectl -n openwebui apply -f ./resources/openwebui/secrets

helm upgrade open-webui open-webui/open-webui \
   --install \
  --namespace openwebui \
  -f ./resources/openwebui/helm/openwebui.yaml \
  --wait

kubectl -n openwebui apply -f ./resources/openwebui/httproutes

echo -e "[INFO] ...done."

## HELIX
echo -e "\n[INFO] Installing HELIX..."
kubectl create namespace helix --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace helix trust-manager/inject-lab-ca-secret=enabled 
kubectl label namespace helix service-type=lab

kubectl -n helix apply -R -f ./resources/helix/secrets

helm upgrade helix aphp-helix/helix \
    --install \
    --namespace helix \
    -f ./resources/helix/helm/helix.yaml
echo -e "[INFO] ...done."

echo -e "\n[INFO] Tier 3 layer sucessfully deployed.\n"