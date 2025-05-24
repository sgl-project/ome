# Build the model-controller binary
FROM golang:1.24 AS builder

# Install git and ca-certificates for go mod download
RUN apk add --no-cache git ca-certificates

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

# Build the model-controller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o model-controller ./cmd/model-controller

# Use distroless as minimal base image to package the model-controller binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/model-controller .
USER 65532:65532

ENTRYPOINT ["/model-controller"]
