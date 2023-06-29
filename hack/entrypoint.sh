#!/usr/bin/env sh

rootCA=/certs/ca.pem

if [ ! -e "${rootCA}" ]; then
  echo "root certificate at location ${rootCA} not found"

  exit 1
fi

echo "Updating root certificate with provided certificate..."
cat "${rootCA}" | tee -a /etc/ssl/certs/ca-certificates.crt

echo "done."

exec "$@"
