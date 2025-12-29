// Package inference provides LLM inference capabilities
package inference

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// QuantizationType represents supported quantization types
type QuantizationType string

const (
	QuantQ8_0    QuantizationType = "q8_0"
	QuantQ6_K    QuantizationType = "q6_k"
	QuantQ5_K_M  QuantizationType = "q5_k_m"
	QuantQ5_K_S  QuantizationType = "q5_k_s"
	QuantQ5_0    QuantizationType = "q5_0"
	QuantQ4_K_M  QuantizationType = "q4_k_m"
	QuantQ4_K_S  QuantizationType = "q4_k_s"
	QuantQ4_0    QuantizationType = "q4_0"
	QuantQ3_K_M  QuantizationType = "q3_k_m"
	QuantQ3_K_S  QuantizationType = "q3_k_s"
	QuantQ2_K    QuantizationType = "q2_k"
	QuantIQ4_NL  QuantizationType = "iq4_nl"
	QuantIQ3_XXS QuantizationType = "iq3_xxs"
	QuantIQ2_XXS QuantizationType = "iq2_xxs"
)

// QuantizationInfo describes a quantization type
type QuantizationInfo struct {
	Type        QuantizationType `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	SizeRatio   float64          `json:"size_ratio"`  // Ratio vs F16 (1.0 = same size)
	Quality     int              `json:"quality"`     // 1-10 quality rating
	Recommended bool             `json:"recommended"` // Recommended for most users
}

// AvailableQuantizations returns all supported quantization types
func AvailableQuantizations() []QuantizationInfo {
	return []QuantizationInfo{
		{QuantQ8_0, "Q8_0", "8-bit quantization, best quality", 0.50, 10, true},
		{QuantQ6_K, "Q6_K", "6-bit K-quant, excellent quality", 0.40, 9, true},
		{QuantQ5_K_M, "Q5_K_M", "5-bit K-quant medium, great balance", 0.35, 8, true},
		{QuantQ5_K_S, "Q5_K_S", "5-bit K-quant small, good quality", 0.34, 7, false},
		{QuantQ5_0, "Q5_0", "5-bit quantization", 0.33, 7, false},
		{QuantQ4_K_M, "Q4_K_M", "4-bit K-quant medium, recommended", 0.28, 7, true},
		{QuantQ4_K_S, "Q4_K_S", "4-bit K-quant small, smaller size", 0.27, 6, false},
		{QuantQ4_0, "Q4_0", "4-bit quantization, compact", 0.25, 6, false},
		{QuantQ3_K_M, "Q3_K_M", "3-bit K-quant medium, very compact", 0.22, 5, false},
		{QuantQ3_K_S, "Q3_K_S", "3-bit K-quant small", 0.20, 4, false},
		{QuantQ2_K, "Q2_K", "2-bit K-quant, minimum size", 0.15, 3, false},
		{QuantIQ4_NL, "IQ4_NL", "i-quant 4-bit non-linear, high quality", 0.27, 8, false},
		{QuantIQ3_XXS, "IQ3_XXS", "i-quant 3-bit extra small", 0.18, 4, false},
		{QuantIQ2_XXS, "IQ2_XXS", "i-quant 2-bit ultra compact", 0.12, 2, false},
	}
}

// QuantizationProgress reports progress during quantization
type QuantizationProgress struct {
	Stage        string  `json:"stage"`
	Progress     float64 `json:"progress"`
	CurrentLayer int     `json:"current_layer,omitempty"`
	TotalLayers  int     `json:"total_layers,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// Quantizer handles model quantization
type Quantizer struct {
	binManager *BinaryManager
	modelsDir  string
}

// NewQuantizer creates a new quantizer
func NewQuantizer(binDir, modelsDir string) *Quantizer {
	return &Quantizer{
		binManager: NewBinaryManager(binDir),
		modelsDir:  modelsDir,
	}
}

// QuantizeModel converts a model to a different quantization level
// inputPath: path to source GGUF model
// outputPath: path for quantized model (if empty, auto-generates)
// quantType: target quantization type
// progressCh: optional channel for progress updates
func (q *Quantizer) QuantizeModel(inputPath, outputPath string, quantType QuantizationType, progressCh chan<- QuantizationProgress) (string, error) {
	defer func() {
		if progressCh != nil {
			close(progressCh)
		}
	}()

	// Validate input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("input model not found: %s", inputPath)
	}

	// Generate output path if not provided
	if outputPath == "" {
		dir := filepath.Dir(inputPath)
		base := filepath.Base(inputPath)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		// Remove existing quant suffix if present
		name = removeQuantSuffix(name)
		outputPath = filepath.Join(dir, fmt.Sprintf("%s-%s.gguf", name, quantType))
	}

	// Check if output already exists
	if _, err := os.Stat(outputPath); err == nil {
		return "", fmt.Errorf("output file already exists: %s", outputPath)
	}

	sendProgress := func(stage string, progress float64) {
		if progressCh != nil {
			select {
			case progressCh <- QuantizationProgress{Stage: stage, Progress: progress}:
			default:
			}
		}
	}

	sendProgress("Preparing quantization", 0)

	// Get llama-quantize binary
	quantizePath, err := q.binManager.GetLlamaQuantize()
	if err != nil {
		return "", fmt.Errorf("failed to get llama-quantize binary: %w", err)
	}

	// Build command
	cmd := exec.Command(quantizePath, inputPath, outputPath, string(quantType))

	// Create pipe to read stderr for progress
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	log.Printf("Starting quantization: %s -> %s (%s)", inputPath, outputPath, quantType)
	sendProgress("Loading model", 0.1)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start quantization: %w", err)
	}

	// Parse progress from stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		layerRegex := regexp.MustCompile(`\[(\d+)/(\d+)\]`)

		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[quantize] %s", line)

			// Try to parse layer progress
			if matches := layerRegex.FindStringSubmatch(line); len(matches) == 3 {
				current, _ := strconv.Atoi(matches[1])
				total, _ := strconv.Atoi(matches[2])
				if total > 0 {
					progress := 0.1 + (float64(current)/float64(total))*0.85
					if progressCh != nil {
						select {
						case progressCh <- QuantizationProgress{
							Stage:        "Quantizing layers",
							Progress:     progress,
							CurrentLayer: current,
							TotalLayers:  total,
						}:
						default:
						}
					}
				}
			}
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		// Cleanup partial output
		os.Remove(outputPath)
		return "", fmt.Errorf("quantization failed: %w", err)
	}

	// Verify output exists
	info, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("output file not created: %w", err)
	}

	sendProgress("Complete", 1.0)
	log.Printf("Quantization complete: %s (%.2f GB)", outputPath, float64(info.Size())/(1024*1024*1024))

	return outputPath, nil
}

// EstimateOutputSize estimates the output size for a given quantization
func (q *Quantizer) EstimateOutputSize(inputPath string, quantType QuantizationType) (int64, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return 0, err
	}

	// Get size ratio for quantization type
	for _, qInfo := range AvailableQuantizations() {
		if qInfo.Type == quantType {
			// Estimate based on F16 -> quant ratio
			// First estimate F16 size from current (assumes Q8_0 or similar)
			f16Size := float64(info.Size()) * 2.0 // Rough estimate
			return int64(f16Size * qInfo.SizeRatio), nil
		}
	}

	return 0, fmt.Errorf("unknown quantization type: %s", quantType)
}

// GetQuantizationFromFilename tries to detect quantization from filename
func GetQuantizationFromFilename(filename string) QuantizationType {
	lower := strings.ToLower(filename)

	patterns := []struct {
		pattern string
		quant   QuantizationType
	}{
		{"q8_0", QuantQ8_0},
		{"q6_k", QuantQ6_K},
		{"q5_k_m", QuantQ5_K_M},
		{"q5_k_s", QuantQ5_K_S},
		{"q5_0", QuantQ5_0},
		{"q4_k_m", QuantQ4_K_M},
		{"q4_k_s", QuantQ4_K_S},
		{"q4_0", QuantQ4_0},
		{"q3_k_m", QuantQ3_K_M},
		{"q3_k_s", QuantQ3_K_S},
		{"q2_k", QuantQ2_K},
		{"iq4_nl", QuantIQ4_NL},
		{"iq3_xxs", QuantIQ3_XXS},
		{"iq2_xxs", QuantIQ2_XXS},
	}

	for _, p := range patterns {
		if strings.Contains(lower, p.pattern) {
			return p.quant
		}
	}

	return ""
}

// removeQuantSuffix removes common quantization suffixes from model name
func removeQuantSuffix(name string) string {
	suffixes := []string{
		"-q8_0", "-q6_k", "-q5_k_m", "-q5_k_s", "-q5_0",
		"-q4_k_m", "-q4_k_s", "-q4_0", "-q3_k_m", "-q3_k_s",
		"-q2_k", "-iq4_nl", "-iq3_xxs", "-iq2_xxs",
		"_q8_0", "_q6_k", "_q5_k_m", "_q5_k_s", "_q5_0",
		"_q4_k_m", "_q4_k_s", "_q4_0", "_q3_k_m", "_q3_k_s",
		"_q2_k", "_iq4_nl", "_iq3_xxs", "_iq2_xxs",
		".q8_0", ".q6_k", ".q5_k_m", ".q5_k_s", ".q5_0",
		".q4_k_m", ".q4_k_s", ".q4_0", ".q3_k_m", ".q3_k_s",
		".q2_k", ".iq4_nl", ".iq3_xxs", ".iq2_xxs",
	}

	lower := strings.ToLower(name)
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, suffix) {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
}

// QuantizeAsync runs quantization in background and reports via channel
func (q *Quantizer) QuantizeAsync(inputPath, outputPath string, quantType QuantizationType) (<-chan QuantizationProgress, error) {
	progressCh := make(chan QuantizationProgress, 100)

	go func() {
		_, err := q.QuantizeModel(inputPath, outputPath, quantType, progressCh)
		if err != nil {
			log.Printf("Quantization error: %v", err)
		}
	}()

	return progressCh, nil
}

// GetLlamaQuantize gets or downloads the llama-quantize binary
func (bm *BinaryManager) GetLlamaQuantize() (string, error) {
	// llama-quantize is usually bundled with llama.cpp
	// Check in same directory as llama-server
	llamaServer, err := bm.GetLlamaServer()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(llamaServer)
	quantizePath := filepath.Join(dir, "llama-quantize")

	// Check for quantize binary
	if _, err := os.Stat(quantizePath); err == nil {
		return quantizePath, nil
	}

	// Try with .exe extension on Windows
	quantizePathExe := quantizePath + ".exe"
	if _, err := os.Stat(quantizePathExe); err == nil {
		return quantizePathExe, nil
	}

	// Not found - user needs to provide it
	return "", fmt.Errorf("llama-quantize not found in %s - please download from llama.cpp releases", dir)
}

// QuickQuantizeProgress tracks a simple quantization job
type QuickQuantizeJob struct {
	ID          string    `json:"id"`
	InputPath   string    `json:"input_path"`
	OutputPath  string    `json:"output_path"`
	QuantType   string    `json:"quant_type"`
	Status      string    `json:"status"` // pending, running, complete, failed
	Progress    float64   `json:"progress"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}
