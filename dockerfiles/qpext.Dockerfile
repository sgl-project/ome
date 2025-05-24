# Build the omae qpext binary
FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/    cmd/
COPY pkg/    pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on GOFIPS140=latest go build -a -o qpext ./cmd/qpext

FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.42 / /
RUN microdnf install io-ol8-container-hardening && rm -rf /var/cache/yum
RUN microdnf update -y && microdnf clean all

# Copy the built binary from the builder image
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/qpext /
# Create a new user 'appuser' and change ownership of relevant files
RUN adduser --system --no-create-home --uid 10001 appuser && \
    chown appuser /qpext

USER 10001

ENTRYPOINT ["/qpext"]