FROM alpine
WORKDIR /bin
COPY ./bin/manager /bin/manager

ENTRYPOINT ["/bin/manager"]
