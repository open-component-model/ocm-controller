#!/usr/bin/env bash

path=$(pwd)

if [ "$(kubectl get secret -n ocm-system developer-root-certificate)" ] && [ "$(kubectl get secret -n ocm-system registry-certs)" ]; then
  echo "secrets already exist, no need to re-run"

  exit 0
fi

if [[ "${path}" == *hack* ]]; then
  echo "This script is intended to be executed from the project root."

  exit 1
fi

# Set up certificate paths
serverPemPath="./hack/certs/server.pem"
serverKeyPath="./hack/certs/server-key.pem"
rootCAPath=$(./bin/mkcert -CAROOT)/rootCA.pem

if [ ! -e "${serverPemPath}" ] && [ ! -e "${serverKeyPath}" ]; then
  echo "Please generate certificates first with make generate-developer-certs."

  exit 1
fi

# Read the certificate content
serverPemContent=$(base64 < "${serverPemPath}")
serverKeyContent=$(base64 < "${serverKeyPath}")
rootCAContent=$(base64 < "${rootCAPath}")

# Generate and apply certificate secrets
cat > ./hack/certs/rootCASecret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: developer-root-certificate
  namespace: ocm-system
type: Opaque
data:
  ca-certificates.crt: ${rootCAContent}
EOF

cat > ./hack/certs/registryCertificateSecret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: registry-certs
  namespace: ocm-system
type: Opaque
data:
  server.pem: ${serverPemContent}
  server-key.pem: ${serverKeyContent}
EOF
