# Multi-stage build for OffGrid LLM
# Two-process architecture: OffGrid (Go) + llama-server (C++)

# Stage 1: Build llama.cpp with llama-server
FROM ubuntu:22.04 AS llama-builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    wget \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Build llama.cpp
WORKDIR /build
RUN git clone https://github.com/ggerganov/llama.cpp.git && \
    cd llama.cpp && \
    mkdir -p build && \
    cd build && \
    cmake .. -DBUILD_SHARED_LIBS=ON && \
    cmake --build . --config Release --target llama-server -j$(nproc)

# Stage 2: Build OffGrid LLM (Go)
FROM golang:1.21-bookworm AS go-builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build OffGrid LLM
ARG VERSION=0.1.0
RUN go build -ldflags "-X main.Version=$VERSION" -o offgrid ./cmd/offgrid

# Stage 3: Runtime image
FROM ubuntu:22.04

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    libgomp1 \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/*

# Copy llama-server binary and shared libraries from builder
COPY --from=llama-builder /build/llama.cpp/build/bin/llama-server /usr/local/bin/llama-server
COPY --from=llama-builder /build/llama.cpp/build/bin/*.so* /usr/local/lib/

# Copy OffGrid binary from builder
COPY --from=go-builder /app/offgrid /usr/local/bin/offgrid

# Copy web UI
COPY --from=go-builder /app/web /var/lib/offgrid/web

# Update library cache
RUN ldconfig

# Create offgrid user and directories
RUN useradd -r -s /bin/false offgrid && \
    mkdir -p /var/lib/offgrid/models /etc/offgrid && \
    chown -R offgrid:offgrid /var/lib/offgrid

# Create startup script
RUN echo '#!/bin/bash\n\
set -e\n\
\n\
# Start llama-server in background\n\
LLAMA_PORT=$((RANDOM % 16383 + 49152))\n\
echo "Starting llama-server on port $LLAMA_PORT..."\n\
llama-server -m /var/lib/offgrid/models/*.gguf \\\n\
  --host 127.0.0.1 \\\n\
  --port $LLAMA_PORT \\\n\
  --ctx-size 4096 \\\n\
  --n-gpu-layers 0 \\\n\
  --log-disable &\n\
\n\
LLAMA_PID=$!\n\
echo $LLAMA_PORT > /tmp/llama-port\n\
echo "llama-server started (PID: $LLAMA_PID, Port: $LLAMA_PORT)"\n\
\n\
# Wait for llama-server to be ready\n\
echo "Waiting for llama-server..."\n\
for i in {1..30}; do\n\
  if curl -s http://127.0.0.1:$LLAMA_PORT/health > /dev/null 2>&1; then\n\
    echo "llama-server is ready"\n\
    break\n\
  fi\n\
  sleep 1\n\
done\n\
\n\
# Start OffGrid LLM\n\
echo "Starting OffGrid LLM..."\n\
export LLAMA_SERVER_URL="http://127.0.0.1:$LLAMA_PORT"\n\
exec offgrid serve\n\
' > /usr/local/bin/start.sh && chmod +x /usr/local/bin/start.sh

# Set working directory
WORKDIR /var/lib/offgrid

# Expose ports
EXPOSE 11611

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:11611/health || exit 1

# Run as offgrid user
USER offgrid

# Run startup script
ENTRYPOINT ["/usr/local/bin/start.sh"]

