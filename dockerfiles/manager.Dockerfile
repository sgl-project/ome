# Build the manager binary
FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Use commit hash artifact for current test purpose, will update to use released version later
# ARG COMMIT_HASH=04cdb94a8afba266caa43bff1c9ed09e44d7ba80

ARG OME_VERSION=0.1.4
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
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/sgl-project/ome/${COMMIT_HASH}" | tar -xzvf - && mv ome-* /opensource-ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/sgl-project/ome/v${OME_VERSION}" | tar -xzvf - && mv ome-${OME_VERSION} /opensource-ome; \
    fi

# Set OpenSSL env for Rust build
ENV OPENSSL_DIR=/usr \
    OPENSSL_LIB_DIR=/usr/lib64/openssl3 \
    OPENSSL_INCLUDE_DIR=/usr/include/openssl3

# Build xet library from downloaded opensource ome
WORKDIR /opensource-ome/pkg/xet
RUN cargo build --release && \
    cp /opensource-ome/pkg/xet/target/release/libxet.a /usr/local/lib/

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

ENV CGO_LDFLAGS="-L/usr/local/lib -L/usr/lib64/openssl3 -lxet -lssl -lcrypto -ldl -lpthread"

COPY cmd/    cmd/
COPY pkg/    pkg/

# Copy xet source from opensource ome (must be after COPY pkg/ to not get overwritten)
RUN cp -r /opensource-ome/pkg/xet pkg/

# Build
RUN GOFIPS140=latest CGO_ENABLED=1 go build -o manager ./cmd/manager

# Copy the controller-manager into a thin image
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.42 / /
RUN microdnf update -y && microdnf clean all

COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/manager /
ENTRYPOINT ["/manager"]
