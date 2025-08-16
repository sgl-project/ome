# Build the multinode-prober binary
FROM golang:1.25 AS builder

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

# Build the multinode-prober binary
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a -installsuffix cgo \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o multinode-prober ./cmd/multinode-prober

# Use distroless as minimal base image to package the multinode-prober binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/multinode-prober .
USER 65532:65532

ENTRYPOINT ["/multinode-prober"]
