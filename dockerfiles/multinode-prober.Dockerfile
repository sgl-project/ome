# Build the multinode-prober binary
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

# Build the multinode-prober binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o multinode-prober ./cmd/multinode-prober

# Use distroless as minimal base image to package the multinode-prober binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/multinode-prober .
USER 65532:65532

ENTRYPOINT ["/multinode-prober"]
