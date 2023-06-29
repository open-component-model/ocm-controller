#!/usr/bin/env sh

rootCA=/certs/ca.pem

if [ ! -e "${rootCA}" ]; then
  echo "root certificate at location ${rootCA} not found, ignoring appending it..."

  exec "$@"
fi

echo "Updating root certificate with provided certificate..."
tee -a /etc/ssl/certs/ca-certificates.crt < "${rootCA}"

echo "done."

exec "$@"
