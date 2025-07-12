# Build the ome-agent binary
FROM golang:1.24 AS builder

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

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

# Build the ome-agent binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o ome-agent ./cmd/ome-agent

# Use Oracle Linux 9 as base image for OCI SDK compatibility
FROM oraclelinux:10-slim
RUN microdnf update -y && microdnf clean all

COPY --from=builder /workspace/ome-agent /
COPY config/ome-agent/ome-agent.yaml /
ENTRYPOINT ["/ome-agent"]
