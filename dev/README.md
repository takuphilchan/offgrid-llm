# Developer Resources

This directory contains tools and resources for **developers** who want to build, modify, or contribute to OffGrid LLM.

## 📁 What's Here

| File/Folder | Purpose |
|-------------|---------|
| `install.sh` | **Build from source** with GPU optimization (CUDA/ROCm) |
| `Makefile` | Build automation and development tasks |
| `Dockerfile` | Container image builds |
| `docker-compose.yml` | Local development environment |
| `scripts/` | Build scripts, test utilities, packaging tools |
| `examples/` | Code examples and demos |
| `CONTRIBUTING.md` | Contribution guidelines |
| `build/` | Build artifacts (gitignored) |
| `dist/` | Distribution packages (gitignored) |

## 🚀 Quick Start for Developers

### Option 1: Build from Source (with GPU)

For advanced users who want maximum performance with GPU acceleration:

```bash
# Clone repository
git clone https://github.com/takuphilchan/offgrid-llm.git
cd offgrid-llm/dev

# Build with GPU support (auto-detects CUDA/ROCm)
sudo ./install.sh
```

**Features:**
- Custom GPU optimizations
- Compiles llama.cpp with your GPU drivers
- Installs systemd service
- Full control over build process

**Time:** 10-15 minutes (compiles C++ code)

### Option 2: Standard Build (CPU only)

```bash
# Using Makefile
cd offgrid-llm
make build

# Or directly with Go
go build -o offgrid ./cmd/offgrid
```

### Option 3: Docker Development

```bash
# Build and run in container
docker-compose up --build
```

## 🛠️ Development Workflow

1. **Make changes** to source code in `cmd/`, `internal/`, or `pkg/`
2. **Build**: `make build` or `go build`
3. **Test**: `make test` or `go test ./...`
4. **Run**: `./offgrid version`

## [Package] Building Releases

The CI/CD pipeline (`.github/workflows/release.yml`) automatically builds releases when you push a tag:

```bash
git tag v0.1.3
git push origin v0.1.3
```

This creates pre-built binaries for 6 platforms that users can install via the quick installer.

## 🔄 Build vs Install Methods

### For End Users (Recommended)
```bash
# Quick install - downloads pre-built binaries
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/installers/install.sh | bash
```

### For Developers (This Directory)
```bash
# Build from source - compiles everything
cd dev
sudo ./install.sh
```

## 📚 More Information

- **User Documentation**: `../docs/`
- **API Reference**: `../docs/API.md`
- **Architecture**: `../docs/ARCHITECTURE.md`
- **Contributing Guidelines**: `CONTRIBUTING.md`

---

**Note**: If you just want to **use** OffGrid LLM, you don't need anything from this directory! Use the quick installer from the main README.
