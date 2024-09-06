FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:1.23.0-1 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/    cmd/
COPY pkg/    pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE=on go build -o model-controller ./cmd/model-controller

FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol8:1.34 / /

COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/model-controller /
ENTRYPOINT ["/model-controller"]
