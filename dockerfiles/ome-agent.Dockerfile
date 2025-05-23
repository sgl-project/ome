# Build the ome-agent binary
FROM golang:1.24 AS builder

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

# Build the ome-agent binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o ome-agent ./cmd/ome-agent

# Use Oracle Linux 9 as base image for OCI SDK compatibility
FROM oraclelinux:9-slim
RUN microdnf update -y && microdnf clean all

COPY --from=builder /workspace/ome-agent /
COPY config/ome-agent/ome-agent.yaml /
ENTRYPOINT ["/ome-agent"]
