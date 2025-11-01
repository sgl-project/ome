# Configurable base image - must be declared before any FROM statement
# Defaults to Oracle Linux 10 for OCI SDK compatibility
# Can be overridden with --build-arg BASE_IMAGE=ubuntu:22.04
ARG BASE_IMAGE=oraclelinux:10-slim

# Build the ome-agent binary
FROM golang:1.24 AS builder

# Install Rust and Cargo for building the XET library
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

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

# Copy XET dependencies from other pkg subdirectories
COPY pkg/configutils/ pkg/configutils/
COPY pkg/logging/ pkg/logging/

# Copy XET package for building with better caching
COPY pkg/xet/ pkg/xet/

# Download Rust dependencies with cargo cache
RUN --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=cache,target=/root/.cargo/git \
    cd pkg/xet && cargo fetch

# Build the XET library with cargo build cache
RUN --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=cache,target=/root/.cargo/git \
    cd pkg/xet && \
    cargo build --release

# Copy remaining source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY internal/ internal/

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

# Build the ome-agent binary (CGO must be enabled for XET library)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o ome-agent ./cmd/ome-agent

# Use the base image specified at the top of the file
ARG BASE_IMAGE
FROM ${BASE_IMAGE}

# Install/update packages and runtime dependencies based on the base image
RUN if [ -f /usr/bin/microdnf ]; then \
        microdnf update -y && \
        microdnf install -y glibc libgcc libstdc++ openssl-libs && \
        microdnf clean all; \
    elif [ -f /usr/bin/apt-get ]; then \
        apt-get update && \
        apt-get upgrade -y && \
        apt-get install -y libc6 libgcc-s1 libstdc++6 libssl3 && \
        apt-get clean && rm -rf /var/lib/apt/lists/*; \
    fi

COPY --from=builder /workspace/ome-agent /
COPY config/ome-agent/ome-agent.yaml /
ENTRYPOINT ["/ome-agent"]
