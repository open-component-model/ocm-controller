FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY ocm-controller /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
