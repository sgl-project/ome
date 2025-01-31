FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:go1.23.5-35 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/    cmd/
COPY pkg/    pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -a -o model-agent ./cmd/model-agent

# Copy the controller-manager into a thin image
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol8:1.40 / /
RUN microdnf update -y && microdnf clean all

COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/model-agent /
ENTRYPOINT ["/model-agent"]
