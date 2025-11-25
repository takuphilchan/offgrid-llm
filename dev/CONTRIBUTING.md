# Contributing to OffGrid LLM

Thank you for your interest in contributing to OffGrid LLM!

## Repository Structure

```
offgrid-llm/
├── README.md              # Main documentation
├── LICENSE                # MIT License
├── go.mod/go.sum         # Go dependencies
│
├── cmd/                   # Application entry points
│   └── offgrid/          # Main CLI application
│
├── internal/              # Private application code
│   ├── inference/        # LLM inference engine
│   ├── models/           # Model management
│   ├── server/           # API server
│   ├── cache/            # Response caching
│   └── ...               # Other internal packages
│
├── pkg/                   # Public/reusable packages
│   └── api/              # API types
│
├── installers/            # Installation scripts for users
│   ├── install.sh        # Universal installer (downloads from releases)
│   ├── install-macos.sh  # macOS-specific installer
│   └── install-windows.ps1  # Windows installer
│
├── docs/                  # Documentation
│   ├── INSTALLATION.md   # Installation guide
│   ├── API.md           # API reference
│   └── ...              # Other guides
│
├── scripts/              # Development & build scripts
│   ├── quickstart.sh    # Quick development setup
│   └── ...              # Test scripts, utilities
│
├── build/                # Build configurations
│   ├── macos/           # macOS packaging scripts
│   └── windows/         # Windows packaging scripts
│
├── examples/             # Code examples
│   └── *.go             # Example usage
│
├── web/                  # Web UI (if applicable)
│   └── ui/
│
├── .github/              # GitHub-specific files
│   └── workflows/       # CI/CD workflows
│
└── Development Files (root)
    ├── install.sh        # Linux source build installer
    ├── Makefile         # Build automation
    ├── Dockerfile       # Container build
    └── docker-compose.yml  # Docker orchestration
```

## File Purposes

### Root Directory Files

| File | Purpose | Who Needs It |
|------|---------|--------------|
| `README.md` | Main project documentation | Everyone |
| `LICENSE` | MIT License | Everyone |
| `install.sh` | **Linux source build installer** - Compiles from source | Linux developers/advanced users |
| `Makefile` | Build automation (compile, test, release) | Developers |
| `Dockerfile` | Container image build | DevOps/container users |
| `docker-compose.yml` | Multi-container orchestration | DevOps |
| `go.mod` / `go.sum` | Go dependency management | Developers |

### Key Directories

- **`installers/`** - End-user installation scripts (use these for quick install!)
- **`docs/`** - All documentation
- **`cmd/offgrid/`** - Application source code entry point
- **`internal/`** - Core implementation (not importable by other projects)
- **`.github/workflows/`** - Automated builds and releases

## Installation vs Development

### For Users (Quick Install):
```bash
# Use the installers directory
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

### For Developers (Build from Source):
```bash
# Clone and build
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm
make build

# Or use root install.sh for full Linux setup
sudo ./install.sh
```

## Development Workflow

1. **Fork & Clone**
   ```bash
   git clone https://github.com/YOUR_USERNAME/offgrid-llm.git
   cd offgrid-llm
   ```

2. **Build**
   ```bash
   make build
   # or
   go build -o offgrid ./cmd/offgrid
   ```

3. **Test**
   ```bash
   make test
   ```

4. **Run**
   ```bash
   ./offgrid help
   ```

## Building Releases

The project uses GitHub Actions to automatically build releases:

```bash
# Create a new release
git tag -a v0.1.6 -m "Release v0.1.6"
git push origin v0.1.6
```

This triggers `.github/workflows/release.yml` which:
- Builds binaries for all platforms
- Creates GitHub Release
- Uploads artifacts

## Questions?

-  Read the [docs/](docs/) folder
-  Open an issue
-  Start a discussion

