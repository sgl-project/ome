FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Use commit hash artifact for current test purpose, will update to use released version later
ARG COMMIT_HASH=6446c318bf3310696c4b4254a6e867deccb34751

ARG OME_VERSION=0.1.5
ARG BUILD_CGO_ENABLED=1
# Setup the environment variables used in the Go Language build
ENV GOPATH=/gopath GOROOT=/usr/local/go

# Install build dependencies for Rust xet library
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
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/ome-projects/ome/${COMMIT_HASH}" | tar -xzvf - && mv ome-* /ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/ome-projects/ome/v${OME_VERSION}" | tar -xzvf - && mv ome-${OME_VERSION} /ome; \
    fi

WORKDIR /ome

RUN go mod edit -go=1.25
RUN go get \
    github.com/kedacore/keda/v2@v2.17.3 \
    github.com/expr-lang/expr@v1.17.7 \
    golang.org/x/crypto@v0.52.0 \
    golang.org/x/net@v0.55.0 \
    golang.org/x/sys@v0.45.0
RUN go mod tidy

# Set env so Rust picks up OpenSSL 3
ENV OPENSSL_DIR=/usr \
    OPENSSL_LIB_DIR=/usr/lib64/openssl3 \
    OPENSSL_INCLUDE_DIR=/usr/include/openssl3

# Build the Rust xet library first (required for Go CGO linking)
RUN cd pkg/xet && cargo build --release

# Copy the static lib for CGO
RUN cp pkg/xet/target/release/libxet.a /usr/local/lib/
ENV CGO_LDFLAGS="-L/usr/local/lib -L/usr/lib64/openssl3 -lxet -lssl -lcrypto -ldl -lpthread"

# Build the manager binary with CGO enabled for xet integration
RUN GOFIPS140=latest CGO_ENABLED=${BUILD_CGO_ENABLED} GOOS=linux go build -o manager ./cmd/manager

# Build runnable image
# See https://confluence.oraclecorp.com/confluence/display/OCIODO/Docker+Support+Image+-+Self+Service for latest version
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim-fips
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.46 / /
RUN microdnf update -y && \
    microdnf install -y io-ol9-container-hardening crypto-policies-scripts && \
    update-crypto-policies --set FIPS:NO-ENFORCE-EMS && \
    rm -f /usr/local/bin/supercronic && \
    microdnf clean all && \
    rm -rf /var/cache/yum/*

COPY --from=builder /ome/manager /

ENTRYPOINT ["/manager"]
