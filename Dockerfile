FROM --platform=$BUILDPLATFORM golang:1.20.6-alpine AS builder

RUN apk add --update --no-cache gcc g++

WORKDIR /build
COPY api ./api

WORKDIR /build/grype-server
COPY grype-server/go.* ./
RUN go mod download

# Copy and build backend code
COPY grype-server .

ARG TARGETOS
ARG TARGETARCH

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o grype-server ./cmd/grype-server/main.go

FROM alpine:3.17

WORKDIR /app

COPY --from=builder ["/build/grype-server/grype-server", "./grype-server"]

ENTRYPOINT ["/app/grype-server"]

USER 1000

# Build-time metadata as defined at http://label-schema.org
ARG BUILD_DATE
ARG VCS_REF
LABEL org.label-schema.build-date=$BUILD_DATE \
    org.label-schema.name="grype-server" \
    org.label-schema.description="Running Grype scanner as a K8s server" \
    org.label-schema.url="https://github.com/openclarity/grype-server" \
    org.label-schema.vcs-ref=$VCS_REF \
    org.label-schema.vcs-url="https://github.com/openclarity/grype-server"

### Required OpenShift Labels
ARG IMAGE_VERSION
LABEL name="grype-server" \
      vendor="openclarity" \
      version=${IMAGE_VERSION} \
      release=${IMAGE_VERSION} \
      summary="Grype scanner as a K8s server" \
      description="Running Grype scanner as a K8s server"
