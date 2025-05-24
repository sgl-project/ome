# Build the model-agent binary
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

# Build the model-agent binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o model-agent ./cmd/model-agent

# Use Oracle Linux 9 as base image for OCI SDK compatibility
FROM oraclelinux:9-slim
RUN microdnf update -y && microdnf clean all

COPY --from=builder /workspace/model-agent /
ENTRYPOINT ["/model-agent"]
