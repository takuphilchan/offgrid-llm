# OffGrid LLM - Production Docker Image
# Multi-stage build for optimized image size
# Build from project root: docker build -f docker/Dockerfile -t offgrid-llm .

# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
FROM golang:1.26.6-alpine3.24 AS go-builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    g++ \
    musl-dev \
    linux-headers

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY web/ ./web/

# Build the application with static linking
RUN CGO_ENABLED=1 GOOS=linux go build \
    -a -ldflags '-extldflags "-static"' \
    -o offgrid ./cmd/offgrid/

# =============================================================================
# Stage 2: Build llama.cpp server
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
    cmake -B build -DGGML_STATIC=ON -DLLAMA_BUILD_SERVER=ON && \
    cmake --build build --config Release -j$(nproc) --target llama-server && \
    strip build/bin/llama-server

# =============================================================================
# Stage 3: Final runtime image
# =============================================================================
FROM alpine:3.24

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

# Copy web UI
COPY --from=go-builder /build/web/ui /var/lib/offgrid/web/ui

# Switch to non-root user
USER offgrid
WORKDIR /var/lib/offgrid

# Expose default port
EXPOSE 11611

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:11611/health || exit 1

# Environment variables
ENV OFFGRID_MODELS_DIR=/var/lib/offgrid/models \
    OFFGRID_DATA_DIR=/var/lib/offgrid/data \
    OFFGRID_BIN_DIR=/var/lib/offgrid/bin \
    OFFGRID_PORT=11611 \
    OFFGRID_HOST=localhost

# Run the server
CMD ["/usr/local/bin/offgrid"]
