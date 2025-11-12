FROM golang:1.25.4
WORKDIR /
COPY ./bin/manager /manager

RUN go install github.com/go-delve/delve/cmd/dlv@latest
RUN chmod +x /go/bin/dlv
RUN mv /go/bin/dlv /

EXPOSE 30000

# dlv --listen=:30000 --api-version=2 --headless=true exec /app/build/api
ENTRYPOINT ["/dlv", "--listen=:30000", "--api-version=2", "--headless=true", "--continue=true", "--accept-multiclient=true", "exec", "/manager", "--"]
