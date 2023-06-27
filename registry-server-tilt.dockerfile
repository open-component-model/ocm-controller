FROM alpine
WORKDIR /
COPY ./bin/registry-server /registry-server
COPY ./pkg/oci/registry/certs ./pkg/oci/registry/certs

ENTRYPOINT ["/registry-server"]
