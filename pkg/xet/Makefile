.PHONY: build test clean release-darwin-% release-linux-% release

RUST_TARGET ?=

# Default target
all: build

# Build the Rust library
build:
	@echo "Building Rust library..."
	cargo build --release $(if $(RUST_TARGET),--target $(RUST_TARGET))
	@echo "Copying static library..."
	@if [ -z "$(RUST_TARGET)" ]; then \
		cp target/release/libxet.a . 2>/dev/null || cp target/release/libxet.dylib . 2>/dev/null || true; \
	else \
		cp target/$(RUST_TARGET)/release/libxet.a . 2>/dev/null || true; \
	fi

# Run tests
test: build
	@echo "Running Rust tests..."
	cargo test

# Build for specific Darwin architecture
release-darwin-%: test
	@echo "Building for darwin-$*..."
	$(MAKE) build RUST_TARGET=$*-apple-darwin
	mkdir -p artifacts/darwin-$*
	cp libxet.a artifacts/darwin-$*/ 2>/dev/null || cp libxet.dylib artifacts/darwin-$*/ 2>/dev/null || true
	cd artifacts/darwin-$* && tar -czf libxet.darwin-$*.tar.gz libxet.*

# Build for Linux using Docker
release-linux-%:
	@echo "Building for linux-$*..."
	docker build --platform linux/$* \
		--build-arg DOCKER_TARGETPLATFORM=linux/$* \
		-f build/Dockerfile . \
		-t xet-build:linux-$*
	docker run --rm -v $(PWD)/artifacts:/artifacts xet-build:linux-$*

# Build all release targets
release: release-darwin-aarch64 release-darwin-x86_64 release-linux-amd64 release-linux-arm64
	@echo "Release artifacts created in ./artifacts/"

# Clean build artifacts
clean:
	cargo clean
	rm -f libxet.a libxet.so libxet.dylib
	rm -rf artifacts/

# Install dependencies
deps:
	@echo "Installing Rust dependencies..."
	cargo fetch

# Format code
fmt:
	cargo fmt

# Run linters
lint:
	cargo clippy

# Generate C header from Rust (requires cbindgen)
header:
	cbindgen --config cbindgen.toml --crate ome-xet-binding --output xet.h

# Help target
help:
	@echo "Available targets:"
	@echo "  make build    - Build the Rust library"
	@echo "  make test     - Run Rust tests"
	@echo "  make clean    - Clean build artifacts"
	@echo "  make release  - Build release artifacts for all platforms"
	@echo "  make deps     - Install dependencies"
	@echo "  make fmt      - Format code"
	@echo "  make lint     - Run linters"
	@echo "  make header   - Generate C header file"