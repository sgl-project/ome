# Build the benchmark-controller binary
FROM golang:1.25 AS builder

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

# Build the benchmark-controller binary (CGO disabled for static binary)
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -a \
    -ldflags "-X github.com/sgl-project/ome/pkg/version.GitVersion=${GIT_TAG} -X github.com/sgl-project/ome/pkg/version.GitCommit=${GIT_COMMIT}" \
    -o benchmark-controller ./cmd/benchmark

# Use scratch for minimal image
FROM scratch
WORKDIR /
COPY --from=builder /workspace/benchmark-controller .
USER 65532:65532

ENTRYPOINT ["/benchmark-controller"]
