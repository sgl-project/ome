# Build the manager binary
FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.10-ol9-119 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/    cmd/
COPY pkg/    pkg/

# Build
RUN GOFIPS140=latest go build -o manager ./cmd/manager

# Copy the controller-manager into a thin image
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.52 / /
RUN microdnf update -y && \
    microdnf install -y crypto-policies-scripts && \
    update-crypto-policies --set FIPS:NO-ENFORCE-EMS && \
    rm -f /usr/local/bin/supercronic && \
    microdnf clean all

COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/manager /
ENTRYPOINT ["/manager"]
