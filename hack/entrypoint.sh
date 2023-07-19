#!/usr/bin/env sh

rootCA=${REGISTRY_ROOT_CERTIFICATE:-/certs/ca.pem}

if [ ! -e "${rootCA}" ]; then
  echo "warning... root certificate at location ${rootCA} not found."

  exec "$@"
fi

echo "updating root certificate with provided certificate..."
tee -a /etc/ssl/certs/ca-certificates.crt < "${rootCA}"

echo "done."

exec "$@"
