.PHONY: build run clean test help

# Binary name
BINARY=offgrid
MAIN_PATH=./cmd/offgrid

# Build the application
build:
	@echo "🔨 Building OffGrid LLM..."
	go build -o $(BINARY) $(MAIN_PATH)
	@echo "✅ Build complete: ./$(BINARY)"

# Run the application
run: build
	@echo "🚀 Starting OffGrid LLM..."
	./$(BINARY)

# Run without building (for development)
dev:
	@echo "🔧 Running in dev mode..."
	go run $(MAIN_PATH)

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY)
	go clean
	@echo "✅ Cleaned"

# Format code
fmt:
	@echo "📝 Formatting code..."
	go fmt ./...
	@echo "✅ Formatted"

# Lint code
lint:
	@echo "🔍 Linting code..."
	golangci-lint run || echo "Install golangci-lint for linting"

# Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated"

# Help
help:
	@echo "OffGrid LLM - Makefile Commands:"
	@echo "  make build   - Build the binary"
	@echo "  make run     - Build and run the application"
	@echo "  make dev     - Run without building (dev mode)"
	@echo "  make test    - Run tests"
	@echo "  make clean   - Remove build artifacts"
	@echo "  make fmt     - Format code"
	@echo "  make lint    - Lint code"
	@echo "  make deps    - Download and tidy dependencies"
