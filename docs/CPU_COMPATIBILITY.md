# CPU Compatibility

offgrid-llm automatically detects and uses the correct binary for your CPU, ensuring maximum compatibility and performance.

## Supported CPU Types

### x86-64 (Intel/AMD)

| CPU Feature Set | Examples | Build Flags |
|----------------|----------|-------------|
| **AVX-512** | Intel Ice Lake+, Sapphire Rapids | `-march=x86-64-v4` |
| **AVX2** | Intel Haswell+ (2013+), AMD Zen+ | `-march=x86-64-v3 -mno-avx512f` |

### ARM

| Architecture | Examples | Build Flags |
|-------------|----------|-------------|
| **Apple Silicon** | M1, M2, M3, M4 | `-mcpu=apple-m1` with Metal |
| **ARM64/NEON** | Raspberry Pi 4+, AWS Graviton | `-march=armv8-a` |

## Automatic Detection

The installer automatically:
1. Detects your CPU capabilities (`/proc/cpuinfo` on Linux, `sysctl` on macOS)
2. Downloads the appropriate binary (AVX-512 or AVX2)
3. Falls back to AVX2 if AVX-512 binary unavailable (maximum compatibility)

### Detection Logic

```bash
# Linux
if grep -q "avx512" /proc/cpuinfo; then
    # Use AVX-512 build
elif grep -q "avx2" /proc/cpuinfo; then
    # Use AVX2 build
fi

# macOS
if sysctl | grep -qi "avx512"; then
    # Use AVX-512 build
else
    # Use AVX2 build (most Macs)
fi
```

## Why Multiple Builds?

**The Problem:**
- llama.cpp uses advanced CPU instructions (AVX-512, AVX2) for performance
- Binaries compiled with newer instructions crash on older CPUs (SIGILL error)
- Previous offgrid-llm releases only had AVX-512 binaries

**The Solution:**
- Build separate binaries for each CPU architecture
- Installer automatically selects the correct one
- Users never encounter CPU incompatibility issues

## Minimum Requirements

### x86-64 (Intel/AMD)
- **Recommended:** Intel Haswell (2013) or AMD Zen+ or newer
- **Minimum:** x86-64 with SSE4.2 support

### ARM
- **Apple Silicon:** M1 or newer
- **Linux ARM:** ARMv8-A (AArch64) with NEON

## Manual Override

If you need to force a specific CPU variant:

```bash
# Force AVX2 (maximum compatibility)
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | \
  bash -s -- --cpu-features avx2

# Force AVX-512 (maximum performance on newer CPUs)
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | \
  bash -s -- --cpu-features avx512
```

## Troubleshooting

### "Illegal instruction" (SIGILL)
This means the binary was compiled for a newer CPU than yours. The universal installer prevents this, but if you manually downloaded a release:

```bash
# Solution: Re-run installer (auto-detects correct variant)
curl -fsSL https://raw.githubusercontent.com/takuphilchan/offgrid-llm/main/install.sh | bash
```

### Check Your CPU Features

```bash
# Linux
grep -o -E 'avx512|avx2|sse4_2' /proc/cpuinfo | sort -u

# macOS
sysctl machdep.cpu.features machdep.cpu.leaf7_features

# Output example:
# avx2         [Compatible] Compatible with AVX2 builds
# avx512f      [Compatible] Compatible with AVX-512 builds
```

### Performance Comparison

| CPU Type | Speed | Compatibility |
|----------|-------|---------------|
| AVX-512 | Fastest | Intel Ice Lake+ only |
| AVX2 | Fast | Most CPUs from 2013+ |
| Basic | Slower | All x86-64 CPUs |

**Recommendation:** Let the installer auto-detect. It chooses the fastest variant your CPU supports.

## Technical Details

### Build Matrix

The GitHub Actions workflow builds:

**Linux:**
- `linux-amd64-cpu-avx2` - CPU inference, AVX2
- `linux-amd64-cpu-avx512` - CPU inference, AVX-512
- `linux-amd64-vulkan-avx2` - Vulkan GPU, AVX2 CPU fallback
- `linux-amd64-vulkan-avx512` - Vulkan GPU, AVX-512 CPU fallback
- `linux-arm64-cpu-neon` - ARM64 with NEON

**macOS:**
- `darwin-arm64-metal-apple-silicon` - M1/M2/M3 with Metal
- `darwin-amd64-cpu-avx2` - Intel Macs (AVX2)

**Windows:**
- `windows-amd64-cpu-avx2` - AVX2 (maximum compatibility)

### Compiler Flags Reference

```cmake
# AVX2 (compatible mode - no AVX-512)
-march=x86-64-v3 -mno-avx512f

# AVX-512 (performance mode)
-march=x86-64-v4

# ARM Apple Silicon
-mcpu=apple-m1

# ARM Generic
-march=armv8-a
```

## FAQ

**Q: Will AVX2 work on my old laptop?**
A: If it's from 2013 or newer (Intel Haswell, AMD Zen+), yes!

**Q: Do I need to manually choose?**
A: No! The installer automatically detects and downloads the correct binary.

**Q: Can I use GPU acceleration with AVX2?**
A: Yes! GPU (Vulkan/Metal) and CPU features are independent. You can have Vulkan GPU with AVX2 CPU.

**Q: What about AMD CPUs?**
A: AMD Zen and newer support AVX2. The installer works perfectly with AMD.

**Q: My CPU has AVX-512 but installer chose AVX2?**
A: This happens if the AVX-512 binary isn't available yet. AVX2 still works, just slightly slower.
