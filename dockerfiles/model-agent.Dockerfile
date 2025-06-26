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

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build arguments for version info
ARG VERSION
ARG GIT_TAG
ARG GIT_COMMIT

# Build the model-agent binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o model-agent ./cmd/model-agent

# Use Oracle Linux 9 as base image for OCI SDK compatibility
FROM oraclelinux:9-slim
RUN microdnf update -y && microdnf clean all

COPY --from=builder /workspace/model-agent /
ENTRYPOINT ["/model-agent"]
