#!/usr/bin/env bash

path=$(pwd)

if [[ "${path}" == *hack* ]]; then
  echo "This script is intended to be executed from the project root."

  exit 1
fi

if [ "$(kubectl get secret -n ocm-system developer-root-certificate)" ] && [ "$(kubectl get secret -n ocm-system registry-certs)" ]; then
  echo "secrets already exist, no need to re-run"

  exit 0
fi

echo "generating developer certificates and kubernetes secrets"

# Set up certificate paths
serverPemPath="./hack/certs/server.pem"
serverKeyPath="./hack/certs/server-key.pem"
rootCAPath=$(./bin/mkcert -CAROOT)/rootCA.pem

if [ ! -e "${serverPemPath}" ] && [ ! -e "${serverKeyPath}" ]; then
  echo -n "certificates not found, generating..."

  ./bin/mkcert -cert-file ./hack/certs/server.pem -key-file ./hack/certs/server-key.pem registry.ocm-system.svc.cluster.local localhost 127.0.0.1 ::1

  echo "done"
else
  echo "certificates found, will not re-generate"
fi

echo -n "creating base64 content from certificates..."

# Read the certificate content
serverPemContent=$(base64 < "${serverPemPath}")
serverKeyContent=$(base64 < "${serverKeyPath}")
rootCAContent=$(base64 < "${rootCAPath}")

echo "done."

echo -n "writing kubernetes secrets..."

# Generate and apply certificate secrets
cat > ./hack/certs/rootCASecret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: developer-root-certificate
  namespace: ocm-system
type: Opaque
data:
  ca-certificates.crt: |
    ${rootCAContent}
EOF

cat > ./hack/certs/registryCertificateSecret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: registry-certs
  namespace: ocm-system
type: Opaque
data:
  server.pem: |
    ${serverPemContent}
  server-key.pem: |
    ${serverKeyContent}
EOF

echo "done"

cat ./hack/certs/registryCertificateSecret.yaml
cat ./hack/certs/rootCASecret.yaml
