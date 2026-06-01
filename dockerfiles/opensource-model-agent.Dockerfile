FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"
ENV GOPATH=/gopath GOROOT=/usr/local/go

# Use commit hash artifact for testing; can switch to released version by clearing COMMIT_HASH and setting OME_VERSION
ARG COMMIT_HASH=1f49ae89e5d625a6f3d2ed06a214aac008841e7c
ARG OME_VERSION=0.1.3
ARG BUILD_CGO_ENABLED=1

# Install Rust build dependencies
RUN microdnf install -y \
    gcc \
    gcc-c++ \
    make \
    cmake \
    pkgconfig \
    openssl3-devel \
    curl \
    && microdnf clean all

# Install Rust toolchain
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# Fetch open-source OME code from Artifactory-proxied GitHub
RUN if [ -n "$COMMIT_HASH" ]; then \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/sgl-project/ome/${COMMIT_HASH}" \
        | tar -xzvf - && mv ome-* /ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/sgl-project/ome/v${OME_VERSION}" \
        | tar -xzvf - && mv ome-${OME_VERSION} /ome; \
    fi

WORKDIR /ome

# Keep the signed Go 1.24 FIPS builder and only bump immediately-gating modules.
ENV GOTOOLCHAIN=local
RUN go mod edit -go=1.24
RUN go get \
    google.golang.org/grpc@v1.79.3 \
    go.opentelemetry.io/otel@v1.40.0 \
    go.opentelemetry.io/otel/metric@v1.40.0 \
    go.opentelemetry.io/otel/sdk@v1.40.0 \
    go.opentelemetry.io/otel/trace@v1.40.0
RUN go mod tidy

# Set env so Rust picks up OpenSSL 3
ENV OPENSSL_DIR=/usr \
    OPENSSL_LIB_DIR=/usr/lib64/openssl3 \
    OPENSSL_INCLUDE_DIR=/usr/include/openssl3

# Build the Rust xet library first (required for Go CGO linking)
RUN cd pkg/xet && \
    cargo build --release

# Copy the static lib for CGO
RUN cp pkg/xet/target/release/libxet.a /usr/local/lib/
ENV CGO_LDFLAGS="-L/usr/local/lib -L/usr/lib64/openssl3 -lxet -lssl -lcrypto -ldl -lpthread"

# Build model-agent binary with CGO enabled for xet integration
RUN GOFIPS140=latest CGO_ENABLED=${BUILD_CGO_ENABLED} GOOS=linux \
    go build -a -o model-agent ./cmd/model-agent

# ---------- Runtime stage ----------
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim-fips

# Include Oracle base support layer & apply hardening
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.46 / /
RUN microdnf install -y io-ol9-container-hardening && rm -rf /var/cache/yum

# Update system packages and cleanup
RUN microdnf update -y && \
    microdnf clean all && \
    rm -rf /var/cache/yum/*

# Copy the built binary
COPY --from=builder /ome/model-agent /

# Default entrypoint
ENTRYPOINT ["/model-agent"]
