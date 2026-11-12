# CPU and runtime compatibility

Choose the [desktop download](../setup/installation.md#download-and-verify-a-release)
for ordinary first use. CLI/runtime archives are separate packages with explicit
instruction requirements. CPU model names and purchase dates alone do not establish
compatibility, especially inside a virtual machine.

## Published CLI targets

| Platform | Archive suffix | Runtime requirements |
| --- | --- | --- |
| Linux x64 | cpu-avx2 or vulkan-avx2 | SSE4.2, AVX, AVX2, F16C, FMA and BMI2 |
| Linux x64 | cpu-avx512 or vulkan-avx512 | AVX2 requirements plus AVX512F/DQ/BW/VL |
| Linux ARM64 | cpu-neon | ARMv8-A/AArch64 with NEON; CLI/web service only |
| macOS Intel | cpu-avx2 | AVX2 runtime requirements above |
| macOS Apple Silicon | metal-apple-silicon | Native Apple Silicon and the packaged Metal runtime |
| Windows x64 | cpu-avx2 | AVX2 runtime with F16C, FMA and BMI2; matching bundled DLLs |

There is no published basic/SSE4.2-only x64 CLI archive, Windows ARM64 desktop or
CLI archive, or Linux ARM64 desktop package. SSE4.2 alone is **not sufficient** for
the AVX2 archives. AVX512 is not automatically faster for every workload.

The root and compatibility Unix installers select the exact ARM archive names.
On x64 they inspect the instruction families used by the runtime and refuse an
unsupported CPU before downloading or changing the installation. All four listed
AVX512 families must be present before selecting that variant; an unavailable
AVX512 download can fall back to AVX2 on the already compatible host.

The Unix desktop helper uses AVX2 for an eligible x64 CLI installation and refuses
Linux ARM64 desktop choices before making changes. The Windows PowerShell helper
checks native x64 architecture and OS-visible AVX2 support. A machine whose OS does
not report support is refused rather than assumed compatible. These checks do not
qualify every OS, driver, virtual machine or model; run a real response after setup.
Use Windows Setup or the PowerShell helper on Windows; the root shell installer
supports Linux and macOS.

## Desktop and GPU support

Desktop bundles the OffGrid service, not a general guarantee of inference hardware
support. When needed, it prepares a separate pinned CPU/Metal engine. Its engine
and chosen model must work on the actual computer. Keep downloaded runtimes and
their supporting libraries together. The [platform matrix](../setup/installation.md#platform-capabilities)
distinguishes bundled components from separately required runtimes.

Windows CUDA and Linux GPU operation require compatible runtimes and drivers.
Installing a CUDA toolkit does not convert the CPU archive into a GPU build.
Configure a compatible executable with OFFGRID_LLAMA_SERVER_PATH, or use the
supported GPU container. See [inference performance](PERFORMANCE.md) and
[Docker setup](../setup/docker.md). Apple Silicon uses Metal; the Intel Mac
archive is CPU-only.

## If a runtime cannot start

An Illegal instruction error can mean the binary needs instructions unavailable
to the host or hidden by its virtual machine. Reinstalling the same archive cannot
add CPU instructions. Preserve models and workspace data, check the selected
archive and host capabilities, then use a compatible machine, remote OffGrid
service, or a custom llama.cpp build qualified for that host. Do not repeatedly
retry an incompatible executable or substitute an unrelated release's libraries.

Inspect OS-visible flags with grep -m1 '^flags' /proc/cpuinfo on Linux, or
sysctl machdep.cpu.features machdep.cpu.leaf7_features on Intel macOS.

Memory is a separate requirement. The project targets lightweight workflows on
8 GB systems and recommends 16 GB; model weights, context, KV cache, the OS and
other applications all use memory. A matching instruction set does not guarantee
that a selected model fits or performs well. See the
[quickstart](../setup/quickstart.md) and [qualification evidence](../quality/README.md).
