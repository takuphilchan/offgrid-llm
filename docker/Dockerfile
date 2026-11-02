# OffGrid LLM - Production Docker Image
# Multi-stage build for optimized image size
# Build from project root: docker build -f docker/Dockerfile -t offgrid-llm .

# The pinned Emscripten compiler is Linux x64; its WASM output is portable.
ARG OCR_BUILD_PLATFORM=linux/amd64

# =============================================================================
# Stage 1: Build the web application
# =============================================================================
FROM node:22-alpine AS ui-builder

WORKDIR /build/apps/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN set -eu; \
    for attempt in 1 2 3; do \
        npm ci && exit 0; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        echo "npm dependency download failed (attempt $attempt/3); retrying..." >&2; \
        sleep $((attempt * 5)); \
    done
COPY apps/web/ ./
COPY apps/desktop/compatibility.cjs /build/apps/desktop/compatibility.cjs
RUN npm run build

# Pinned parser assets are acquired only while building, never on import.
FROM node:22-alpine AS document-assets
WORKDIR /build
COPY runtimes/pdfium/engine.lock.json /build/runtimes/pdfium/engine.lock.json
COPY scripts/build/prepare-document-runtime.mjs /build/scripts/build/prepare-document-runtime.mjs
RUN node scripts/build/prepare-document-runtime.mjs

# OCR source compilation is a maintainer build step, never a user prerequisite.
FROM --platform=${OCR_BUILD_PLATFORM} python:3.13-bookworm AS ocr-assets
RUN apt-get update && apt-get install -y --no-install-recommends cmake ninja-build nodejs && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY runtimes/ocr/engine.lock.json runtimes/ocr/source.lock.json runtimes/ocr/bridge.cpp ./runtimes/ocr/
COPY scripts/build/prepare-ocr-runtime.py scripts/build/build-ocr-runtime.py ./scripts/build/
RUN python scripts/build/prepare-ocr-runtime.py --build-from-source /opt/ocr-build

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
COPY tools/executor-pack/ ./tools/executor-pack/
COPY runtimes/pdfium/ ./runtimes/pdfium/
COPY --from=document-assets /build/runtimes/pdfium/assets/ ./runtimes/pdfium/assets/
RUN CGO_ENABLED=0 GOOS=linux go -C runtimes/pdfium build -mod=readonly -trimpath -ldflags="-s -w" -o /build/offgrid-document-worker .
COPY runtimes/ocr/worker/ ./runtimes/ocr/worker/
COPY --from=ocr-assets /build/runtimes/ocr/worker/assets/ ./runtimes/ocr/worker/assets/
RUN CGO_ENABLED=0 GOOS=linux go -C runtimes/ocr/worker build -mod=readonly -trimpath -ldflags="-s -w" -o /build/offgrid-ocr-worker .

ARG VERSION=dev
ARG VCS_REF=unknown

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build \
	-trimpath -ldflags="-s -w -X main.Version=${VERSION} -X github.com/takuphilchan/offgrid-llm/internal/server.BuildRevision=${VCS_REF} -extldflags=-static" \
	-o offgrid ./cmd/offgrid/
RUN go run ./tools/executor-pack --output /build/execution-runtime --version "${VERSION}"

# Dependency-free parser qualification target; no shell, network client or fonts.
FROM scratch AS document-worker-check
COPY --from=go-builder /build/offgrid-document-worker /offgrid-document-worker
COPY --from=document-assets /build/runtimes/pdfium/assets/NOTICE.txt /document-NOTICE.txt
ENTRYPOINT ["/offgrid-document-worker"]

FROM scratch AS ocr-worker-check
COPY --from=go-builder /build/offgrid-ocr-worker /offgrid-ocr-worker
COPY --from=ocr-assets /build/runtimes/ocr/worker/assets/NOTICE.txt /ocr-NOTICE.txt
ENTRYPOINT ["/offgrid-ocr-worker"]

# =============================================================================
# Optional target for local application/UI refreshes without rebuilding llama.cpp.
# docker build --target application-artifacts -f docker/Dockerfile ...
# This is an artifact carrier, not a runnable service image.
# =============================================================================
FROM alpine:3.24 AS application-artifacts
COPY --from=go-builder /build/offgrid /usr/local/bin/offgrid
COPY --from=go-builder /build/offgrid-ocr-worker /usr/local/bin/offgrid-ocr-worker
COPY --from=ocr-assets /build/runtimes/ocr/worker/assets/NOTICE.txt /usr/local/share/offgrid/ocr-NOTICE.txt
COPY --from=go-builder /build/offgrid-document-worker /usr/local/bin/offgrid-document-worker
COPY --from=document-assets /build/runtimes/pdfium/assets/NOTICE.txt /usr/local/share/offgrid/document-NOTICE.txt
COPY --from=go-builder /build/execution-runtime /usr/local/bin/execution-runtime
COPY --from=ui-builder /build/apps/web/dist/ /var/lib/offgrid/web/ui/

# =============================================================================
# Stage 3: Build llama.cpp server
# =============================================================================
FROM alpine:3.24 AS llama-builder

ARG LLAMA_CPP_COMMIT=b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9

RUN apk add --no-cache \
    git \
    cmake \
    make \
    g++ \
    linux-headers

WORKDIR /build

# Fetch an immutable llama.cpp revision and build it
COPY runtimes/llama-cpp/ /build/offgrid-llama-patches/
RUN mkdir llama.cpp && cd llama.cpp && \
    git init && \
    git remote add origin https://github.com/ggml-org/llama.cpp.git && \
    git fetch --depth 1 origin ${LLAMA_CPP_COMMIT} && \
    git checkout --detach FETCH_HEAD && \
    sh /build/offgrid-llama-patches/apply-patches.sh /build/llama.cpp && \
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
COPY --from=go-builder /build/execution-runtime /usr/local/bin/execution-runtime
COPY --from=llama-builder /build/llama.cpp/build/bin/llama-server /var/lib/offgrid/bin/llama-server
COPY --from=go-builder /build/offgrid-ocr-worker /usr/local/bin/offgrid-ocr-worker
COPY --from=ocr-assets /build/runtimes/ocr/worker/assets/NOTICE.txt /usr/local/share/offgrid/ocr-NOTICE.txt
COPY --from=go-builder /build/offgrid-document-worker /usr/local/bin/offgrid-document-worker
COPY --from=document-assets /build/runtimes/pdfium/assets/NOTICE.txt /usr/local/share/offgrid/document-NOTICE.txt

RUN chmod +x /usr/local/bin/offgrid /var/lib/offgrid/bin/llama-server

# Copy the exact UI built from this source revision.
COPY --from=ui-builder --chown=offgrid:offgrid /build/apps/web/dist /var/lib/offgrid/web/ui

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
