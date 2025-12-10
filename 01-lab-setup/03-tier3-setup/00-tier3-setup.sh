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
helm repo add vllm-production-stack https://vllm-project.github.io/production-stack --force-update
helm repo add open-webui https://helm.openwebui.com/ --force-update
helm repo update
echo -e "\n[INFO] ...done"

# Installing applicative stack

## vLLM Stack
echo -e "\n[INFO] Installing vLLM Stack..."
kubectl create namespace vllm --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace openwebui service-type=llm

### vLLM
kubectl -n vllm apply -f ./resources/vllm/secrets

helm upgrade vllm vllm-production-stack/vllm-stack \
    --install \
    --namespace vllm \
    -f ./resources/vllm/helm/vllm.yaml \
    --wait

### Inference Pool
helm upgrade vllm-model-pool oci://registry.k8s.io/gateway-api-inference-extension/charts/inferencepool \
  --install \
  --version v1.1.0 \
  --namespace vllm \
  -f ./resources/vllm/inferencepools/helm/vllm.yaml
  
echo -e "[INFO] ...done."

## Open WebUI
echo -e "\n[INFO] Installing Open WebUI..."
kubectl create namespace openwebui --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace openwebui service-type=lab
kubectl label namespace openwebui trust-manager/inject-lab-ca-secret=enabled

kubectl -n openwebui apply -f ./resources/openwebui/secrets
kubectl -n openwebui apply -f ./resources/openwebui/backends
kubectl -n openwebui apply -f ./resources/openwebui/aiservicebackends
kubectl -n openwebui apply -f ./resources/openwebui/aigatewayroutes

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