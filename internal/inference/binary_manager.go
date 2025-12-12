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
	version string
	binDir  string
}

// NewBinaryManager creates a new binary manager
func NewBinaryManager(binDir string) *BinaryManager {
	return &BinaryManager{
		version: "b4320", // Pinned version for stability
		binDir:  binDir,
	}
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

	fmt.Printf("Downloading llama-server from %s...\n", url)

	// Download zip to temp file
	tmpFile, err := os.CreateTemp("", "llama-server-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}

	// Extract zip
	return bm.extractZip(tmpFile.Name(), destPath)
}

func (bm *BinaryManager) getDownloadURL() (string, error) {
	baseURL := "https://github.com/ggerganov/llama.cpp/releases/download"

	// Map OS/Arch to asset name
	// Note: These names are based on recent llama.cpp release conventions
	// We might need to update this if they change
	var assetName string

	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			assetName = fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", bm.version)
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			assetName = fmt.Sprintf("llama-%s-bin-macos-arm64.zip", bm.version)
		} else if runtime.GOARCH == "amd64" {
			assetName = fmt.Sprintf("llama-%s-bin-macos-x64.zip", bm.version)
		}
	case "windows":
		if runtime.GOARCH == "amd64" {
			// Default to AVX2 for Windows x64
			assetName = fmt.Sprintf("llama-%s-bin-win-avx2-x64.zip", bm.version)
		}
	}

	if assetName == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	return fmt.Sprintf("%s/%s/%s", baseURL, bm.version, assetName), nil
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
