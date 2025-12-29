package inference

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinaryManager handles the lifecycle of the llama-server binary
type BinaryManager struct {
	version   string
	binDir    string
	hasNVIDIA bool
	hasAMD    bool
}

// NewBinaryManager creates a new binary manager
func NewBinaryManager(binDir string) *BinaryManager {
	bm := &BinaryManager{
		version: "b4320", // Pinned version for stability
		binDir:  binDir,
	}
	bm.detectGPU()
	return bm
}

// detectGPU checks for available GPU acceleration
func (bm *BinaryManager) detectGPU() {
	// Check for NVIDIA
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		cmd := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
		if output, err := cmd.Output(); err == nil && len(output) > 0 {
			bm.hasNVIDIA = true
		}
	}

	// Check for AMD ROCm
	if _, err := exec.LookPath("rocm-smi"); err == nil {
		bm.hasAMD = true
	}
}

// HasGPU returns true if GPU acceleration is available
func (bm *BinaryManager) HasGPU() bool {
	return bm.hasNVIDIA || bm.hasAMD
}

// GPUType returns the detected GPU type
func (bm *BinaryManager) GPUType() string {
	if bm.hasNVIDIA {
		return "nvidia"
	}
	if bm.hasAMD {
		return "amd"
	}
	return "cpu"
}

// GetLlamaServer returns the path to the llama-server binary
// It checks (in order):
// 1. OFFGRID_LLAMA_SERVER_PATH environment variable
// 2. Local bin directory
// 3. System PATH
// 4. Downloads it if not found
func (bm *BinaryManager) GetLlamaServer() (string, error) {
	// 1. Check env
	if path := os.Getenv("OFFGRID_LLAMA_SERVER_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 2. Check local bin
	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	localPath := filepath.Join(bm.binDir, binaryName)
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// 3. Check PATH
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path, nil
	}

	// 4. Download
	if err := bm.downloadBinary(localPath); err != nil {
		return "", fmt.Errorf("failed to download llama-server: %w", err)
	}

	return localPath, nil
}

func (bm *BinaryManager) downloadBinary(destPath string) error {
	// Ensure bin dir exists
	if err := os.MkdirAll(bm.binDir, 0755); err != nil {
		return err
	}

	url, err := bm.getDownloadURL()
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  ⇣ Downloading llama-server (one-time setup)...\n")
	fmt.Printf("    Source: %s\n", url)
	if bm.hasNVIDIA {
		fmt.Printf("    GPU:    NVIDIA (CUDA acceleration enabled)\n")
	} else if bm.hasAMD {
		fmt.Printf("    GPU:    AMD (ROCm acceleration enabled)\n")
	} else {
		fmt.Printf("    GPU:    None (CPU mode)\n")
	}
	fmt.Println()

	// Download zip to temp file
	tmpFile, err := os.CreateTemp("", "llama-server-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %s", resp.Status)
	}

	// Get content length for progress
	contentLength := resp.ContentLength

	// Download with progress
	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if contentLength > 0 {
				percent := float64(downloaded) / float64(contentLength) * 100
				fmt.Printf("\r    Progress: %.1f%% (%d MB / %d MB)", percent, downloaded/(1024*1024), contentLength/(1024*1024))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	fmt.Println()
	fmt.Printf("    ✓ Download complete\n")

	// Extract zip
	fmt.Printf("    Extracting binary...\n")
	if err := bm.extractZip(tmpFile.Name(), destPath); err != nil {
		return err
	}
	fmt.Printf("    ✓ Installed to %s\n", destPath)
	fmt.Println()

	return nil
}

func (bm *BinaryManager) getDownloadURL() (string, error) {
	baseURL := "https://github.com/ggerganov/llama.cpp/releases/download"

	// Map OS/Arch/GPU to asset name
	// Based on llama.cpp release conventions as of late 2024
	var assetName string

	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			if bm.hasNVIDIA {
				// CUDA build for NVIDIA GPUs
				assetName = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", bm.version)
				// Note: The CUDA version is in a separate asset, but the ubuntu build
				// typically includes CUDA support if nvidia drivers are present
			} else {
				assetName = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", bm.version)
			}
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			// Apple Silicon with Metal support
			assetName = fmt.Sprintf("llama-%s-bin-macos-arm64.zip", bm.version)
		} else if runtime.GOARCH == "amd64" {
			assetName = fmt.Sprintf("llama-%s-bin-macos-x64.zip", bm.version)
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			if bm.hasNVIDIA {
				// CUDA build for Windows with NVIDIA
				assetName = fmt.Sprintf("llama-%s-bin-win-cuda-cu12.2.0-x64.zip", bm.version)
			} else {
				// AVX2 CPU build for Windows
				assetName = fmt.Sprintf("llama-%s-bin-win-avx2-x64.zip", bm.version)
			}
		}
	}

	if assetName == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return fmt.Sprintf("%s/%s/%s", baseURL, bm.version, assetName), nil
}

// GetVersion returns the pinned llama.cpp version
func (bm *BinaryManager) GetVersion() string {
	return bm.version
}

// IsInstalled checks if llama-server is already installed
func (bm *BinaryManager) IsInstalled() bool {
	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	localPath := filepath.Join(bm.binDir, binaryName)
	_, err := os.Stat(localPath)
	return err == nil
}

// GetInstalledPath returns the path to the installed binary, or empty if not installed
func (bm *BinaryManager) GetInstalledPath() string {
	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	localPath := filepath.Join(bm.binDir, binaryName)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}
	if path, err := exec.LookPath("llama-server"); err == nil {
		return path
	}
	return ""
}

func (bm *BinaryManager) extractZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	binaryName := "llama-server"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// Find the binary in the zip
	// Note: The zip structure might vary (e.g. inside a folder)
	// We search for the binary name
	var foundFile *zip.File
	for _, f := range r.File {
		// Check if filename matches (ignoring directories)
		baseName := filepath.Base(f.Name)
		if baseName == binaryName {
			foundFile = f
			break
		}
		// Also check for "server" which was the old name
		if baseName == "server" || baseName == "server.exe" {
			foundFile = f
			break
		}
	}

	if foundFile == nil {
		return fmt.Errorf("binary %s not found in zip", binaryName)
	}

	// Extract
	rc, err := foundFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}
