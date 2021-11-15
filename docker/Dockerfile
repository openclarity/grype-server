FROM golang:1.16-alpine AS builder
WORKDIR /go/src/github.com/Portshift/grype-server/
COPY ./ ./
RUN CGO_ENABLED=0 go build -o grype-server ./grype-server/cmd/grype-server/

FROM registry.access.redhat.com/ubi8
RUN yum install ca-certificates -y
RUN mkdir /licenses
COPY ./LICENSE /licenses/
RUN mkdir /app
COPY --from=builder /go/src/github.com/Portshift/grype-server/grype-server /app/grype-server
RUN chmod +x /app/grype-server
ENTRYPOINT ["/app/grype-server"]

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
LABEL org.label-schema.build-date=$BUILD_DATE \
    org.label-schema.name="grype-server" \
    org.label-schema.description="Running Grype scanner as a K8s server" \
    org.label-schema.url="https://github.com/Portshift/grype-server" \
    org.label-schema.vcs-ref=$VCS_REF \
    org.label-schema.vcs-url="https://github.com/Portshift/grype-server"

### Required OpenShift Labels
ARG IMAGE_VERSION
LABEL name="grype-server" \
      vendor="Portshift" \
      version=${IMAGE_VERSION} \
      release=${IMAGE_VERSION} \
      summary="Grype scanner as a K8s server" \
      description="Running Grype scanner as a K8s server"

USER 1000