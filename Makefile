.PHONY: build run clean test coverage help fmt lint install cross-compile build-llama

# Binary name
BINARY=offgrid
MAIN_PATH=./cmd/offgrid
VERSION?=0.1.0-alpha
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
BUILD_TAGS_LLAMA=-tags llama

# Build the application (mock mode - no CGO required)
build:
	@echo "🔨 Building OffGrid LLM (mock mode)..."
	go build $(LDFLAGS) -o $(BINARY) $(MAIN_PATH)
	@echo "✅ Build complete: ./$(BINARY)"
	@echo "   Note: Using mock inference. For real LLM inference, use 'make build-llama'"

# Build with llama.cpp support (requires CGO and llama.cpp installation)
build-llama:
	@echo "🔨 Building OffGrid LLM with llama.cpp support..."
	@echo "   Prerequisites: llama.cpp must be installed and C_INCLUDE_PATH set"
	@echo "   See docs/LLAMA_CPP_SETUP.md for setup instructions"
	go build $(LDFLAGS) $(BUILD_TAGS_LLAMA) -o $(BINARY) $(MAIN_PATH)
	@echo "✅ Build complete: ./$(BINARY)"
	@echo "   Real llama.cpp inference enabled!"

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

# Run tests with coverage
coverage:
	@echo "📊 Running tests with coverage..."
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "✅ Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY)
	rm -f coverage.txt coverage.html
	rm -f offgrid-*
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
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	golangci-lint run ./...
	@echo "✅ Linting complete"

# Install locally
install:
	@echo "📦 Installing OffGrid LLM..."
	go install $(LDFLAGS) $(MAIN_PATH)
	@echo "✅ Installed to $(shell go env GOPATH)/bin/$(BINARY)"

# Cross-compile for all platforms
cross-compile:
	@echo "🌍 Cross-compiling for all platforms..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(MAIN_PATH)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(MAIN_PATH)
	@echo "✅ Built for all platforms in dist/"

# Download dependencies
deps:
	@echo "📦 Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies updated"

# Help
help:
	@echo "OffGrid LLM - Makefile Commands:"
	@echo ""
	@echo "  make build          - Build the binary"
	@echo "  make run            - Build and run the application"
	@echo "  make dev            - Run without building (dev mode)"
	@echo "  make test           - Run tests"
	@echo "  make coverage       - Run tests with coverage report"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Lint code (requires golangci-lint)"
	@echo "  make install        - Install to GOPATH/bin"
	@echo "  make cross-compile  - Build for all platforms"
	@echo "  make deps           - Download and tidy dependencies"
	@echo ""
