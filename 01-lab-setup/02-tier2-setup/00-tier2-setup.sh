#!/bin/bash

##############################################################################################################
# Name: 00-tier2-setup.sh                                                                                    #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to the transversal stack that will address cross-cutting concerns               #
############################################################################################################## 

set -eup

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
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm repo add falcosecurity https://falcosecurity.github.io/charts --force-update
helm repo add vm https://victoriametrics.github.io/helm-charts --force-update
helm repo add oauth2-proxy https://oauth2-proxy.github.io/manifests --force-update
helm repo update
echo -e "[INFO] ...done."

# Installing transversal stack

## Kyverno
echo -e "\n[INFO] Installing Kyverno..."
kubectl create namespace kyverno --dry-run=client -o yaml | kubectl apply -f -
helm upgrade kyverno kyverno/kyverno \
    --install \
    --namespace kyverno \
    --wait
echo -e "[INFO] ...done."


## Falco
echo -e "\n[INFO] Installing Falco..."
kubectl create namespace falco --dry-run=client -o yaml | kubectl apply -f -
helm upgrade falco falcosecurity/falco \
    --install \
    --namespace falco \
    -f ./resources/falco/helm/falco.yaml \
    --wait
echo -e "[INFO] ...done."


## Keycloak
### Keycloak Operator
echo -e "\n[INFO] Installing Keycloak Operator..."
kubectl create namespace keycloak --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace keycloak service-type=auth
kubectl label namespace keycloak trust-manager/inject-secret=enabled

kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/keycloaks.k8s.keycloak.org-v1.yml
kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/keycloakrealmimports.k8s.keycloak.org-v1.yml
kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/kubernetes.yml

kubectl -n keycloak wait deploy/keycloak-operator --for=condition=Available --timeout=300s
echo -e "[INFO] ...done."

### Keycloak Instance
echo -e "\n[INFO] Deploying a Keycloak Instance.."
kubectl -n keycloak apply -R -f ./resources/keycloak/postresql
kubectl -n keycloak wait -l statefulset.kubernetes.io/pod-name=postgresql-db-0 --for=condition=ready pod --timeout=300s

kubectl -n keycloak apply -R -f ./resources/keycloak/secrets
kubectl -n keycloak apply -R -f ./resources/keycloak/certificates

kubectl -n keycloak apply -R -f ./resources/keycloak/keycloaks
kubectl -n keycloak wait --for=condition=Ready keycloaks.k8s.keycloak.org/keycloak --timeout=300s

kubectl -n keycloak apply -R -f ./resources/keycloak/keycloakrealmimports
kubectl -n keycloak wait --for=condition=Done keycloakrealmimports/k8s-lab-import --timeout=300s

kubectl -n keycloak apply -R -f ./resources/keycloak/tlsroutes
echo -e "[INFO] ...done"


## OAuth2-Proxy
echo -e "\n[INFO] Installing OAuth2-Proxy..."
kubectl create namespace oauth2-proxy --dry-run=client -o yaml | kubectl apply -f -

kubectl label namespace oauth2-proxy service-type=auth
kubectl label namespace oauth2-proxy trust-manager/inject-secret=enabled

kubectl -n oauth2-proxy apply -R -f ./resources/oauth2-proxy/certificates
kubectl -n oauth2-proxy apply -R -f ./resources/oauth2-proxy/secrets

helm upgrade oauth2-proxy oauth2-proxy/oauth2-proxy \
  --install \
  --namespace oauth2-proxy \
  -f ./resources/oauth2-proxy/helm/oauth2-proxy.yaml \
  --wait
kubectl -n oauth2-proxy apply -R -f ./resources/oauth2-proxy/tlsroutes
echo -e "[INFO] ...done."


## Victoria Stack
### Victoria Logs
echo -e "\n[INFO] Installing Victoria Logs Server..."
kubectl create namespace victorialogs --dry-run=client -o yaml | kubectl apply -f -

helm upgrade vls vm/victoria-logs-single \
    --install \
    --namespace victorialogs \
    -f ./resources/victorialogs/helm/vlogs.yaml \
    --wait
echo -e "[INFO] ...done."

### Victoria Metrics K8S Stack
echo -e "\n[INFO] Installing Victoria Metrics K8S Stack..."
kubectl create namespace victoriametrics --dry-run=client -o yaml | kubectl apply -f -

kubectl label namespace victoriametrics service-type=lab
kubectl label namespace victoriametrics trust-manager/inject-secret=enabled

kubectl -n victoriametrics apply -f ./resources/victoriametrics/secrets

helm upgrade vmks vm/victoria-metrics-k8s-stack \
    --install \
    --namespace victoriametrics \
    -f ./resources/victoriametrics/helm/vmks.yaml \
    --wait

kubectl -n victoriametrics apply -f ./resources/victoriametrics/httproutes
echo -e "[INFO] ...done."

echo -e "\n[INFO] Tier 2 layer sucessfully deployed.\n"