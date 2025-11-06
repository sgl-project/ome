# SGLang Router with Oracle Linux 9
FROM ocr-docker-remote.artifactory.oci.oraclecorp.com/os/oraclelinux:9-slim AS base

# Copy base image support layer
COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/base-image-support/ol9:1.42 / /

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

# Install system dependencies
RUN microdnf install -y \
    python3.12 \
    python3.12-pip \
    python3.12-devel \
    gcc \
    gcc-c++ \
    make \
    git \
    ca-certificates \
    && microdnf clean all \
    && rm -rf /var/cache/yum \
    && ln -s /usr/bin/python3.12 /usr/bin/python3 \
    && ln -s /usr/bin/python3.12 /usr/bin/python

# Install uv and sglang-router using pip
RUN python3.12 -m pip install --no-cache-dir uv && \
    python3.12 -m pip install --no-cache-dir sglang-router

# Set working directory
WORKDIR /app

# Expose default router port (adjust as needed)
EXPOSE 30000

# Run the router
ENTRYPOINT ["python3", "-m", "sglang_router.launch_router"]
