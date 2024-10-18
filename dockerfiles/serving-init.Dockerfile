FROM odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:1.23.0-1 AS builder
ENV GOPROXY="https://artifactory-builds.oci.oraclecorp.com/api/go/go-proxy"

# Copy in the go src
WORKDIR /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome
COPY go.mod  go.mod
COPY go.sum  go.sum

RUN go mod download

COPY cmd/serving-init    cmd/serving-init
COPY cmd/serving-ft    cmd/serving-ft
COPY internal/serving-init    internal/serving-init
COPY internal/serving-ft    internal/serving-ft
COPY pkg/    pkg/
COPY appconfigs/serving-init     configs/
COPY appconfigs/serving-ft     configs/

RUN CGO_ENABLED=1 GOOS=linux go build -a -o serving-init ./cmd/serving-init
RUN CGO_ENABLED=1 GOOS=linux go build -a -o serving-ft ./cmd/serving-ft


FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:8-slim
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol8:1.34 / /

WORKDIR /
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/serving-init .
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/serving-ft .
COPY --from=builder /go/src/bitbucket.oci.oraclecorp.com/genaicore/ome/configs/ /configs/
COPY scripts/serving-init-launcher.sh .

USER 12:20
ENTRYPOINT ["/serving-init-launcher.sh"]
