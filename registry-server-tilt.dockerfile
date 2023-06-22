FROM alpine
WORKDIR /
COPY ./bin/registry-server /registry-server
COPY ./pkg/oci/registry/certs/tls.key /certs/tls.key
COPY ./pkg/oci/registry/certs/tls.crt /certs/tls.crt

ENTRYPOINT ["/registry-server"]
