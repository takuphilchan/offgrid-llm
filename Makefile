.PHONY: build run clean test coverage help fmt lint install cross-compile build-llama

# Binary name
BINARY=offgrid
MAIN_PATH=./cmd/offgrid
VERSION?=0.1.0-alpha
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
BUILD_TAGS_LLAMA=-tags llama

# Force using local Go toolchain to prevent auto-upgrade
export GOTOOLCHAIN=local

# Color definitions (matching installer)
BRAND_PRIMARY=\033[38;5;45m
BRAND_SUCCESS=\033[38;5;78m
BRAND_ERROR=\033[38;5;196m
BRAND_MUTED=\033[38;5;240m
RESET=\033[0m
BOLD=\033[1m
DIM=\033[2m

# Build the application (mock mode - no CGO required)
build:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Building OffGrid LLM$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Building binary..."
	@go build $(LDFLAGS) -o $(BINARY) $(MAIN_PATH)
	@echo "$(BRAND_SUCCESS)✓$(RESET) Build complete: $(BOLD)./$(BINARY)$(RESET)"
	@echo "$(DIM)  Using HTTP-based llama-server integration$(RESET)"
	@echo ""

# Build with llama.cpp support (requires CGO and llama.cpp installation)
build-llama:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Building OffGrid LLM with llama.cpp$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_MUTED)Prerequisites: llama.cpp must be installed$(RESET)"
	@echo "$(DIM)  See docs/LLAMA_CPP_SETUP.md for setup instructions$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Building with llama.cpp support..."
	@go build $(LDFLAGS) $(BUILD_TAGS_LLAMA) -o $(BINARY) $(MAIN_PATH)
	@echo "$(BRAND_SUCCESS)✓$(RESET) Build complete: $(BOLD)./$(BINARY)$(RESET)"
	@echo "$(BRAND_SUCCESS)✓$(RESET) Real llama.cpp inference enabled"
	@echo ""

# Run the application
run: build
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Starting OffGrid LLM$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@./$(BINARY)

# Run without building (for development)
dev:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Running in Development Mode$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@go run $(MAIN_PATH)

# Run tests
test:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Running Tests$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@go test -v ./...
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Tests complete"
	@echo ""

# Run tests with coverage
coverage:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Running Tests with Coverage$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Running tests..."
	@go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Generating coverage report..."
	@go tool cover -html=coverage.txt -o coverage.html
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Coverage report: $(BOLD)coverage.html$(RESET)"
	@echo ""

# Clean build artifacts
clean:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Cleaning Build Artifacts$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Removing binaries..."
	@rm -f $(BINARY)
	@rm -f offgrid-*
	@echo "$(BRAND_PRIMARY)→$(RESET) Removing coverage files..."
	@rm -f coverage.txt coverage.html
	@echo "$(BRAND_PRIMARY)→$(RESET) Running go clean..."
	@go clean
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Cleaned"
	@echo ""

# Format code
fmt:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Formatting Code$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Running go fmt..."
	@go fmt ./...
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Code formatted"
	@echo ""

# Lint code
lint:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Linting Code$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "$(BRAND_PRIMARY)→$(RESET) Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		echo "$(BRAND_SUCCESS)✓$(RESET) golangci-lint installed"; \
		echo ""; \
	}
	@echo "$(BRAND_PRIMARY)→$(RESET) Running linter..."
	@golangci-lint run ./...
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Linting complete"
	@echo ""

# Install to user's Go bin (adds to PATH if GOPATH/bin is configured)
install:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Installing OffGrid LLM (User)$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Installing to user bin..."
	@go install $(LDFLAGS) $(MAIN_PATH)
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Installed to $(BOLD)$$(go env GOPATH)/bin/$(BINARY)$(RESET)"
	@echo ""
	@echo "$(BRAND_MUTED)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_MUTED)│$(RESET) $(BOLD)Post-Installation Steps$(RESET)"
	@echo "$(BRAND_MUTED)├────────────────────────────────────────────────────────────────────┤$(RESET)"
	@echo "$(BRAND_MUTED)│$(RESET) Add to your shell configuration:"
	@echo "$(BRAND_MUTED)│$(RESET)   $(DIM)export PATH=\"\$$PATH:\$$(go env GOPATH)/bin\"$(RESET)"
	@echo "$(BRAND_MUTED)│$(RESET)"
	@echo "$(BRAND_MUTED)│$(RESET) Then reload your shell:"
	@echo "$(BRAND_MUTED)│$(RESET)   $(DIM)source ~/.bashrc$(RESET) or $(DIM)source ~/.zshrc$(RESET)"
	@echo "$(BRAND_MUTED)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""

# Install to system-wide location (requires sudo)
install-system:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Installing OffGrid LLM (System-Wide)$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Building binary..."
	@go build $(LDFLAGS) -o $(BINARY) $(MAIN_PATH)
	@echo "$(BRAND_PRIMARY)→$(RESET) Installing to /usr/local/bin..."
	@sudo install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Installed to $(BOLD)/usr/local/bin/$(BINARY)$(RESET)"
	@echo "$(BRAND_SUCCESS)✓$(RESET) Run $(BOLD)offgrid$(RESET) from anywhere"
	@echo ""

# Uninstall from system
uninstall-system:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Uninstalling OffGrid LLM$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Removing from /usr/local/bin..."
	@sudo rm -f /usr/local/bin/$(BINARY)
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Uninstalled"
	@echo ""

# Cross-compile for all platforms
cross-compile:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Cross-Compiling for All Platforms$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Creating dist directory..."
	@mkdir -p dist
	@echo "$(BRAND_PRIMARY)→$(RESET) Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 $(MAIN_PATH)
	@echo "$(BRAND_PRIMARY)→$(RESET) Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 $(MAIN_PATH)
	@echo "$(BRAND_PRIMARY)→$(RESET) Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 $(MAIN_PATH)
	@echo "$(BRAND_PRIMARY)→$(RESET) Building for darwin/arm64 (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 $(MAIN_PATH)
	@echo "$(BRAND_PRIMARY)→$(RESET) Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe $(MAIN_PATH)
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Built for all platforms in $(BOLD)dist/$(RESET)"
	@echo ""
	@echo "$(BRAND_MUTED)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_MUTED)│$(RESET) $(BOLD)Available Binaries$(RESET)"
	@echo "$(BRAND_MUTED)├────────────────────────────────────────────────────────────────────┤$(RESET)"
	@ls -lh dist/ | tail -n +2 | awk '{print "$(BRAND_MUTED)│$(RESET)  " $$9 " $(DIM)(" $$5 ")$(RESET)"}'
	@echo "$(BRAND_MUTED)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""

# Download dependencies
deps:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)Managing Dependencies$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BRAND_PRIMARY)→$(RESET) Downloading dependencies..."
	@go mod download
	@echo "$(BRAND_PRIMARY)→$(RESET) Tidying go.mod and go.sum..."
	@go mod tidy
	@echo ""
	@echo "$(BRAND_SUCCESS)✓$(RESET) Dependencies updated"
	@echo ""

# Help
help:
	@echo ""
	@echo "$(BRAND_PRIMARY)╭────────────────────────────────────────────────────────────────────╮$(RESET)"
	@echo "$(BRAND_PRIMARY)│$(RESET) $(BOLD)OffGrid LLM - Makefile Commands$(RESET)"
	@echo "$(BRAND_PRIMARY)╰────────────────────────────────────────────────────────────────────╯$(RESET)"
	@echo ""
	@echo "$(BOLD)Building$(RESET)"
	@echo "  $(BRAND_PRIMARY)make build$(RESET)            Build the binary"
	@echo "  $(BRAND_PRIMARY)make build-llama$(RESET)      Build with llama.cpp support (CGO)"
	@echo "  $(BRAND_PRIMARY)make cross-compile$(RESET)    Build for all platforms"
	@echo ""
	@echo "$(BOLD)Running$(RESET)"
	@echo "  $(BRAND_PRIMARY)make run$(RESET)              Build and run the application"
	@echo "  $(BRAND_PRIMARY)make dev$(RESET)              Run without building (dev mode)"
	@echo ""
	@echo "$(BOLD)Testing$(RESET)"
	@echo "  $(BRAND_PRIMARY)make test$(RESET)             Run tests"
	@echo "  $(BRAND_PRIMARY)make coverage$(RESET)         Run tests with coverage report"
	@echo ""
	@echo "$(BOLD)Code Quality$(RESET)"
	@echo "  $(BRAND_PRIMARY)make fmt$(RESET)              Format code with go fmt"
	@echo "  $(BRAND_PRIMARY)make lint$(RESET)             Lint code with golangci-lint"
	@echo ""
	@echo "$(BOLD)Installation$(RESET)"
	@echo "  $(BRAND_PRIMARY)make install$(RESET)          Install to GOPATH/bin (user)"
	@echo "  $(BRAND_PRIMARY)make install-system$(RESET)   Install to /usr/local/bin (system-wide)"
	@echo "  $(BRAND_PRIMARY)make uninstall-system$(RESET) Uninstall from /usr/local/bin"
	@echo ""
	@echo "$(BOLD)Maintenance$(RESET)"
	@echo "  $(BRAND_PRIMARY)make clean$(RESET)            Remove build artifacts"
	@echo "  $(BRAND_PRIMARY)make deps$(RESET)             Download and tidy dependencies"
	@echo "  $(BRAND_PRIMARY)make help$(RESET)             Show this help message"
	@echo ""
