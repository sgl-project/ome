# Build the ome-agent binary
FROM golang:1.25 AS builder

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

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY internal/ internal/

# Build the XET library first
RUN cd pkg/xet && make build

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

# Build the ome-agent binary (CGO must be enabled for XET library)
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o ome-agent ./cmd/ome-agent

# Use Oracle Linux 10 as base image for OCI SDK compatibility
FROM oraclelinux:10-slim
RUN microdnf update -y && microdnf clean all

# Install runtime dependencies for the XET library
RUN microdnf install -y \
    glibc \
    libgcc \
    libstdc++ \
    openssl-libs \
    && microdnf clean all

COPY --from=builder /workspace/ome-agent /
COPY config/ome-agent/ome-agent.yaml /
ENTRYPOINT ["/ome-agent"]
