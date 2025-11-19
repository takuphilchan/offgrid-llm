# OffGrid LLM - Production Docker Image
# Multi-stage build for optimized image size

FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    g++ \
    musl-dev \
    linux-headers

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o offgrid cmd/offgrid/main.go

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    wget

# Create non-root user
RUN addgroup -g 1000 offgrid && \
    adduser -D -u 1000 -G offgrid offgrid

# Create required directories
RUN mkdir -p /var/lib/offgrid/models \
             /var/lib/offgrid/web/ui \
             /var/lib/offgrid/data && \
    chown -R offgrid:offgrid /var/lib/offgrid

# Copy binary from builder
COPY --from=builder /build/offgrid /usr/local/bin/offgrid
RUN chmod +x /usr/local/bin/offgrid

# Copy web UI
COPY --from=builder /build/web/ui /var/lib/offgrid/web/ui

# Switch to non-root user
USER offgrid
WORKDIR /var/lib/offgrid

# Expose default port
EXPOSE 11611

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:11611/health || exit 1

# Set environment variables
ENV OFFGRID_MODELS_DIR=/var/lib/offgrid/models \
    OFFGRID_PORT=11611 \
    OFFGRID_HOST=0.0.0.0

# Run the server
CMD ["/usr/local/bin/offgrid"]
