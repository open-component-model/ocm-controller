#!/usr/bin/env bash

path=$(pwd)

if [[ "${path}" == *hack* ]]; then
  echo "This script is intended to be executed from the project root."

  exit 1
fi

if [ "$(kubectl get secret -n ocm-system registry-certs)" ]; then
  echo "secret already exist, no need to re-run"

  exit 0
fi

echo "generating developer certificates and kubernetes secrets"

# Set up certificate paths
sudo ./bin/mkcert -install
certPath="./hack/certs/cert.pem"
keyPath="./hack/certs/key.pem"
rootCAPath=$(sudo ./bin/mkcert -CAROOT)/rootCA.pem

echo "updating root certificate"

sudo cat "${rootCAPath}" | sudo tee -a /etc/ssl/certs/ca-certificates.crt || echo "appending to ca-certificates failed but ignoring"
#sudo cp "${rootCAPath}" /etc/ssl/certs/ || echo "failed to copy to /usr/local/share/ca-certificates/ but ignoring"
#sudo cp "${rootCAPath}" /usr/local/share/ca-certificates/ || echo "failed to copy to /usr/local/share/ca-certificates/ but ignoring"
#sudo cp "${rootCAPath}" /usr/share/ca-certificates || echo "failed to copy to /usr/share/ca-certificates but ignoring"
#sudo cp "${rootCAPath}" /etc/ca-certificates/ || echo "failed to copy to /etc/ca-certificates/ but ignoring"

#sudo update-ca-certificates || echo "ignore update ca fail"  # Option 1.
#trust extract-compat || echo "ignore extract fail"        # Option 2.

if [ ! -e "${certPath}" ] && [ ! -e "${keyPath}" ]; then
  echo -n "certificates not found, generating..."

  sudo ./bin/mkcert -cert-file ./hack/certs/cert.pem -key-file ./hack/certs/key.pem registry.ocm-system.svc.cluster.local localhost 127.0.0.1 ::1

  echo "done"
else
  echo "certificates found, will not re-generate"
fi

#sudo chmod 777 "./hack/certs/cert.pem"
#sudo chmod 777 "./hack/certs/key.pem"
#sudo chmod 777 "${rootCAPath}"

echo -n "creating secret..."
sudo kubectl create secret generic \
  -n ocm-system registry-certs \
  --from-file=ca.pem="${rootCAPath}" \
  --from-file=cert.pem="${certPath}" \
  --from-file=key.pem="${keyPath}" \
  --dry-run=client -o yaml > ./hack/certs/registry_certs_secret.yaml

cat ./hack/certs/registry_certs_secret.yaml

# Read the certificate content
#certContent=$(base64 < "${certPath}")
#keyContent=$(base64 < "${keyPath}")
#rootCAContent=$(base64 < "${rootCAPath}")
#
#echo "done."
#
#echo -n "writing kubernetes secrets..."
#
#echo -n "apiVersion: v1
#kind: Secret
#metadata:
#  name: registry-certs
#  namespace: ocm-system
#type: Opaque
#data:
#  cert.pem: ${certContent}
#  key.pem: ${keyContent}
#  ca.pem: ${rootCAContent}" > ./hack/certs/registryCertificateSecret.yaml
#
#echo "done"
#
#cat ./hack/certs/registryCertificateSecret.yaml
