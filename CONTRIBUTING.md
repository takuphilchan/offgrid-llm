# Contributing to OffGrid LLM

Thank you for your interest in contributing to OffGrid LLM! This project aims to make AI accessible in offline and edge environments.

## 🎯 Project Vision

OffGrid LLM is designed for environments where internet connectivity is:
- **Unavailable** (ships, air-gapped networks)
- **Intermittent** (rural areas, disaster zones)
- **Expensive** (satellite, metered connections)

All contributions should keep this "offline-first" philosophy in mind.

## 🚀 Getting Started

### Prerequisites

- Go 1.21.5 or higher
- Git
- Basic understanding of LLMs and REST APIs

### Setup Development Environment

```bash
# Clone the repository
git clone git@github.com:takuphilchan/offgrid-llm.git
cd offgrid-llm

# Build the project
make build

# Run tests
make test

# Run in development mode
make dev
```

## 📂 Project Structure

```
offgrid-llm/
├── cmd/offgrid/          # Main application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── inference/        # LLM inference engine interface
│   ├── models/           # Model registry and management
│   ├── p2p/              # Peer-to-peer networking
│   ├── resource/         # Resource monitoring
│   └── server/           # HTTP server and API handlers
├── pkg/api/              # Public API types (OpenAI-compatible)
├── docs/                 # Documentation
├── scripts/              # Helper scripts
└── Makefile              # Build automation
```

## 🔧 Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Changes

- Write clean, idiomatic Go code
- Follow existing patterns and conventions
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

```bash
# Run all tests
make test

# Format code
make fmt

# Build
make build

# Test manually
./offgrid
```

### 4. Commit

```bash
git add .
git commit -m "Brief description of changes

Detailed explanation if needed:
- What changed
- Why it changed
- Any breaking changes"
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## 🎨 Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `go fmt` for formatting
- Keep functions small and focused
- Write descriptive variable names
- Add comments for exported functions
- Use error wrapping: `fmt.Errorf("context: %w", err)`

## 🧪 Testing

- Write unit tests for new functionality
- Aim for meaningful test coverage
- Test edge cases and error paths
- Use table-driven tests where appropriate

Example test structure:
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("wanted error: %v, got: %v", tt.wantErr, err)
            }
            if got != tt.want {
                t.Errorf("wanted %v, got %v", tt.want, got)
            }
        })
    }
}
```

## 📝 Documentation

- Update README.md for new features
- Add/update docs/ files for major changes
- Include code examples where helpful
- Document configuration options
- Update API documentation for endpoint changes

## 🐛 Bug Reports

When filing a bug report, include:

1. **Environment**: OS, Go version, OffGrid LLM version
2. **Steps to reproduce**: Exact commands/actions
3. **Expected behavior**: What should happen
4. **Actual behavior**: What actually happens
5. **Logs**: Relevant error messages
6. **Models**: Which models were being used

## 💡 Feature Requests

For feature requests, explain:

1. **Use case**: What problem does it solve?
2. **Offline scenario**: How does it help offline/edge deployments?
3. **Proposed solution**: How might it work?
4. **Alternatives**: Other approaches you considered

## 🎯 Priority Areas

We're currently focusing on:

### High Priority
- [ ] llama.cpp integration for actual inference
- [ ] P2P file transfer implementation
- [ ] Model loading from disk (GGUF support)
- [ ] USB distribution tools
- [ ] Resource optimization

### Medium Priority
- [ ] Streaming responses (SSE)
- [ ] Web dashboard UI
- [ ] Advanced quantization support
- [ ] Multi-model support
- [ ] ARM optimization

### Low Priority
- [ ] Embeddings endpoint
- [ ] Fine-tuning support
- [ ] Model conversion tools
- [ ] Batch processing
- [ ] API key authentication

## 🔐 Security

- Never commit sensitive data (API keys, passwords)
- Verify model checksums before loading
- Validate all user inputs
- Follow secure coding practices
- Report security issues privately to maintainers

## 📜 License

By contributing, you agree that your contributions will be licensed under the MIT License.

## 🤝 Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Collaborate openly
- Remember: we're building this for underserved communities

## 💬 Questions?

- Open an issue for bugs/features
- Start a discussion for questions
- Check existing docs first
- Be patient - maintainers may be offline!

## 🌟 Recognition

All contributors will be recognized in the project README.

Thank you for helping make AI accessible everywhere! 🌐
