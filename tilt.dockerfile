FROM alpine
WORKDIR /
COPY ./bin/manager /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
