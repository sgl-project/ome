# Configurable base image - must be declared before any FROM statement
# Defaults to Oracle Linux 10 for OCI SDK compatibility
# Can be overridden with --build-arg BASE_IMAGE=ubuntu:22.04
ARG BASE_IMAGE=oraclelinux:10-slim

# Build the model-agent binary
FROM golang:1.24 AS builder

# Build arguments for cross-compilation
ARG TARGETOS
ARG TARGETARCH

# Set working directory
WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies with Go module cache
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

# Build the model-agent binary with Go build cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o model-agent ./cmd/model-agent

# Use the base image specified at the top of the file
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Install/update packages based on the base image
RUN if [ -f /usr/bin/microdnf ]; then \
        microdnf update -y && microdnf clean all; \
    elif [ -f /usr/bin/apt-get ]; then \
        apt-get update && \
        apt-get install -y ca-certificates && \
        apt-get upgrade -y && \
        apt-get clean && rm -rf /var/lib/apt/lists/*; \
    fi

COPY --from=builder /workspace/model-agent /
ENTRYPOINT ["/model-agent"]
