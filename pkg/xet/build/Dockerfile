# Build container for xet binding
FROM rust:1.89 as builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /build

# Copy the entire xet package
COPY . /build/

# Build the Rust library
RUN cargo build --release

# Copy the built library to artifacts
RUN mkdir -p /artifacts && \
    cp target/release/libxet.a /artifacts/ && \
    tar -czf /artifacts/libxet.linux-$(uname -m).tar.gz -C /artifacts libxet.a

# Final stage
FROM alpine:latest
COPY --from=builder /artifacts /artifacts
CMD ["ls", "-la", "/artifacts"]