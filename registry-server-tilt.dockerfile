FROM alpine
WORKDIR /
COPY ./bin/registry-server /registry-server
COPY ./pkg/oci/registry/certs /certs

ENTRYPOINT ["/registry-server"]
