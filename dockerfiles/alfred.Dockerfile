# Configurable base image - must be declared before any FROM statement
# Defaults to Oracle Linux 10 for parity with the other OME images
# Can be overridden with --build-arg BASE_IMAGE=ubuntu:24.04
ARG BASE_IMAGE=oraclelinux:10-slim

# Build the alfred binary (pure Go — no Rust/XET, CGO disabled)
FROM golang:1.26 AS builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags "-X sigs.k8s.io/ome/pkg/version.GitVersion=${GIT_TAG} -X sigs.k8s.io/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o alfred ./cmd/alfred

# Use the base image specified at the top of the file
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

RUN if [ -f /usr/bin/microdnf ]; then \
        microdnf update -y && \
        microdnf install -y ca-certificates && \
        microdnf clean all; \
    elif [ -f /usr/bin/apt-get ]; then \
        apt-get update && apt-get install -y ca-certificates && \
        apt-get upgrade -y && \
        apt-get clean && rm -rf /var/lib/apt/lists/*; \
    fi
WORKDIR /
COPY --from=builder /workspace/alfred .
USER 65532:65532

ENTRYPOINT ["/alfred"]
