# Build the manager binary
FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:go1.23.3-30 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/download-agent    cmd/download-agent
COPY internal/download-agent    internal/download-agent
COPY pkg/    pkg/
COPY appconfigs/download-agent/     configs/

RUN CGO_ENABLED=1 GOOS=linux go build -a -o download-agent ./cmd/download-agent

FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol8:1.40 / /
RUN microdnf update -y && microdnf clean all

WORKDIR /
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/download-agent .
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/configs/ /configs/

USER 12:20
ENTRYPOINT ["/download-agent", "download-agent", "-c", "/configs/partner-download-agent.yaml"]