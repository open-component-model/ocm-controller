#!/usr/bin/env bash

# cleanup
if [[ -f hack/rootCA.pem ]]; then
  rm hack/rootCA.pem
fi

CERT_MANAGER_VERSION=${CERT_MANAGER_VERSION:-v1.13.1}

if [ ! -e 'hack/cert-manager.yaml' ]; then
  echo "fetching cert-manager manifest for version ${CERT_MANAGER_VERSION}"
  curl -L https://github.com/cert-manager/cert-manager/releases/download/"${CERT_MANAGER_VERSION}"/cert-manager.yaml -o hack/cert-manager.yaml
fi

kind create cluster --name=e2e-test-cluster

echo 'installing cert-manager'
kubectl apply -f hack/cert-manager.yaml
kubectl wait --for=condition=Available=True Deployment/cert-manager -n cert-manager --timeout=60s
kubectl wait --for=condition=Available=True Deployment/cert-manager-webhook -n cert-manager --timeout=60s
kubectl wait --for=condition=Available=True Deployment/cert-manager-cainjector -n cert-manager --timeout=60s
echo 'done'

echo 'applying root certificate issuer'
kubectl apply -f hack/cluster_issuer.yaml
echo 'done'

echo 'waiting for root certificate to be generated...'
kubectl wait --for=condition=Ready=true Certificate/mpas-bootstrap-certificate -n cert-manager --timeout=60s
echo 'done'

kubectl get secret ocm-registry-tls-certs -n cert-manager -o jsonpath="{.data['tls\.crt']}" | base64 -d > hack/rootCA.pem
echo 'installing root certificate into local trust store...'
CAROOT=hack ./bin/mkcert -install
rootCAPath="./hack/rootCA.pem"

if [ -e '/etc/ssl/certs/ca-certificates.crt' ]; then
  echo "updating root certificate"
  sudo cat "${rootCAPath}" | sudo tee -a /etc/ssl/certs/ca-certificates.crt || echo "failed to append to ca-certificates. Ignoring the failure"
fi

echo 'done'
