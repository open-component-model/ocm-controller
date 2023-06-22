FROM alpine
WORKDIR /
COPY ./bin/manager /manager
COPY ./pkg/oci/registry/certs /certs

ENTRYPOINT ["/manager"]
