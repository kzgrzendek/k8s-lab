#!/bin/bash

##############################################################################################################
# Name: 03-transversal-stack install.sh                                                                      #
# Version: 0.1                                                                                               #
# Author: @kzgrzendek                                                                                        #
# Description: Helper script to the transversal stack that will address cross-cutting concerns               #
############################################################################################################## 

set -eup

echo -e "[INFO] Stating K8S transversal stack install script v1.0"

echo -e "\n[INFO] Checking if kubectl is installed..."
if command -v kubectl &>/dev/null; then
    echo -e "[INFO] ...kubectl is installed."
else
    echo -e "[ERROR] ...kubectl is not installed! Please follow these instructions and launch the script again : https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
    exit 1
fi

echo -e "\n[INFO] Checking if helm is installed..."
if command -v helm &>/dev/null; then
    echo -e "[INFO] ...helm is installed."
else
    echo -e "[ERROR] ...helm is not installed! Please follow these instructions and launch the script again : https://helm.sh/docs/intro/install/"
    exit 1
fi

echo -e "\n[INFO] Adding Helm repositories..."
helm repo add falcosecurity https://falcosecurity.github.io/charts --force-update
helm repo add kyverno https://kyverno.github.io/kyverno/ --force-update
helm repo add vm https://victoriametrics.github.io/helm-charts --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update
echo -e "\n[INFO] ...done"

# Installing transversal stack

## Kyverno
echo -e "\n[INFO] Installing Kyverno..."
kubectl create namespace kyverno --dry-run=client -o yaml | kubectl apply -f -
helm upgrade kyverno kyverno/kyverno \
    --install \
    --namespace kyverno \
    --wait
echo -e "\n[INFO] ...done"


## Falco
echo -e "\n[INFO] Installing Falco..."
kubectl create namespace falco --dry-run=client -o yaml | kubectl apply -f -
helm upgrade falco falcosecurity/falco \
    --install \
    --namespace falco \
    --set tty=true \
    --wait
echo -e "\n[INFO] ...done"

## Cert Manager
echo -e "\n[INFO] Installing Cert Manager..."
kubectl create namespace cert-manager --dry-run=client -o yaml | kubectl apply -f -
helm upgrade cert-manager jetstack/cert-manager \
  --install \
  --version v1.19.1 \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --wait

kubectl -n cert-manager apply -R -f ./resources/cert-manager/certificates
kubectl -n cert-manager apply -R -f ./resources/cert-manager/clusterissuers
echo -e "\n[INFO] ...done"

## Trust Manager
echo -e "\n[INFO] Installing Trust Manager..."
helm upgrade trust-manager jetstack/trust-manager \
  --install \
  --namespace cert-manager \
  --wait
echo -e "\n[INFO] ...done"

## Keycloak
### Keycloak Operator
echo -e "\n[INFO] Installing Keycloak Operator..."
kubectl create namespace keycloak --dry-run=client -o yaml | kubectl apply -f -

kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/keycloaks.k8s.keycloak.org-v1.yml
kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/keycloakrealmimports.k8s.keycloak.org-v1.yml
kubectl -n keycloak apply -f https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/26.4.1/kubernetes/kubernetes.yml

kubectl -n keycloak wait deploy/keycloak-operator --for=condition=Available --timeout=300s

echo -e "\n[INFO] ...done"

### Keycloak Instance
echo -e "\n[INFO] Deploying a Keycloak Instance.."
kubectl -n keycloak apply -R -f ./resources/keycloak/postresql
kubectl -n keycloak wait -l statefulset.kubernetes.io/pod-name=postgresql-db-0 --for=condition=ready pod --timeout=300s

kubectl -n keycloak apply -R -f ./resources/keycloak/secrets

kubectl -n keycloak apply -R -f ./resources/keycloak/keycloaks
kubectl -n keycloak wait --for=condition=Ready keycloaks.k8s.keycloak.org/keycloak

kubectl -n keycloak apply -R -f ./resources/keycloak/keycloakrealmimports
kubectl -n keycloak wait --for=condition=Done keycloakrealmimports/k8s-lab --timeout=300s  


## Victoria Stack
### Victoria Logs
echo -e "\n[INFO] Installing Victoria Logs Server..."
kubectl create namespace victorialogs --dry-run=client -o yaml | kubectl apply -f -
helm upgrade vls vm/victoria-logs-single \
    --install \
    --namespace victorialogs \
    -f ./resources/victorialogs/helm/vlogs.yaml \
    --wait
echo -e "\n[INFO] ...done"

### Victoria Metrics K8S Stack
echo -e "\n[INFO] Installing Victoria Metrics K8S Stack..."
kubectl create namespace victoriametrics --dry-run=client -o yaml | kubectl apply -f -
kubectl -n victoriametrics apply -f ./resources/victoriametrics/secrets/

helm upgrade vmks vm/victoria-metrics-k8s-stack \
    --install \
    --namespace victoriametrics \
    -f ./resources/victoriametrics/helm/vmks.yaml \
    --wait
echo -e "\n[INFO] ...done"

echo -e "\n[INFO] Script terminated successfully!"