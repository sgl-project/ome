# Build & Release Guide

## Local builds
- `make build` – compiles `libxet` (`cargo build --release`) and the Go wrapper (`go build`). Copy operations search for `libxet.a` or `libxet.dylib` in `target/release`.
- `make test` – runs `make build`, `cargo test`, then `go test ./... -count=1` from `pkg/xet`.
- `make header` – regenerates `xet.h` via `cbindgen --config cbindgen.toml --crate ome-xet-binding`.
- `make clean` – removes Cargo targets, copied libraries, Go cache, and `artifacts/`.

## Cross-platform artifacts
- **Darwin:** `make release-darwin-{aarch64,x86_64}` sets `RUST_TARGET`, copies the appropriate static/dynamic library, and packages it under `artifacts/darwin-*/libxet.darwin-*.tar.gz`.
- **Linux:** `make release-linux-{amd64,arm64}` builds via `docker build -f build/Dockerfile` and extracts artifacts using `docker run -v $(PWD)/artifacts:/artifacts`.
- `make release` invokes all four platform targets.

### Docker build (`build/Dockerfile`)
1. Uses `rust:1.75` base image; installs build-essential, cmake, pkg-config, libssl-dev.
2. Copies the entire crate, runs `cargo build --release`.
3. Stages `libxet.a` into `/artifacts` and tars it with architecture suffix.
4. Final `alpine` stage exposes `/artifacts`.

## Library configuration
- `Cargo.toml` exports both `staticlib` and `cdylib`. Release profile enables LTO and stripping to shrink artifacts.
- Dependencies on `xet-core` components are sourced from GitHub; ensure vendoring or network access in CI.

## Versioning & ABI stability
- `xet_version_1_0_0` (link symbol) must change when breaking the C ABI; bump to `xet_version_1_1_0`, update Go `#cgo` block, and add release notes.
- When adding/removing exported structs/fields, regenerate `xet.h`, review diffs, and update compatibility shims in Go.
- Maintain semantic versioning in `Cargo.toml`. Tag releases alongside published binaries.

## Continuous integration checklist
- Run `make build`, `make test`, and `make header` in CI for macOS and Linux.
- Cache Cargo/git dependencies where possible; the project pulls multiple `xet-core` git crates.
- Archive `artifacts/` outputs as CI build products for downstream consumers.