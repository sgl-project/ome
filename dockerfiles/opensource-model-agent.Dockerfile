FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"
ENV GOPATH=/gopath GOROOT=/usr/local/go

# Use commit hash artifact for testing; can switch to released version by clearing COMMIT_HASH and setting OME_VERSION
ARG COMMIT_HASH=57808a0f57c63d9acb6d7ebc3238fa25536a858c
ARG OME_VERSION=0.1.3
ARG BUILD_CGO_ENABLED=0

# Fetch open-source OME code from Artifactory-proxied GitHub
RUN if [ -n "$COMMIT_HASH" ]; then \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/sgl-project/ome/${COMMIT_HASH}" \
        | tar -xzvf - && mv ome-* /ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/sgl-project/ome/v${OME_VERSION}" \
        | tar -xzvf - && mv ome-${OME_VERSION} /ome; \
    fi

WORKDIR /ome

# Build model-agent binary
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