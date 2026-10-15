package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// BinaryManager resolves a compatible llama-server. The native fallback is a
// checksum-pinned CPU/Metal release; GPU builds must be supplied explicitly or
// obtained from the OffGrid GPU container.
type BinaryManager struct {
	version    string
	binDir     string
	hasNVIDIA  bool
	hasAMD     bool
	gpuEnabled atomic.Bool
}

func NewBinaryManager(binDir string) *BinaryManager {
	if configured := strings.TrimSpace(os.Getenv("OFFGRID_BIN_DIR")); configured != "" {
		binDir = configured
	}
	bm := &BinaryManager{version: "b10516", binDir: binDir}
	bm.SetGPUEnabled(strings.EqualFold(os.Getenv("OFFGRID_ENABLE_GPU"), "true"))
	bm.detectGPU()
	return bm
}

func (bm *BinaryManager) detectGPU() {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
		configureBackgroundProcess(cmd)
		if output, err := cmd.Output(); err == nil && len(output) > 0 {
			bm.hasNVIDIA = true
		}
	}
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		bm.hasAMD = true
	}
}

func (bm *BinaryManager) HasGPU() bool { return bm.hasNVIDIA || bm.hasAMD }

// SetGPUEnabled applies the effective configuration, including values loaded
// from a configuration file. The environment is only a constructor default.
func (bm *BinaryManager) SetGPUEnabled(enabled bool) { bm.gpuEnabled.Store(enabled) }

func (bm *BinaryManager) GPUType() string {
	if bm.hasNVIDIA {
		return "nvidia"
	}
	if bm.hasAMD {
		return "amd"
	}
	return "cpu"
}

func llamaServerBinaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

func (bm *BinaryManager) versionedPath() string {
	return filepath.Join(bm.binDir, "llama-"+bm.version, llamaServerBinaryName())
}

func (bm *BinaryManager) localPath() string {
	return filepath.Join(bm.binDir, llamaServerBinaryName())
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// GetLlamaServer honors explicit paths. GPU requests prefer a compatible
// installed GPU runtime; a cached CPU fallback must not shadow that runtime.
// CPU/no-device requests retain the checksum-pinned portable fallback.
func (bm *BinaryManager) GetLlamaServer() (string, error) {
	if path := strings.TrimSpace(os.Getenv("OFFGRID_LLAMA_SERVER_PATH")); path != "" {
		if !regularFile(path) {
			return "", fmt.Errorf("OFFGRID_LLAMA_SERVER_PATH does not point to a regular file: %s", path)
		}
		if bm.gpuRequested() && !llamaServerHasGPUDevice(path, bm.GPUType()) {
			return "", fmt.Errorf("explicit llama-server has no available %s device; supply a compatible GPU runtime or disable OFFGRID_ENABLE_GPU for CPU operation", bm.GPUType())
		}
		return path, nil
	}
	if bm.gpuRequested() {
		if path := bm.installedGPURuntime(llamaServerSupportsRequiredFlags, llamaServerHasGPUDevice); path != "" {
			return path, nil
		}
		return "", fmt.Errorf("no compatible GPU llama-server found; set OFFGRID_LLAMA_SERVER_PATH to a GPU build, use the OffGrid GPU container, or disable OFFGRID_ENABLE_GPU for CPU operation")
	}
	if path := bm.versionedPath(); regularFile(path) {
		if llamaServerSupportsRequiredFlags(path) {
			return path, nil
		}
		return "", fmt.Errorf("existing llama-server at %s lacks required flags; remove that specific runtime directory and retry", path)
	}
	if path := bm.localPath(); regularFile(path) && llamaServerSupportsRequiredFlags(path) {
		return path, nil
	}
	if path, err := exec.LookPath("llama-server"); err == nil && llamaServerSupportsRequiredFlags(path) {
		return path, nil
	}
	path := bm.versionedPath()
	if err := bm.downloadBinary(path); err != nil {
		return "", fmt.Errorf("download llama-server: %w", err)
	}
	return path, nil
}

func (bm *BinaryManager) gpuRequested() bool {
	return (bm.hasNVIDIA || bm.hasAMD) && bm.gpuEnabled.Load() && runtime.GOOS != "darwin"
}

func (bm *BinaryManager) installedGPURuntime(compatible func(string) bool, device func(string, string) bool) string {
	candidates := []string{bm.localPath(), bm.versionedPath()}
	if path, err := exec.LookPath("llama-server"); err == nil {
		candidates = append(candidates, path)
	}
	for _, path := range candidates {
		if regularFile(path) && compatible(path) && device(path, bm.GPUType()) {
			return path
		}
	}
	return ""
}

func llamaServerHasGPUDevice(path, gpuType string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, "--list-devices")
	configureBackgroundProcess(command)
	output, err := command.CombinedOutput()
	return err == nil && ctx.Err() == nil && llamaDeviceListHasGPU(string(output), gpuType)
}

// Read only the runtime's device-list section, never a driver error or help text
// containing the word CUDA. CPU operation does not depend on this probe.
func llamaDeviceListHasGPU(output, gpuType string) bool {
	listing := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "Available devices:" {
			listing = true
			continue
		}
		if !listing {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasSuffix(fields[0], ":") {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		prefixes := []string{"Vulkan"}
		if gpuType == "nvidia" {
			prefixes = append(prefixes, "CUDA")
		}
		if gpuType == "amd" {
			prefixes = append(prefixes, "ROCm", "HIP")
		}
		for _, prefix := range prefixes {
			index := strings.TrimPrefix(name, prefix)
			if index == name || index == "" {
				continue
			}
			valid := true
			for _, digit := range index {
				valid = valid && digit >= '0' && digit <= '9'
			}
			if valid {
				return true
			}
		}
	}
	return false
}

func llamaServerSupportsRequiredFlags(binaryPath string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, _ := exec.CommandContext(ctx, binaryPath, "--help").CombinedOutput()
	text := string(output)
	return ctx.Err() == nil && strings.Contains(text, "--jinja") && strings.Contains(text, "--fit-ctx") && strings.Contains(text, "--cache-type-k")
}

func (bm *BinaryManager) downloadBinary(destPath string) error {
	if err := os.MkdirAll(bm.binDir, 0o755); err != nil {
		return err
	}
	asset, digest, err := bm.downloadAsset()
	if err != nil {
		return err
	}
	assetURL := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/%s", bm.version, asset)
	fmt.Printf("Downloading verified llama.cpp %s runtime...\n", bm.version)
	file, err := os.CreateTemp("", "offgrid-llama-archive-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Get(assetURL)
	if err != nil {
		file.Close()
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		file.Close()
		return fmt.Errorf("release archive returned HTTP %d", response.StatusCode)
	}
	const maxArchive = 128 << 20
	if response.ContentLength > maxArchive {
		file.Close()
		return fmt.Errorf("release archive exceeds %d bytes", maxArchive)
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxArchive+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > maxArchive {
		return fmt.Errorf("release archive exceeds %d bytes", maxArchive)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != digest {
		return fmt.Errorf("release archive SHA-256 mismatch: got %s", actual)
	}
	stage, err := os.MkdirTemp(bm.binDir, ".llama-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := extractLlamaArchive(file.Name(), stage, bm.version, asset); err != nil {
		return err
	}
	if !regularFile(filepath.Join(stage, llamaServerBinaryName())) {
		return fmt.Errorf("verified release archive contains no %s", llamaServerBinaryName())
	}
	if err := os.Rename(stage, filepath.Dir(destPath)); err != nil {
		if llamaServerSupportsRequiredFlags(destPath) {
			// Another OffGrid process completed the same pinned install first.
			return nil
		}
		return fmt.Errorf("install verified runtime: %w", err)
	}
	fmt.Printf("Installed llama-server at %s\n", destPath)
	return nil
}

func (bm *BinaryManager) downloadAsset() (name, sha256Digest string, err error) {
	if bm.version != "b10516" {
		return "", "", fmt.Errorf("no verified assets for llama.cpp %s", bm.version)
	}
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "llama-b10516-bin-ubuntu-x64.tar.gz", "f263a91280471b4c33c4999d7c76259c0f3a0a53a0b3e692b2c0b84380137a35", nil
	case "linux/arm64":
		return "llama-b10516-bin-ubuntu-arm64.tar.gz", "e7491dca79c9799fc3ae169675a79f5777d3027e31ffb08ae679e5e0a7ae3c97", nil
	case "darwin/arm64":
		return "llama-b10516-bin-macos-arm64.tar.gz", "ee3324327d621026ae80c24031670e65fa62a0b23a3a027dbe2f65f240affd30", nil
	case "darwin/amd64":
		return "llama-b10516-bin-macos-x64.tar.gz", "b7adecf7bd2cde577ddabee8357a72409165d8104f43b4acee9f1b98cc9c447a", nil
	case "windows/amd64":
		return "llama-b10516-bin-win-cpu-x64.zip", "fbbbc55e0eb2e1b07f9dcb9488616c98ed47d9003b90e15e7c8c7812c4307cd3", nil
	case "windows/arm64":
		return "llama-b10516-bin-win-cpu-arm64.zip", "4b136692ab17009722e350d5bb8e5905f9af6bcd43d2897f0655186a9cc65db6", nil
	default:
		return "", "", fmt.Errorf("unsupported llama.cpp platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func (bm *BinaryManager) getDownloadURL() (string, error) {
	asset, _, err := bm.downloadAsset()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/%s", bm.version, asset), nil
}

func (bm *BinaryManager) GetVersion() string { return bm.version }

func (bm *BinaryManager) IsInstalled() bool {
	return regularFile(bm.versionedPath()) || regularFile(bm.localPath())
}

func (bm *BinaryManager) GetInstalledPath() string {
	if path := bm.versionedPath(); regularFile(path) {
		return path
	}
	if path := bm.localPath(); regularFile(path) {
		return path
	}
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	return ""
}
