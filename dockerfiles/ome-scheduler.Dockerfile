# ome-scheduler: the upstream kube-scheduler built as a library with the OME
# placement plugin (OMEGangPack) registered. Lives in its own Go module
# (scheduler/go.mod, pinned to the target Kubernetes minor), and links no
# CGO, so it cross-compiles to a static binary on scratch. Build context is
# the repo root (matches the other dockerfiles/).
ARG GO_BUILDER_IMAGE=golang:1.26
FROM ${GO_BUILDER_IMAGE} AS builder
WORKDIR /workspace/scheduler

# Module files first so the dependency layer caches independently of source.
COPY scheduler/go.mod scheduler/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Source.
COPY scheduler/ ./

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /ome-scheduler ./cmd/ome-scheduler

FROM scratch
COPY --from=builder /ome-scheduler /ome-scheduler
USER 65532:65532
ENTRYPOINT ["/ome-scheduler"]
