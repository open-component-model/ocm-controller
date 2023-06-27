#!/usr/bin/env bash

./bin/mkcert --install
./bin/mkcert -cert-file ./pkg/oci/registry/certs/server.pem  -key-file ./pkg/oci/registry/certs/server-key.pem registry.ocm-system.svc.cluster.local localhost 127.0.0.1 ::1

# Copy the RootCRTs into `~/.mkcert`
# Setup mkcert to trust that location

# Overwrite the root certificate in secret.yaml
