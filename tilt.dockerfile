FROM alpine
WORKDIR /
COPY ./bin/manager /manager
COPY ./hack/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["/manager"]
