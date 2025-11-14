# Installation Strategy Analysis & Recommendations

**Date:** November 14, 2025  
**Issue:** Multiple confusing installation paths causing poor user experience  

---

## Current Problems

### 1. **Three Different Installation Scripts**

| Script | Purpose | What It Does | Problems |
|--------|---------|--------------|----------|
| `installers/install.sh` (585 lines) | Quick install | Downloads pre-built binaries from GitHub releases | ❌ References releases that don't exist<br>❌ Downloads llama.cpp from external source<br>❌ Creates systemd service sometimes |
| `dev/install.sh` (1584 lines) | Production install | Compiles everything from source | ❌ Takes 10-15 minutes<br>❌ Requires build tools<br>❌ Confusing for end users |
| `dev/scripts/build-static-bundle.sh` (260 lines) | Bundle builder | Creates Ollama-like bundles | ❌ Not integrated with releases<br>❌ Manual process |

### 2. **GitHub Workflows Don't Match Scripts**

**Release workflow (`.github/workflows/release.yml`):**
- ✅ Builds Go binaries only
- ❌ No llama-server included
- ❌ Creates `offgrid-linux-amd64.tar.gz` but installers expect bundles

**Bundle workflow (`.github/workflows/build-release-bundles.yml`):**
- ✅ Builds complete bundles with llama-server
- ✅ Creates `offgrid-v1.0.0-linux-amd64-vulkan.tar.gz`
- ❌ Not triggered by tags (only manual dispatch)
- ❌ Different naming than what installers expect

### 3. **Installer Script Mismatch**

`installers/install.sh` tries to download:
```bash
# It looks for:
https://github.com/takuphilchan/offgrid-llm/releases/download/v0.1.0/offgrid-linux-amd64.tar.gz

# But releases only have:
offgrid-linux-amd64 (just the binary, no tar.gz)
```

Then it separately downloads llama.cpp from:
```bash
https://github.com/ggml-org/llama.cpp/releases/download/b3950/llama-server-b3950-bin-ubuntu-x64.zip
```

**Problems with this approach:**
- ❌ External dependency on llama.cpp releases
- ❌ Version mismatch issues between offgrid and llama-server
- ❌ llama.cpp binaries have dynamic libraries that fail (`BUILD_SHARED_LIBS=ON`)
- ❌ Two separate downloads confuse users
- ❌ No guarantee of compatibility

### 4. **Desktop App Not in Releases**

The Electron desktop app exists but:
- ❌ No build process in CI/CD
- ❌ Not available as downloadable package
- ❌ Users must build from source

---

## How Successful Projects Handle This

### **Ollama** (Best Practice Example)

**Installation:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**What they do right:**
1. **Single install script** - One command, works everywhere
2. **Self-contained releases** - Each release has everything bundled
3. **Platform detection** - Auto-detects OS, arch, GPU
4. **GitHub releases structure:**
   ```
   v0.1.0/
   ├── ollama-linux-amd64           (single binary, ~50MB)
   ├── ollama-linux-amd64.sha256
   ├── ollama-darwin-arm64
   └── ollama-windows-amd64.exe
   ```
5. **Everything embedded** - llama.cpp compiled into binary
6. **No separate dependencies** - Users download one file

**Key insight:** They embed llama.cpp directly into the Go binary using CGO and static linking.

---

### **Docker** 

**Installation:**
```bash
curl -fsSL https://get.docker.com | sh
```

**What they do right:**
1. **Single script** - Detects distro and installs from repos
2. **Clear versioning** - Stable/Edge channels
3. **Package managers** - APT/YUM repos for updates
4. **No manual downloads** - Script handles everything

---

### **Rust (rustup)**

**Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

**What they do right:**
1. **Single installer** - Works on all platforms
2. **Self-updating** - `rustup update` for new versions
3. **Toolchain management** - Easy switching between versions
4. **Clear documentation** - One path for everyone

---

### **Node.js / nvm**

**What they do right:**
1. **Multiple methods well-documented:**
   - Official installers (recommended)
   - Package managers
   - From source (advanced)
2. **Clear separation** - Each method has dedicated docs
3. **Version managers** - nvm/fnm for switching versions

---

## Recommended Strategy for OffGrid LLM

### **Goal: One-Command Install Like Ollama**

```bash
curl -fsSL https://offgrid.dev/install | sh
```

### **Unified Approach**

#### **1. GitHub Releases Structure (v1.0.0)**

```
v1.0.0/
├── offgrid-v1.0.0-linux-amd64-cpu.tar.gz           (~40MB)
├── offgrid-v1.0.0-linux-amd64-vulkan.tar.gz        (~55MB)
├── offgrid-v1.0.0-linux-arm64-cpu.tar.gz
├── offgrid-v1.0.0-darwin-arm64-metal.tar.gz        (macOS Apple Silicon)
├── offgrid-v1.0.0-darwin-amd64-cpu.tar.gz          (macOS Intel)
├── offgrid-v1.0.0-windows-amd64-cpu.zip
├── offgrid-v1.0.0-windows-amd64-cuda.zip
├── offgrid-desktop-v1.0.0-linux-x64.AppImage       (Electron app)
├── offgrid-desktop-v1.0.0-windows-x64.exe          (Electron installer)
├── offgrid-desktop-v1.0.0-macos-arm64.dmg
├── checksums.sha256
└── install.sh                                       (universal installer)
```

**Each bundle contains:**
- `offgrid` binary (~15MB)
- `llama-server` binary (~30-40MB, statically linked)
- `install.sh` script
- `checksums.sha256`

#### **2. Single Universal Installer**

**Location:** Root of repo: `/install.sh` (hosted at https://offgrid.dev/install)

```bash
#!/bin/bash
# OffGrid LLM Universal Installer
# One script for all platforms

detect_platform() {
    # Auto-detect: OS, arch, GPU
    # Returns: linux-amd64-vulkan, darwin-arm64-metal, etc.
}

download_bundle() {
    # Download from GitHub releases
    # URL: https://github.com/takuphilchan/offgrid-llm/releases/download/v1.0.0/offgrid-v1.0.0-${PLATFORM}.tar.gz
}

install_bundle() {
    # Extract and copy to /usr/local/bin
    # Optional: Setup systemd service on Linux
}

verify_checksums() {
    # Verify downloaded files
}

main() {
    detect_platform
    download_bundle
    verify_checksums
    install_bundle
    print_success
}
```

**Features:**
- Auto-detects platform and GPU
- Downloads appropriate bundle from GitHub releases
- Verifies checksums
- Installs both binaries
- Optional systemd service setup
- ~300 lines total (simple, maintainable)

#### **3. GitHub Actions Workflow**

**Single workflow:** `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags: ['v*']

jobs:
  build-bundles:
    strategy:
      matrix:
        include:
          - os: ubuntu-22.04
            platform: linux-amd64
            variant: cpu
          - os: ubuntu-22.04
            platform: linux-amd64
            variant: vulkan
          - os: macos-latest
            platform: darwin-arm64
            variant: metal
          # ... more combinations
    
    steps:
      - Build offgrid binary
      - Build llama-server with static linking
      - Create bundle tarball
      - Upload to GitHub release
  
  build-desktop:
    strategy:
      matrix:
        os: [ubuntu, windows, macos]
    steps:
      - Build Electron app
      - Create installer (AppImage/exe/dmg)
      - Upload to GitHub release
  
  publish-installer:
    steps:
      - Copy install.sh to release assets
      - Update checksums.sha256
```

#### **4. Remove Confusing Methods**

**Keep:**
- ✅ Single universal installer (root `/install.sh`)
- ✅ Desktop app downloads (releases)
- ✅ Advanced: Build from source (`dev/README.md` for contributors)

**Remove:**
- ❌ `installers/` directory (merged into root `/install.sh`)
- ❌ `dev/install.sh` (move to `dev/build.sh` for development only)
- ❌ Separate llama.cpp downloads
- ❌ Multiple installation paths

#### **5. Documentation Structure**

**README.md:**
```markdown
## Installation

### Recommended (One Command)
```bash
curl -fsSL https://offgrid.dev/install | sh
# or
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | sh
```

### Desktop App
Download from [releases](https://github.com/takuphilchan/offgrid-llm/releases/latest):
- Linux: `.AppImage`
- macOS: `.dmg`
- Windows: `.exe`

### Advanced: Build from Source
See [dev/README.md](dev/README.md)
```

---

## Implementation Plan

### Phase 1: Fix Releases (Immediate)

**Tasks:**
1. ✅ Create unified `.github/workflows/release.yml`
2. ✅ Build bundles with both offgrid + llama-server
3. ✅ Use consistent naming: `offgrid-v{VERSION}-{OS}-{ARCH}-{VARIANT}.tar.gz`
4. ✅ Add desktop app builds to workflow
5. ✅ Generate checksums.sha256

**Impact:** Users can download working bundles from releases

### Phase 2: Unified Installer (Next)

**Tasks:**
1. ✅ Create `/install.sh` (root level)
2. ✅ Move `installers/install.sh` logic to root installer
3. ✅ Remove `installers/` directory
4. ✅ Installer detects platform and downloads from releases
5. ✅ Update README to use new installer

**Impact:** One clear installation path

### Phase 3: Clean Up (Final)

**Tasks:**
1. ✅ Rename `dev/install.sh` to `dev/build.sh` (for development)
2. ✅ Update all documentation
3. ✅ Remove references to multiple install methods
4. ✅ Update contributing guide

**Impact:** Clear separation: users vs developers

---

## Comparison: Before vs After

### Before (Current - Confusing)

```
Installation options:
1. installers/install.sh → Downloads from releases (broken)
2. dev/install.sh → Builds from source (slow)
3. Desktop app → Not released

User journey:
❌ Finds 3 different READMEs
❌ Tries quick install → fails (release not found)
❌ Tries production install → takes 15 minutes
❌ Gives up and uses Docker
```

### After (Proposed - Clear)

```
Installation options:
1. curl install.sh (recommended) → Works immediately
2. Desktop app download → Double-click installer
3. Build from source (dev/build.sh) → For contributors only

User journey:
✅ Sees one command in README
✅ Runs it, works in 30 seconds
✅ Downloads desktop app if preferred
✅ Clear and simple
```

---

## Technical Details

### Static Bundle Creation

**Embed llama-server into offgrid binary:**

Option A: **Separate binaries in bundle (Current approach - RECOMMENDED)**
```
offgrid-v1.0.0-linux-amd64-vulkan.tar.gz
└── offgrid-v1.0.0-linux-amd64-vulkan/
    ├── offgrid           (15MB)
    ├── llama-server      (40MB)
    ├── install.sh
    └── checksums.sha256
```

Option B: **Embed as Go binary (Ollama approach - ADVANCED)**
```go
//go:embed llama-server-linux-amd64
var llamaServerBinary []byte

func startLlamaServer() {
    tmpPath := extractBinary(llamaServerBinary)
    cmd := exec.Command(tmpPath, args...)
    cmd.Start()
}
```

**Recommendation:** Use Option A (separate binaries) for simplicity. Option B requires:
- CGO for llama.cpp integration
- Complex build process
- Platform-specific compilation
- Larger binary size

### Systemd Integration

**Optional during install:**
```bash
# install.sh offers:
"Setup auto-start on boot? (systemd) [y/N]"

# If yes, creates:
/etc/systemd/system/offgrid.service
/etc/systemd/system/llama-server.service
```

### Desktop App Distribution

**Platform-specific:**
- **Linux:** AppImage (single file, no install needed)
- **macOS:** DMG (drag-to-Applications)
- **Windows:** NSIS installer or portable .exe

---

## Success Metrics

After implementation:

✅ **One command install works** - `curl ... | sh` succeeds  
✅ **Releases have all bundles** - Every platform covered  
✅ **Desktop apps available** - AppImage/DMG/exe in releases  
✅ **Clear documentation** - One installation section  
✅ **Fast install time** - <1 minute for bundles  
✅ **No external dependencies** - Everything in GitHub releases  
✅ **Automatic updates** - Future: `offgrid update` command  

---

## Next Steps

1. **Immediate:** Fix `.github/workflows/release.yml` to build complete bundles
2. **Week 1:** Create unified `/install.sh` at root
3. **Week 2:** Add desktop app builds to CI/CD
4. **Week 3:** Update all documentation
5. **Week 4:** Remove old installation methods and deprecated dirs

**Goal:** By end of month, have one clear installation path that always works.
