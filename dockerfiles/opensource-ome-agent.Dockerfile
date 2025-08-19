FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Use commit hash artifact for current test purpose, will update to use released version later
ARG COMMIT_HASH=fc646e3df16504168af202c3c6f7c0249dd84ac9

ARG OME_VERSION=0.1.3
ARG BUILD_CGO_ENABLED=0
# Setup the environment variables used in the Go Language build
ENV GOPATH=/gopath GOROOT=/usr/local/go

RUN if [ -n "$COMMIT_HASH" ]; then \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadCommit/github-vcs-remote/sgl-project/ome/${COMMIT_HASH}" | tar -xzvf - && mv ome-* /ome; \
    else \
        curl -sSL "https://artifactory.oci.oraclecorp.com/api/vcs/downloadTag/github-vcs-remote/sgl-project/ome/v${OME_VERSION}" | tar -xzvf - && mv ome-${OME_VERSION} /ome; \
    fi

WORKDIR /ome

# Build the ome-agent binary
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
