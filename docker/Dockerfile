# OffGrid LLM - Production Docker Image
# Multi-stage build for optimized image size
# Build from project root: docker build -f docker/Dockerfile -t offgrid-llm .

# =============================================================================
# Stage 1: Build the web application
# =============================================================================
FROM node:22-alpine AS ui-builder

WORKDIR /build/web/app
COPY web/app/package.json web/app/package-lock.json ./
RUN set -eu; \
    for attempt in 1 2 3; do \
        npm ci && exit 0; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        echo "npm dependency download failed (attempt $attempt/3); retrying..." >&2; \
        sleep $((attempt * 5)); \
    done
COPY web/app/ ./
RUN npm run build

# =============================================================================
# Stage 2: Build the Go binary
# =============================================================================
FROM golang:1.26.6-alpine3.24 AS go-builder

# OffGrid uses the HTTP inference engine; native CUDA/C++ lives in the separate
# llama-builder stage. Match release binaries with a portable, pure-Go build.
WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN set -eu; \
    for attempt in 1 2 3; do \
        go mod download && exit 0; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        echo "Go module download failed (attempt $attempt/3); retrying..." >&2; \
        sleep $((attempt * 5)); \
    done

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

ARG VERSION=dev
ARG VCS_REF=unknown

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build \
	-trimpath -ldflags="-s -w -X main.Version=${VERSION} -X github.com/takuphilchan/offgrid-llm/internal/server.BuildRevision=${VCS_REF} -extldflags=-static" \
	-o offgrid ./cmd/offgrid/

# =============================================================================
# Optional target for local application/UI refreshes without rebuilding llama.cpp.
# docker build --target application-artifacts -f docker/Dockerfile ...
# This is an artifact carrier, not a runnable service image.
# =============================================================================
FROM alpine:3.24 AS application-artifacts
COPY --from=go-builder /build/offgrid /usr/local/bin/offgrid
COPY --from=ui-builder /build/web/dist/ /var/lib/offgrid/web/ui/

# =============================================================================
# Stage 3: Build llama.cpp server
# =============================================================================
FROM alpine:3.24 AS llama-builder

ARG LLAMA_CPP_COMMIT=e9fa0781f1c25fc4fe8c86be1edc6970661ad6f0

RUN apk add --no-cache \
    git \
    cmake \
    make \
    g++ \
    linux-headers

WORKDIR /build

# Fetch an immutable llama.cpp revision and build it
RUN mkdir llama.cpp && cd llama.cpp && \
    git init && \
    git remote add origin https://github.com/ggml-org/llama.cpp.git && \
    git fetch --depth 1 origin ${LLAMA_CPP_COMMIT} && \
    git checkout --detach FETCH_HEAD && \
    cmake -B build \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_SHARED_LIBS=OFF \
        -DGGML_OPENMP=OFF \
        -DGGML_NATIVE=OFF \
        -DLLAMA_BUILD_SERVER=ON \
        -DLLAMA_BUILD_TESTS=OFF \
        -DLLAMA_BUILD_EXAMPLES=OFF && \
    cmake --build build --config Release -j$(nproc) --target llama-server && \
    strip build/bin/llama-server

# =============================================================================
# Stage 4: Final runtime image
# =============================================================================
FROM alpine:3.24

ARG VERSION=dev
ARG VCS_REF=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="OffGrid LLM" \
      org.opencontainers.image.description="Private, local-first LLM inference, agents, and knowledge retrieval" \
      org.opencontainers.image.url="https://github.com/takuphilchan/offgrid-llm" \
      org.opencontainers.image.source="https://github.com/takuphilchan/offgrid-llm" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}"

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    wget \
    libstdc++ \
    libgcc

# Create non-root user
RUN addgroup -g 1000 offgrid && \
    adduser -D -u 1000 -G offgrid offgrid

# Create required directories
RUN mkdir -p /var/lib/offgrid/models \
             /var/lib/offgrid/web/ui \
             /var/lib/offgrid/data \
             /var/lib/offgrid/bin && \
    chown -R offgrid:offgrid /var/lib/offgrid

# Copy binaries from builders
COPY --from=go-builder /build/offgrid /usr/local/bin/offgrid
COPY --from=llama-builder /build/llama.cpp/build/bin/llama-server /var/lib/offgrid/bin/llama-server

RUN chmod +x /usr/local/bin/offgrid /var/lib/offgrid/bin/llama-server

# Copy the exact UI built from this source revision.
COPY --from=ui-builder --chown=offgrid:offgrid /build/web/dist /var/lib/offgrid/web/ui

# Switch to non-root user
USER offgrid
WORKDIR /var/lib/offgrid

# Expose default port
EXPOSE 11611

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://127.0.0.1:11611/health || exit 1

# Environment variables
ENV OFFGRID_MODELS_DIR=/var/lib/offgrid/models \
    OFFGRID_DATA_DIR=/var/lib/offgrid/data \
    OFFGRID_BIN_DIR=/var/lib/offgrid/bin \
    OFFGRID_VERSION=${VERSION} \
    OFFGRID_PORT=11611 \
    OFFGRID_HOST=0.0.0.0 \
    OFFGRID_ALLOW_UNAUTHENTICATED_REMOTE=true

# Keep the image useful as both a server and a CLI. For example,
# `docker run --rm IMAGE version` executes `offgrid version`.
ENTRYPOINT ["/usr/local/bin/offgrid"]
CMD ["serve"]
