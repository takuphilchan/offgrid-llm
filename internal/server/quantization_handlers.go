package server

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// handleQuantizeTypes returns available quantization types
func (s *Server) handleQuantizeTypes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	types := inference.AvailableQuantizations()
	json.NewEncoder(w).Encode(map[string]any{
		"quantization_types": types,
	})
}

// handleQuantize handles model quantization requests
func (s *Server) handleQuantize(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ModelID    string `json:"model_id"`    // Model ID to quantize
		ModelPath  string `json:"model_path"`  // Or direct path
		OutputPath string `json:"output_path"` // Optional output path
		QuantType  string `json:"quant_type"`  // Target quantization (e.g., q4_k_m)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	// Resolve model path
	inputPath := req.ModelPath
	if inputPath == "" && req.ModelID != "" {
		// Look up model by ID
		model, err := s.registry.GetModel(req.ModelID)
		if err != nil || model == nil {
			http.Error(w, `{"error": "model not found"}`, http.StatusNotFound)
			return
		}
		inputPath = model.Path
	}

	if inputPath == "" {
		http.Error(w, `{"error": "model_id or model_path required"}`, http.StatusBadRequest)
		return
	}

	// Validate quantization type
	quantType := inference.QuantizationType(strings.ToLower(req.QuantType))
	validQuant := false
	for _, q := range inference.AvailableQuantizations() {
		if q.Type == quantType {
			validQuant = true
			break
		}
	}
	if !validQuant {
		http.Error(w, `{"error": "invalid quantization type"}`, http.StatusBadRequest)
		return
	}

	// Create quantizer
	binDir := filepath.Join(s.config.ModelsDir, "..", "bin")
	quantizer := inference.NewQuantizer(binDir, s.config.ModelsDir)

	// Estimate output size
	estimatedSize, _ := quantizer.EstimateOutputSize(inputPath, quantType)

	// Start quantization in background
	progressCh, err := quantizer.QuantizeAsync(inputPath, req.OutputPath, quantType)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return immediately with job info
	json.NewEncoder(w).Encode(map[string]any{
		"success":        true,
		"status":         "started",
		"input_path":     inputPath,
		"quant_type":     quantType,
		"estimated_size": estimatedSize,
		"message":        "Quantization started in background. Check model list for completion.",
	})

	// Log progress in background
	go func() {
		for progress := range progressCh {
			if progress.Error != "" {
				log.Printf("Quantization error: %s", progress.Error)
			} else {
				log.Printf("Quantization: %s (%.1f%%)", progress.Stage, progress.Progress*100)
			}
		}
		// Rescan models after completion
		s.registry.ScanModels()
	}()
}
