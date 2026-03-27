FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Use commit hash artifact for current test purpose, will update to use released version later
ARG COMMIT_HASH=1502d4e9e9fd956f22ecdccf7750ea8684a4149b

ARG OME_VERSION=0.1.3
ARG BUILD_CGO_ENABLED=1
# Setup the environment variables used in the Go Language build
ENV GOPATH=/gopath GOROOT=/usr/local/go

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

RUN if [ -n "$COMMIT_HASH" ]; then \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/sgl-project/ome/${COMMIT_HASH}" | tar -xzvf - && mv ome-* /ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/sgl-project/ome/v${OME_VERSION}" | tar -xzvf - && mv ome-${OME_VERSION} /ome; \
    fi

WORKDIR /ome

# Downgrade go.mod directive
ENV GOTOOLCHAIN=local
RUN go mod edit -go=1.24
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

# Build the ome-agent binary with CGO enabled for xet integration
RUN GOFIPS140=latest CGO_ENABLED=${BUILD_CGO_ENABLED} GOOS=linux go build -o ome-agent ./cmd/ome-agent

# Build runnable image
# See https://confluence.oraclecorp.com/confluence/display/OCIODO/Docker+Support+Image+-+Self+Service for latest version
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim-fips
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.46 / /
RUN microdnf install io-ol9-container-hardening && rm -rf /var/cache/yum

RUN microdnf update -y && \
    microdnf clean all && \
    rm -rf /var/cache/yum/*

COPY --from=builder /ome/ome-agent /
COPY --from=builder /ome/config/ome-agent/ome-agent.yaml /

ENTRYPOINT ["/ome-agent"]