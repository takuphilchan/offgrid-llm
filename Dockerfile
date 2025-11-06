# Multi-stage build for OffGrid LLM
# Supports both mock and llama.cpp modes

# Stage 1: Build llama.cpp (optional, for real inference)
FROM golang:1.21-alpine AS llama-builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    build-base \
    cmake

# Clone and build llama.cpp
WORKDIR /build
RUN git clone https://github.com/ggerganov/llama.cpp.git && \
    cd llama.cpp && \
    make

# Stage 2: Build OffGrid LLM
FROM golang:1.21-alpine AS go-builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG BUILD_MODE=mock
ARG VERSION=0.1.0

# Build application
RUN if [ "$BUILD_MODE" = "llama" ]; then \
        COPY --from=llama-builder /build/llama.cpp /opt/llama.cpp && \
        export CGO_ENABLED=1 && \
        export C_INCLUDE_PATH=/opt/llama.cpp && \
        export LIBRARY_PATH=/opt/llama.cpp && \
        go build -tags llama -ldflags "-X main.Version=$VERSION" -o offgrid ./cmd/offgrid; \
    else \
        go build -ldflags "-X main.Version=$VERSION" -o offgrid ./cmd/offgrid; \
    fi

# Stage 3: Runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    libstdc++ \
    libgomp

# Copy binary from builder
COPY --from=go-builder /app/offgrid /usr/local/bin/offgrid

# Copy llama.cpp libraries if built with llama support
ARG BUILD_MODE=mock
RUN if [ "$BUILD_MODE" = "llama" ]; then \
        COPY --from=llama-builder /build/llama.cpp/*.so* /usr/local/lib/ || true && \
        ldconfig /usr/local/lib || true; \
    fi

# Create directories
RUN mkdir -p /root/.offgrid/models /data

# Set working directory
WORKDIR /root/.offgrid

# Expose ports
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run application
ENTRYPOINT ["/usr/local/bin/offgrid"]
CMD ["serve"]
