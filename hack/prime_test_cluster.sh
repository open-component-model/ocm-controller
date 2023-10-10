#!/usr/bin/env bash

# cleanup
rm -fr hack/rootCA.pem

kind create cluster --name=e2e-test-cluster

echo -n 'installing cert-manager'
kubectl apply -f hack/cert-manager.yaml
kubectl wait --for=condition=Available=True Deployment/cert-manager -n cert-manager --timeout=60s
kubectl wait --for=condition=Available=True Deployment/cert-manager-webhook -n cert-manager --timeout=60s
kubectl wait --for=condition=Available=True Deployment/cert-manager-cainjector -n cert-manager --timeout=60s
echo 'done'

echo -n 'applying root certificate issuer'
kubectl apply -f hack/cluster_issuer.yaml
echo 'done'

echo -n 'waiting for root certificate to be generated...'
kubectl wait --for=condition=Ready=true Certificate/mpas-bootstrap-certificate -n cert-manager --timeout=60s
echo 'done'

kubectl get secret ocm-registry-tls-certs -n cert-manager -o jsonpath="{.data['tls\.crt']}" | base64 -D > hack/rootCA.pem
echo -n 'installing root certificate into local trust store...'
CAROOT=hack mkcert -install

echo 'done'
