# Contributing to OffGrid LLM

Thank you for your interest in contributing to OffGrid LLM! This guide will help you get started quickly.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Understanding the Codebase](#understanding-the-codebase)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Code Style Guide](#code-style-guide)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Getting Help](#getting-help)

---

## Quick Start

```bash
# 1. Fork and clone
git clone https://github.com/YOUR_USERNAME/offgrid-llm.git
cd offgrid-llm

# 2. Install dependencies
go mod download

# 3. Build
make build
# OR: go build -o bin/offgrid ./cmd/offgrid

# 4. Run tests
make test

# 5. Start development server
./bin/offgrid serve --verbose
```

**Access the UI:** http://localhost:11611

---

## Understanding the Codebase

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Web Browser / API Client                    │
└─────────────────────────────────────┬───────────────────────────────┘
                                      │ HTTP/WebSocket
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        internal/server                               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │ REST API │ │WebSocket │ │   Auth   │ │ Metrics  │ │  Static  │  │
│  │ Handlers │ │ Handler  │ │Middleware│ │ Endpoint │ │  Files   │  │
│  └────┬─────┘ └────┬─────┘ └──────────┘ └──────────┘ └──────────┘  │
└───────┼────────────┼────────────────────────────────────────────────┘
        │            │
        ▼            ▼
┌───────────────────────────────────────────────────────────────────────┐
│                          Core Services                                 │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐      │
│  │ inference  │  │    rag     │  │   agents   │  │   audio    │      │
│  │ ────────── │  │ ────────── │  │ ────────── │  │ ────────── │      │
│  │ LLM Engine │  │ Embeddings │  │ Task Exec  │  │ STT / TTS  │      │
│  │ llama.cpp  │  │ VectorDB   │  │ MCP Tools  │  │ Whisper    │      │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘      │
│                                                                        │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐      │
│  │   config   │  │   models   │  │   cache    │  │  sessions  │      │
│  │ ────────── │  │ ────────── │  │ ────────── │  │ ────────── │      │
│  │ Settings   │  │ Model Mgmt │  │ Response   │  │ Chat State │      │
│  │ Export/Imp │  │ HuggingFace│  │ Caching    │  │ Persistence│      │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘      │
└───────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌───────────────────────────────────────────────────────────────────────┐
│                          Support Services                              │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ watchdog │ │ metrics  │ │  audit   │ │degradation│ │  power   │   │
│  │ ──────── │ │ ──────── │ │ ──────── │ │ ──────── │ │ ──────── │   │
│  │ Health   │ │ Stats    │ │ Security │ │ Fallback │ │ Battery  │   │
│  │ Monitor  │ │ Tracking │ │ Logging  │ │ Handling │ │ Aware    │   │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │
└───────────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
offgrid-llm/
├── cmd/offgrid/              # CLI entry point
│   ├── main.go               # Application bootstrap, command parsing
│   └── agent_ui.go           # Interactive agent terminal UI
│
├── internal/                 # Core packages (not importable externally)
│   ├── server/               # HTTP API server
│   │   ├── server.go         # Main server setup, routing
│   │   ├── handlers.go       # REST endpoint handlers
│   │   ├── websocket.go      # WebSocket for streaming
│   │   └── middleware.go     # Auth, logging, CORS
│   │
│   ├── inference/            # LLM inference engine
│   │   ├── engine.go         # Core inference logic
│   │   ├── model_cache.go    # Model loading/caching
│   │   └── embeddings.go     # Embedding generation
│   │
│   ├── agents/               # AI agent system
│   │   ├── agent.go          # Agent orchestration
│   │   ├── executor.go       # Task execution
│   │   └── tools.go          # Built-in tools
│   │
│   ├── rag/                  # RAG/Vector search
│   │   ├── vectorstore.go    # Vector database
│   │   ├── chunker.go        # Document chunking
│   │   └── retriever.go      # Similarity search
│   │
│   ├── config/               # Configuration
│   ├── audio/                # Speech-to-text, text-to-speech
│   ├── cache/                # Response caching
│   ├── sessions/             # Chat session management
│   ├── models/               # Model management
│   ├── users/                # User management
│   ├── metrics/              # Statistics tracking
│   ├── watchdog/             # Process health monitoring
│   ├── degradation/          # Graceful degradation
│   ├── audit/                # Security audit logging
│   ├── p2p/                  # Peer-to-peer model sharing
│   ├── integrity/            # Model verification
│   ├── maintenance/          # Disk cleanup
│   ├── power/                # Battery awareness
│   ├── prewarm/              # Model pre-warming
│   └── mcp/                  # MCP server integration
│
├── pkg/api/                  # Public API types (importable)
│   └── types.go              # Request/response structs
│
├── web/ui/                   # Web interface
│   ├── index.html            # HTML structure
│   ├── css/styles.css        # Styling
│   ├── js/                   # JavaScript modules (16 files)
│   │   ├── utils.js          # Core state, helpers
│   │   ├── chat.js           # Chat functionality
│   │   ├── models.js         # Model management
│   │   └── ...               # More modules
│   ├── README.md             # UI documentation
│   └── CONTRIBUTING.md       # UI contribution guide
│
├── desktop/                  # Electron desktop app
│   ├── main.js               # Electron main process
│   ├── index.html            # UI (synced from web/ui)
│   ├── css/                  # Synced from web/ui
│   └── js/                   # Synced from web/ui
│
├── python/                   # Python SDK
│   ├── offgrid/              # Package source
│   ├── pyproject.toml        # Package config
│   └── README.md             # SDK documentation
│
├── docs/                     # Documentation
│   ├── README.md             # Documentation index
│   ├── guides/               # User guides
│   ├── advanced/             # Advanced topics
│   └── templates/            # Doc templates
│
├── scripts/                  # Build & utility scripts
│   ├── sync-ui.sh            # Sync web UI to desktop
│   ├── build-all.sh          # Build all platforms
│   └── ...
│
├── docker/                   # Docker configurations
├── dev/                      # Developer resources
│   ├── CONTRIBUTING.md       # This file
│   └── examples/             # Code examples
│
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
├── Makefile                  # Build automation
├── VERSION                   # Current version
└── README.md                 # Project README
```

### Key Packages Explained

| Package | Purpose | When to Modify |
|---------|---------|----------------|
| `cmd/offgrid` | CLI entry point | Adding new commands |
| `internal/server` | HTTP API | Adding endpoints, middleware |
| `internal/inference` | LLM engine | Model loading, inference |
| `internal/agents` | AI agents | Agent behavior, tools |
| `internal/rag` | RAG system | Document processing, search |
| `internal/config` | Configuration | Settings, env vars |
| `internal/audio` | Voice features | STT, TTS |
| `web/ui` | Browser UI | Frontend features |

---

## Development Setup

### Prerequisites

- **Go 1.21+** - [Install Go](https://go.dev/dl/)
- **Git** - Version control
- **Make** (optional) - Build automation

### Environment Setup

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm

# Install Go dependencies
go mod download

# Build the binary
make build
# OR: go build -o bin/offgrid ./cmd/offgrid

# Verify
./bin/offgrid --version
```

### IDE Setup (VS Code)

Recommended extensions:
- Go (official)
- Go Test Explorer
- GitLens

Settings (`.vscode/settings.json`):
```json
{
    "go.formatTool": "gofmt",
    "go.lintTool": "golangci-lint",
    "go.testFlags": ["-v"],
    "editor.formatOnSave": true
}
```

---

## Making Changes

### Branch Naming

```
feature/add-voice-streaming
fix/memory-leak-in-cache
docs/update-api-reference
refactor/simplify-inference
```

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add voice input support
fix: resolve memory leak in model cache
docs: update API reference
refactor: simplify inference engine
test: add unit tests for config
chore: update dependencies
```

### Adding a New Feature

1. **Create branch:** `git checkout -b feature/my-feature`
2. **Add package (if needed):**
   ```bash
   mkdir -p internal/myfeature
   touch internal/myfeature/myfeature.go
   touch internal/myfeature/myfeature_test.go
   ```
3. **Implement feature** with tests
4. **Update docs** if user-facing
5. **Run tests:** `make test`
6. **Submit PR**

### Adding a New API Endpoint

1. Add handler in `internal/server/handlers.go`:
   ```go
   func (s *Server) handleMyEndpoint(w http.ResponseWriter, r *http.Request) {
       // Implementation
   }
   ```
2. Register route in `internal/server/server.go`:
   ```go
   mux.HandleFunc("/api/v1/myendpoint", s.handleMyEndpoint)
   ```
3. Add types in `pkg/api/types.go` if needed
4. Document in `docs/reference/api.md`

---

## Code Style Guide

### Go Conventions

```go
// Package comments: Start with package name
// Package inference provides LLM inference capabilities.
package inference

// Exported types: Always document
// Engine manages the LLM inference process.
type Engine struct {
    modelPath   string
    contextSize int
}

// Methods: Document what they do
// Generate produces a completion for the given prompt.
func (e *Engine) Generate(ctx context.Context, prompt string) (string, error) {
    if prompt == "" {
        return "", ErrEmptyPrompt
    }
    // Implementation
}
```

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Packages | lowercase | `inference`, `config` |
| Exported types | PascalCase | `ModelCache`, `Engine` |
| Private types | camelCase | `modelEntry` |
| Interfaces | -er suffix | `Loader`, `Generator` |
| Errors | Err prefix | `ErrNotFound` |
| Constants | SCREAMING_SNAKE or PascalCase | `MaxRetries` |

### Error Handling

```go
// Define errors
var ErrModelNotFound = errors.New("model not found")

// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to load model %s: %w", name, err)
}
```

---

## Testing

### Running Tests

```bash
# All tests
make test

# With coverage
make test-coverage

# Specific package
go test -v ./internal/config/...

# Specific test
go test -v -run TestEngine_Generate ./internal/inference/
```

### Test Structure

```go
func TestEngine_Generate(t *testing.T) {
    tests := []struct {
        name    string
        prompt  string
        want    string
        wantErr bool
    }{
        {"valid prompt", "Hello", "response", false},
        {"empty prompt", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            e := NewEngine()
            got, err := e.Generate(context.Background(), tt.prompt)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Generate() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## Pull Request Process

### Before Submitting

- [ ] Code compiles: `make build`
- [ ] Tests pass: `make test`
- [ ] Code formatted: `go fmt ./...`
- [ ] Linter passes: `golangci-lint run`
- [ ] Documentation updated (if user-facing)

### PR Template

```markdown
## Description
Brief description of changes.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Documentation
- [ ] Refactoring

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Code formatted
```

---

## Contributing to the Web UI

The web UI has its own modular structure. See **[web/ui/CONTRIBUTING.md](../web/ui/CONTRIBUTING.md)** for details.

### Quick Overview

- **16 JavaScript modules** in `web/ui/js/`
- **No build step** - vanilla JS, global scope
- **CSS variables** for theming in `web/ui/css/styles.css`
- **Tailwind CSS** for utility classes

### Key Files

| Feature | Files |
|---------|-------|
| Chat | `chat.js`, `chat-ui.js` |
| Models | `models.js`, `models-ui.js` |
| Voice | `audio.js` |
| Agent | `agent.js` |
| Terminal | `terminal.js` |
| Styling | `css/styles.css` |

### Syncing to Desktop

After UI changes:
```bash
./scripts/sync-ui.sh
```

---

## Getting Help

- **Issues:** [GitHub Issues](https://github.com/takuphilchan/offgrid-llm/issues)
- **Documentation:** [docs/](../docs/)
- **Architecture:** [docs/advanced/architecture.md](../docs/advanced/architecture.md)

---

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
