# This Dockerfile is used by `make` via the Makefile
ARG ALPINE_VERSION
FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION}

# Create group and user
ARG UID=1000
ARG GID=1000
RUN addgroup -g "${GID}" ocmGroup && adduser -u "${UID}" ocmUser -G ocmGroup -D

ARG SUFFIX
ARG OCM_VERSION

COPY gen/component/ocm-controller.$SUFFIX /bin/manager

LABEL org.opencontainers.image.description="Open Component Model Kubernetes controller based on Alpine ${ALPINE_VERSION}"
LABEL org.opencontainers.image.vendor="SAP SE"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.url="https://ocm.software/"
LABEL org.opencontainers.image.source="https://github.com/open-component-model/ocm-controller"
LABEL org.opencontainers.image.title="ocm-controller"
LABEL org.opencontainers.image.version="${OCM_VERSION}"
LABEL org.opencontainers.image.revision="${OCM_VERSION}"
LABEL org.opencontainers.image.documentation="https://github.com/open-component-model/ocm-controller/blob/main/README.md"
LABEL org.opencontainers.image.base.name="alpine:${ALPINE_VERSION}"

USER ocmUser
ENTRYPOINT ["/bin/manager"]
