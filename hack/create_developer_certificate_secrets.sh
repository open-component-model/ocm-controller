#!/usr/bin/env bash

path=$(pwd)

if [[ "${path}" == *hack* ]]; then
  echo "This script is intended to be executed from the project root."

  exit 1
fi

if [ "$(kubectl get secret -n ocm-system ocm-registry-tls-certs)" ]; then
  echo "secret already exist, no need to re-run"

  exit 0
fi

echo "generating developer certificates and kubernetes secrets"

# Set up certificate paths
CAROOT=./hack/certs ./bin/mkcert -install
certPath="./hack/certs/cert.pem"
keyPath="./hack/certs/key.pem"
rootCAPath="./hack/certs/rootCA.pem"

if [ ! -e "${certPath}" ] && [ ! -e "${keyPath}" ]; then
  echo -n "certificates not found, generating..."

  CAROOT=./hack/certs ./bin/mkcert -cert-file ./hack/certs/cert.pem -key-file ./hack/certs/key.pem registry.ocm-system.svc.cluster.local localhost 127.0.0.1 ::1

  if [ -e '/etc/ssl/certs/ca-certificates.crt' ]; then
    echo "updating root certificate"
    sudo cat "${rootCAPath}" | sudo tee -a /etc/ssl/certs/ca-certificates.crt || echo "failed to append to ca-certificates. Ignoring the failure"
  fi

  echo "done"
else
  echo "certificates found, will not re-generate"
fi

echo -n "creating secret..."
kubectl create secret generic \
  -n ocm-system ocm-registry-tls-certs \
  --from-file=caFile="${rootCAPath}" \
  --from-file=certFile="${certPath}" \
  --from-file=keyFile="${keyPath}" \
  --dry-run=client -o yaml > ./hack/certs/registry_certs_secret.yaml
