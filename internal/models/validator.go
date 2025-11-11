package models

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Validator validates model files and metadata
type Validator struct {
	modelsDir string
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid       bool     `json:"valid"`
	ModelPath   string   `json:"model_path"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	FileSize    int64    `json:"file_size_bytes"`
	SHA256Hash  string   `json:"sha256_hash,omitempty"`
	IsGGUF      bool     `json:"is_gguf"`
	IsCorrupted bool     `json:"is_corrupted"`
}

// NewValidator creates a new model validator
func NewValidator(modelsDir string) *Validator {
	return &Validator{
		modelsDir: modelsDir,
	}
}

// ValidateModel performs comprehensive validation on a model file
func (v *Validator) ValidateModel(modelPath string) (*ValidationResult, error) {
	result := &ValidationResult{
		ModelPath: modelPath,
		Valid:     true,
		Errors:    []string{},
		Warnings:  []string{},
	}

	// Check file exists
	info, err := os.Stat(modelPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.Valid = false
			result.Errors = append(result.Errors, "model file does not exist")
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check it's a file, not directory
	if info.IsDir() {
		result.Valid = false
		result.Errors = append(result.Errors, "path is a directory, not a file")
		return result, nil
	}

	result.FileSize = info.Size()

	// Check minimum file size (GGUF files should be at least 1MB)
	if result.FileSize < 1024*1024 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("file too small (%d bytes), likely corrupted", result.FileSize))
		result.IsCorrupted = true
		return result, nil
	}

	// Validate GGUF magic number
	isGGUF, err := v.validateGGUFHeader(modelPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to read file header: %v", err))
	} else {
		result.IsGGUF = isGGUF
		if !isGGUF {
			result.Valid = false
			result.Errors = append(result.Errors, "not a valid GGUF file (invalid magic number)")
			result.IsCorrupted = true
		}
	}

	// Check file is readable throughout (detect corruption)
	if err := v.checkFileReadability(modelPath); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("file read error (corrupted): %v", err))
		result.IsCorrupted = true
	}

	// Calculate SHA256 hash
	hash, err := v.calculateSHA256(modelPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to calculate SHA256: %v", err))
	} else {
		result.SHA256Hash = hash
	}

	return result, nil
}

// ValidateWithExpectedHash validates a model and checks against expected SHA256
func (v *Validator) ValidateWithExpectedHash(modelPath, expectedSHA256 string) (*ValidationResult, error) {
	result, err := v.ValidateModel(modelPath)
	if err != nil {
		return nil, err
	}

	if expectedSHA256 != "" && result.SHA256Hash != "" {
		if result.SHA256Hash != expectedSHA256 {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("SHA256 mismatch: expected %s, got %s", expectedSHA256, result.SHA256Hash))
			result.IsCorrupted = true
		}
	}

	return result, nil
}

// validateGGUFHeader checks if file has valid GGUF magic number
func (v *Validator) validateGGUFHeader(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 4 bytes for magic number
	magic := make([]byte, 4)
	n, err := file.Read(magic)
	if err != nil {
		return false, err
	}
	if n < 4 {
		return false, fmt.Errorf("file too small to contain GGUF header")
	}

	// GGUF magic number is "GGUF" (0x47 0x47 0x55 0x46)
	// or "GGML" for older formats (0x67 0x67 0x6d 0x6c)
	if string(magic) == "GGUF" || string(magic) == "gguf" {
		return true, nil
	}

	// Also accept GGML (older format)
	if string(magic) == "GGML" || string(magic) == "ggml" {
		return true, nil
	}

	return false, nil
}

// checkFileReadability tries to read through entire file to detect corruption
func (v *Validator) checkFileReadability(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use larger buffer for faster reads
	buffer := make([]byte, 1024*1024) // 1MB chunks

	for {
		_, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// calculateSHA256 computes SHA256 hash of file
func (v *Validator) calculateSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ValidateDirectory validates all models in the models directory
func (v *Validator) ValidateDirectory() (map[string]*ValidationResult, error) {
	results := make(map[string]*ValidationResult)

	entries, err := os.ReadDir(v.modelsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read models directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// Only validate .gguf files
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") {
			continue
		}

		path := filepath.Join(v.modelsDir, name)
		result, err := v.ValidateModel(path)
		if err != nil {
			return nil, fmt.Errorf("failed to validate %s: %w", name, err)
		}

		results[name] = result
	}

	return results, nil
}

// QuickCheck performs a fast validation (header + size only)
func (v *Validator) QuickCheck(modelPath string) (bool, error) {
	// Check file exists and get size
	info, err := os.Stat(modelPath)
	if err != nil {
		return false, err
	}

	// Check minimum size
	if info.Size() < 1024*1024 {
		return false, fmt.Errorf("file too small")
	}

	// Validate GGUF header
	isGGUF, err := v.validateGGUFHeader(modelPath)
	if err != nil {
		return false, err
	}

	return isGGUF, nil
}

// RepairAttempt tries to recover a corrupted model if possible
func (v *Validator) RepairAttempt(modelPath string) error {
	// For now, we can't repair corrupted GGUF files
	// This is a placeholder for future functionality
	return fmt.Errorf("model repair not yet implemented")
}

// ExportValidationReport exports validation results to JSON
func (v *Validator) ExportValidationReport(results map[string]*ValidationResult, outputPath string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// ValidateIntegrity is a convenience function for quick integrity check
func ValidateIntegrity(modelPath, expectedSHA256 string) error {
	validator := NewValidator(filepath.Dir(modelPath))
	result, err := validator.ValidateWithExpectedHash(modelPath, expectedSHA256)
	if err != nil {
		return err
	}

	if !result.Valid {
		return fmt.Errorf("validation failed: %v", result.Errors)
	}

	return nil
}
